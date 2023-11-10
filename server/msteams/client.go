//go:generate mockery --name=Client
package msteams

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	azidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/markdown"
	"github.com/microsoft/kiota-abstractions-go/serialization"
	msgraphsdk "github.com/microsoftgraph/msgraph-beta-sdk-go"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/chats"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/drives"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/groups"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/models"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/models/odataerrors"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/sites"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/teams"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/users"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	a "github.com/microsoftgraph/msgraph-sdk-go-core/authentication"
	"golang.org/x/oauth2"
)

type ConcurrentGraphRequestAdapter struct {
	msgraphsdk.GraphRequestAdapter
	mutex sync.Mutex
}

type ConcurrentSerializationWriterFactory struct {
	serialization.SerializationWriterFactory
	mutex sync.Mutex
}

func (sf *ConcurrentSerializationWriterFactory) GetSerializationWriter(contentType string) (serialization.SerializationWriter, error) {
	sf.mutex.Lock()
	defer sf.mutex.Unlock()
	return sf.SerializationWriterFactory.GetSerializationWriter(contentType)
}

func (a *ConcurrentGraphRequestAdapter) GetSerializationWriterFactory() serialization.SerializationWriterFactory {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	return a.GraphRequestAdapter.GetSerializationWriterFactory()
}

var clientMutex sync.Mutex

type ClientImpl struct {
	client       *msgraphsdk.GraphServiceClient
	ctx          context.Context
	tenantID     string
	clientID     string
	clientSecret string
	clientType   string // can be "app" or "token"
	token        *oauth2.Token
	logError     func(msg string, keyValuePairs ...any)
}

type Subscription struct {
	ID        string
	Type      string
	ChannelID string
	TeamID    string
	UserID    string
	ExpiresOn time.Time
}

type Channel struct {
	ID          string                       `json:"id"`
	DisplayName string                       `json:"display_name"`
	Description string                       `json:"description"`
	Type        models.ChannelMembershipType `json:"type"`
}

type Chat struct {
	ID      string
	Members []ChatMember
	Type    string
}

type User struct {
	DisplayName      string
	ID               string
	Mail             string
	Type             string
	IsAccountEnabled bool
}

type ChatMember struct {
	DisplayName string
	UserID      string
	Email       string
}

type Team struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

type Attachment struct {
	ID           string
	ContentType  string
	Content      string
	Name         string
	ContentURL   string
	ThumbnailURL string
	Data         io.Reader
}

type Reaction struct {
	UserID   string
	Reaction string
}

type Mention struct {
	ID            int32
	UserID        string
	MentionedText string
}

type Message struct {
	ID              string
	UserID          string
	UserDisplayName string
	Text            string
	Subject         string
	ReplyToID       string
	Attachments     []Attachment
	Reactions       []Reaction
	Mentions        []Mention
	ChannelID       string
	TeamID          string
	ChatID          string
	LastUpdateAt    time.Time
}

type Activity struct {
	Resource                       string
	ClientState                    string
	ChangeType                     string
	LifecycleEvent                 string
	SubscriptionExpirationDateTime time.Time
	SubscriptionID                 string
}

type ActivityIds struct {
	ChatID           string
	TeamID           string
	ChannelID        string
	MessageID        string
	ReplyID          string
	HostedContentsID string
}

type AccessToken struct {
	tokenSource oauth2.TokenSource
}

type GraphAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ChatMessageAttachmentUser struct {
	UserIdentityType string `json:"userIdentityType"`
	ID               string `json:"id"`
	DisplayName      string `json:"displayName"`
}

type ChatMessageAttachmentSender struct {
	User ChatMessageAttachmentUser `json:"user"`
}

type ChatMessageAttachment struct {
	MessageID      string                      `json:"messageId"`
	MessagePreview string                      `json:"messagePreview"`
	MessageSender  ChatMessageAttachmentSender `json:"messageSender"`
}

func (e *GraphAPIError) Error() string {
	return fmt.Sprintf("code: %s, message: %s", e.Code, e.Message)
}

func NormalizeGraphAPIError(err error) error {
	if err == nil {
		return nil
	}

	switch e := err.(type) {
	case *odataerrors.ODataError:
		if terr := e.GetError(); terr != nil {
			return &GraphAPIError{
				Code:    *terr.GetCode(),
				Message: *terr.GetMessage(),
			}
		}
	default:
		return &GraphAPIError{
			Code:    "",
			Message: err.Error(),
		}
	}

	return nil
}

func (at AccessToken) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	token, err := at.tokenSource.Token()
	if err != nil {
		return azcore.AccessToken{}, err
	}
	return azcore.AccessToken{
		Token:     token.AccessToken,
		ExpiresOn: token.Expiry,
	}, nil
}

var teamsDefaultScopes = []string{"https://graph.microsoft.com/.default"}

func NewApp(tenantID, clientID, clientSecret string, logError func(string, ...any)) Client {
	return &ClientImpl{
		ctx:          context.Background(),
		clientType:   "app",
		tenantID:     tenantID,
		clientID:     clientID,
		clientSecret: clientSecret,
		logError:     logError,
	}
}

func NewTokenClient(redirectURL, tenantID, clientID, clientSecret string, token *oauth2.Token, logError func(string, ...any)) Client {
	client := &ClientImpl{
		ctx:        context.Background(),
		clientType: "token",
		tenantID:   tenantID,
		clientID:   clientID,
		token:      token,
		logError:   logError,
	}

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       append(teamsDefaultScopes, "offline_access"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantID),
			TokenURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID),
		},
		RedirectURL: redirectURL,
	}

	accessToken := AccessToken{tokenSource: conf.TokenSource(context.Background(), client.token)}

	auth, err := a.NewAzureIdentityAuthenticationProviderWithScopes(accessToken, append(teamsDefaultScopes, "offline_access"))
	if err != nil {
		logError("Unable to create the client from the token", "error", err)
		return nil
	}

	adapter, err := msgraphsdk.NewGraphRequestAdapter(auth)
	if err != nil {
		logError("Unable to create the client from the token", "error", err)
		return nil
	}

	clientMutex.Lock()
	defer clientMutex.Unlock()
	client.client = msgraphsdk.NewGraphServiceClient(&ConcurrentGraphRequestAdapter{GraphRequestAdapter: *adapter})

	return client
}

func (tc *ClientImpl) Connect() error {
	var cred azcore.TokenCredential
	switch tc.clientType {
	case "token":
		return nil
	case "app":
		var err error
		cred, err = azidentity.NewClientSecretCredential(
			tc.tenantID,
			tc.clientID,
			tc.clientSecret,
			&azidentity.ClientSecretCredentialOptions{
				ClientOptions: azcore.ClientOptions{
					Retry: policy.RetryOptions{
						MaxRetries:    3,
						RetryDelay:    4 * time.Second,
						MaxRetryDelay: 120 * time.Second,
					},
				},
			},
		)
		if err != nil {
			return err
		}

	default:
		return errors.New("not valid client type, this shouldn't happen ever")
	}

	client, err := msgraphsdk.NewGraphServiceClientWithCredentials(cred, teamsDefaultScopes)
	if err != nil {
		return err
	}
	tc.client = client

	return nil
}

func (tc *ClientImpl) GetMyID() (string, error) {
	requestParameters := &users.UserItemRequestBuilderGetQueryParameters{
		Select: []string{"id"},
	}
	configuration := &users.UserItemRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.Me().Get(tc.ctx, configuration)
	if err != nil {
		return "", NormalizeGraphAPIError(err)
	}
	return *r.GetId(), nil
}

func (tc *ClientImpl) GetMe() (*User, error) {
	requestParameters := &users.UserItemRequestBuilderGetQueryParameters{
		Select: []string{"id", "mail", "userPrincipalName", "displayName"},
	}
	configuration := &users.UserItemRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.Me().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	mail := r.GetMail()
	if mail == nil || *mail == "" {
		mail = r.GetUserPrincipalName()
	}

	displayName := r.GetDisplayName()
	user := &User{ID: *r.GetId()}
	if displayName != nil {
		user.DisplayName = *displayName
	}
	if mail != nil {
		user.Mail = strings.ToLower(*mail)
	}

	return user, nil
}

func (tc *ClientImpl) SendMessage(teamID, channelID, parentID, message string) (*Message, error) {
	return tc.SendMessageWithAttachments(teamID, channelID, parentID, message, nil, nil)
}

func (tc *ClientImpl) SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*Attachment, mentions []models.ChatMessageMentionable) (*Message, error) {
	rmsg := models.NewChatMessage()

	msteamsAttachments := []models.ChatMessageAttachmentable{}
	for _, a := range attachments {
		att := a
		contentType := "reference"
		attachment := models.NewChatMessageAttachment()
		attachment.SetId(&att.ID)
		attachment.SetContentType(&contentType)

		extension := filepath.Ext(att.Name)
		if !strings.HasSuffix(att.ContentURL, extension) {
			teamsURL, err := url.Parse(att.ContentURL)
			if err != nil {
				tc.logError("Unable to parse URL", "Error", err.Error())
				continue
			}

			q := teamsURL.Query()
			fileQueryParam := q.Get("file")
			q.Del("file")
			teamsURL.RawQuery = q.Encode()

			// We are deleting the file query param from the content URL as it is present in
			// the middle and when MS Teams processes the content URL, it needs the file query param at the end
			// otherwise, it gives the error: "contentUrl extension and name extension do not match"
			// So, we are appending the file query param at the end
			teamsURL.RawQuery += fmt.Sprintf("&file=%s", fileQueryParam)
			att.ContentURL = teamsURL.String()
		}

		attachment.SetContentUrl(&att.ContentURL)
		attachment.SetName(&att.Name)
		msteamsAttachments = append(msteamsAttachments, attachment)
		message = "<attachment id=\"" + att.ID + "\"></attachment>" + message
	}
	rmsg.SetAttachments(msteamsAttachments)
	rmsg.SetMentions(mentions)

	contentType := models.HTML_BODYTYPE

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&message)
	rmsg.SetBody(body)

	var res models.ChatMessageable
	if parentID != "" {
		var err error
		res, err = tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).Replies().Post(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		var err error
		res, err = tc.client.TeamsById(teamID).ChannelsById(channelID).Messages().Post(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) SendChat(chatID, message string, parentMessage *Message, attachments []*Attachment, mentions []models.ChatMessageMentionable) (*Message, error) {
	rmsg := models.NewChatMessage()

	msteamsAttachments := []models.ChatMessageAttachmentable{}
	if parentMessage != nil && parentMessage.ID != "" {
		parentMessage.Text = markdown.ConvertToMD(parentMessage.Text)
		contentType := "messageReference"
		contentData, err := json.Marshal(ChatMessageAttachment{
			MessageID:      parentMessage.ID,
			MessagePreview: parentMessage.Text,
			MessageSender: ChatMessageAttachmentSender{
				ChatMessageAttachmentUser{
					UserIdentityType: "aadUser",
					ID:               parentMessage.UserID,
					DisplayName:      parentMessage.UserDisplayName,
				},
			},
		})

		if err != nil {
			tc.logError("Unable to convert content to JSON", "error", err)
		} else {
			message = fmt.Sprintf("<attachment id=%q></attachment> %s", parentMessage.ID, message)
			content := string(contentData)
			attachment := models.NewChatMessageAttachment()
			attachment.SetId(&parentMessage.ID)
			attachment.SetContentType(&contentType)
			attachment.SetContent(&content)
			msteamsAttachments = append(msteamsAttachments, attachment)
		}
	}

	for _, a := range attachments {
		att := a
		contentType := "reference"
		attachment := models.NewChatMessageAttachment()
		attachment.SetId(&att.ID)
		attachment.SetContentType(&contentType)

		extension := filepath.Ext(att.Name)
		if !strings.HasSuffix(att.ContentURL, extension) {
			teamsURL, err := url.Parse(att.ContentURL)
			if err != nil {
				tc.logError("Unable to parse URL", "Error", err.Error())
				continue
			}

			q := teamsURL.Query()
			fileQueryParam := q.Get("file")
			q.Del("file")
			teamsURL.RawQuery = q.Encode()

			// We are deleting the file query param from the content URL as it is present in
			// the middle and when MS Teams processes the content URL, it needs the file query param at the end
			// otherwise, it gives the error: "contentUrl extension and name extension do not match"
			// So, we are appending the file query param at the end
			teamsURL.RawQuery += fmt.Sprintf("&file=%s", fileQueryParam)
			att.ContentURL = teamsURL.String()
		}

		attachment.SetContentUrl(&att.ContentURL)
		attachment.SetName(&att.Name)
		msteamsAttachments = append(msteamsAttachments, attachment)
		message = fmt.Sprintf("<attachment id=%q></attachment> %s", att.ID, message)
	}

	rmsg.SetAttachments(msteamsAttachments)

	contentType := models.HTML_BODYTYPE

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&message)
	rmsg.SetBody(body)

	rmsg.SetMentions(mentions)

	res, err := tc.client.ChatsById(chatID).Messages().Post(tc.ctx, rmsg, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return convertToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) UploadFile(teamID, channelID, filename string, filesize int, mimeType string, data io.Reader) (*Attachment, error) {
	driveID := ""
	itemID := ""
	if teamID != "" && channelID != "" {
		folderInfo, err := tc.client.TeamsById(teamID).ChannelsById(channelID).FilesFolder().Get(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}

		driveID = *folderInfo.GetParentReference().GetDriveId()
		itemID = *folderInfo.GetId() + ":/" + filename + ":"
	} else {
		drive, err := tc.client.Me().Drive().Get(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}

		driveID = *drive.GetId()
		rootDirectory, err := tc.client.DrivesById(driveID).Root().Get(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}

		itemID = *rootDirectory.GetId() + ":/" + filename + ":"
	}

	uploadSession, err := tc.client.DrivesById(driveID).ItemsById(itemID).CreateUploadSession().Post(tc.ctx, nil, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	req, err := http.NewRequest("PUT", *uploadSession.GetUploadUrl(), data)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Length", fmt.Sprintf("%d", filesize))
	req.Header.Add("Content-Range", fmt.Sprintf("bytes 0-%d/%d", filesize-1, filesize))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	uploadedFileData, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var uploadedFile struct {
		ID     string
		Name   string
		WebURL string
		ETag   string
	}
	err = json.Unmarshal(uploadedFileData, &uploadedFile)
	if err != nil {
		return nil, err
	}

	attachment := Attachment{
		ID:          uploadedFile.ETag[2:38],
		Name:        uploadedFile.Name,
		ContentURL:  uploadedFile.WebURL,
		ContentType: mimeType,
	}

	return &attachment, nil
}

func (tc *ClientImpl) DeleteMessage(teamID, channelID, parentID, msgID string) error {
	if parentID != "" {
		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(msgID).Delete(tc.ctx, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	} else {
		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(msgID).Delete(tc.ctx, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	}
	return nil
}

func (tc *ClientImpl) DeleteChatMessage(chatID, msgID string) error {
	return NormalizeGraphAPIError(tc.client.ChatsById(chatID).MessagesById(msgID).Delete(tc.ctx, nil))
}

func (tc *ClientImpl) UpdateMessage(teamID, channelID, parentID, msgID, message string, mentions []models.ChatMessageMentionable) error {
	rmsg := models.NewChatMessage()

	contentType := models.HTML_BODYTYPE
	rmsg.SetMentions(mentions)

	var originalMessage models.ChatMessageable
	var err error
	if parentID != "" {
		originalMessage, err = tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(msgID).Get(tc.ctx, nil)
	} else {
		originalMessage, err = tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(msgID).Get(tc.ctx, nil)
	}
	if err != nil {
		tc.logError("Error in getting original message from Teams", "error", NormalizeGraphAPIError(err))
	}

	if originalMessage != nil {
		attachments := originalMessage.GetAttachments()
		for _, a := range attachments {
			message = fmt.Sprintf("<attachment id=%q></attachment> %s", *a.GetId(), message)
		}
		rmsg.SetAttachments(attachments)
	}

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&message)
	rmsg.SetBody(body)

	if parentID != "" {
		if _, err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(msgID).Patch(tc.ctx, rmsg, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	} else {
		if _, err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(msgID).Patch(tc.ctx, rmsg, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	}
	return nil
}

func (tc *ClientImpl) UpdateChatMessage(chatID, msgID, message string, mentions []models.ChatMessageMentionable) error {
	rmsg := models.NewChatMessage()

	originalMessage, err := tc.client.ChatsById(chatID).MessagesById(msgID).Get(tc.ctx, nil)
	if err != nil {
		tc.logError("Error in getting original message from Teams", "error", NormalizeGraphAPIError(err))
	}

	if originalMessage != nil {
		attachments := originalMessage.GetAttachments()
		for _, a := range attachments {
			message = fmt.Sprintf("<attachment id=%q></attachment> %s", *a.GetId(), message)
		}
		rmsg.SetAttachments(attachments)
	}

	contentType := models.HTML_BODYTYPE

	rmsg.SetMentions(mentions)

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&message)
	rmsg.SetBody(body)
	if _, err := tc.client.ChatsById(chatID).MessagesById(msgID).Patch(tc.ctx, rmsg, nil); err != nil {
		return NormalizeGraphAPIError(err)
	}

	return nil
}

func (tc *ClientImpl) subscribe(baseURL, webhookSecret, resource, changeType string) (*Subscription, error) {
	expirationDateTime := time.Now().Add(30 * time.Minute)

	subscriptionsRes, err := tc.client.Subscriptions().Get(tc.ctx, nil)
	if err != nil {
		tc.logError("Unable to get the subcscriptions list", NormalizeGraphAPIError(err))
		return nil, NormalizeGraphAPIError(err)
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Subscriptionable](subscriptionsRes, tc.client.GetAdapter(), models.CreateSubscriptionCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	var existingSubscription models.Subscriptionable
	err = pageIterator.Iterate(context.Background(), func(subscription models.Subscriptionable) bool {
		if subscription.GetResource() != nil && (*subscription.GetResource() == resource || *subscription.GetResource()+"?model=B" == resource) {
			existingSubscription = subscription
			return false
		}
		return true
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	lifecycleNotificationURL := baseURL + "lifecycle"
	notificationURL := baseURL + "changes"

	subscription := models.NewSubscription()
	subscription.SetResource(&resource)
	subscription.SetExpirationDateTime(&expirationDateTime)
	subscription.SetNotificationUrl(&notificationURL)
	subscription.SetLifecycleNotificationUrl(&lifecycleNotificationURL)
	subscription.SetClientState(&webhookSecret)
	subscription.SetChangeType(&changeType)

	if existingSubscription != nil {
		if *existingSubscription.GetChangeType() != changeType || *existingSubscription.GetLifecycleNotificationUrl() != lifecycleNotificationURL || *existingSubscription.GetNotificationUrl() != notificationURL || *existingSubscription.GetClientState() != webhookSecret {
			if err = tc.client.SubscriptionsById(*existingSubscription.GetId()).Delete(tc.ctx, nil); err != nil {
				tc.logError("Unable to delete the subscription", "error", NormalizeGraphAPIError(err), "subscription", existingSubscription)
			}
		} else {
			updatedSubscription := models.NewSubscription()
			updatedSubscription.SetExpirationDateTime(&expirationDateTime)
			if _, err = tc.client.SubscriptionsById(*existingSubscription.GetId()).Patch(tc.ctx, updatedSubscription, nil); err != nil {
				tc.logError("Unable to refresh the subscription", "error", NormalizeGraphAPIError(err), "subscription", existingSubscription)
				return &Subscription{
					ID:        *existingSubscription.GetId(),
					ExpiresOn: *existingSubscription.GetExpirationDateTime(),
				}, nil
			}

			if err = tc.client.SubscriptionsById(*existingSubscription.GetId()).Delete(tc.ctx, nil); err != nil {
				tc.logError("Unable to delete the subscription", "error", NormalizeGraphAPIError(err), "subscription", existingSubscription)
			}
		}
	}

	res, err := tc.client.Subscriptions().Post(tc.ctx, subscription, nil)
	if err != nil {
		tc.logError("Unable to create the new subscription", "error", NormalizeGraphAPIError(err))
		return nil, NormalizeGraphAPIError(err)
	}

	return &Subscription{
		ID:        *res.GetId(),
		ExpiresOn: *res.GetExpirationDateTime(),
	}, nil
}

func (tc *ClientImpl) SubscribeToChannels(baseURL, webhookSecret string, pay bool) (*Subscription, error) {
	resource := "teams/getAllMessages"
	if pay {
		resource = "teams/getAllMessages?model=B"
	}
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) SubscribeToChannel(teamID, channelID, baseURL, webhookSecret string) (*Subscription, error) {
	resource := fmt.Sprintf("/teams/%s/channels/%s/messages", teamID, channelID)
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) SubscribeToChats(baseURL, webhookSecret string, pay bool) (*Subscription, error) {
	resource := "chats/getAllMessages"
	if pay {
		resource = "chats/getAllMessages?model=B"
	}
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) SubscribeToUserChats(userID, baseURL, webhookSecret string, pay bool) (*Subscription, error) {
	resource := fmt.Sprintf("/users/%s/chats/getAllMessages", userID)
	if pay {
		resource = fmt.Sprintf("/users/%s/chats/getAllMessages?model=B", userID)
	}
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) RefreshSubscription(subscriptionID string) (*time.Time, error) {
	expirationDateTime := time.Now().Add(30 * time.Minute)
	updatedSubscription := models.NewSubscription()
	updatedSubscription.SetExpirationDateTime(&expirationDateTime)
	if _, err := tc.client.SubscriptionsById(subscriptionID).Patch(tc.ctx, updatedSubscription, nil); err != nil {
		tc.logError("Unable to refresh the subscription", "error", NormalizeGraphAPIError(err), "subscriptionID", subscriptionID)
		return nil, NormalizeGraphAPIError(err)
	}
	return &expirationDateTime, nil
}

func (tc *ClientImpl) DeleteSubscription(subscriptionID string) error {
	if err := tc.client.SubscriptionsById(subscriptionID).Delete(tc.ctx, nil); err != nil {
		tc.logError("Unable to delete the subscription", "error", NormalizeGraphAPIError(err), "subscriptionID", subscriptionID)
		return NormalizeGraphAPIError(err)
	}
	return nil
}

func (tc *ClientImpl) GetTeam(teamID string) (*Team, error) {
	res, err := tc.client.TeamsById(teamID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	displayName := ""
	if res.GetDisplayName() != nil {
		displayName = *res.GetDisplayName()
	}

	return &Team{ID: teamID, DisplayName: displayName}, nil
}

func (tc *ClientImpl) GetTeams(filterQuery string) ([]*Team, error) {
	requestParameters := &groups.GroupsRequestBuilderGetQueryParameters{
		Filter: &filterQuery,
		Select: []string{"id", "displayName"},
	}

	configuration := &groups.GroupsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}

	res, err := tc.client.Groups().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	msTeamsGroups := res.GetValue()
	teams := make([]*Team, len(msTeamsGroups))
	for idx, group := range msTeamsGroups {
		teams[idx] = &Team{
			ID:          *group.GetId(),
			DisplayName: *group.GetDisplayName(),
		}
	}

	return teams, nil
}

func (tc *ClientImpl) GetChannelInTeam(teamID, channelID string) (*Channel, error) {
	res, err := tc.client.TeamsById(teamID).ChannelsById(channelID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	displayName := ""
	if res.GetDisplayName() != nil {
		displayName = *res.GetDisplayName()
	}

	return &Channel{ID: channelID, DisplayName: displayName}, nil
}

func (tc *ClientImpl) GetChannelsInTeam(teamID, filterQuery string) ([]*Channel, error) {
	requestParameters := &teams.ItemChannelsRequestBuilderGetQueryParameters{
		Filter: &filterQuery,
		Select: []string{"id", "displayName", "membershipType"},
	}

	configuration := &teams.ItemChannelsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}

	res, err := tc.client.TeamsById(teamID).Channels().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	msTeamsChannels := res.GetValue()
	channels := make([]*Channel, len(msTeamsChannels))
	for idx, channel := range msTeamsChannels {
		channels[idx] = &Channel{
			ID:          *channel.GetId(),
			DisplayName: *channel.GetDisplayName(),
			Type:        *channel.GetMembershipType(),
		}
	}

	return channels, nil
}

func (tc *ClientImpl) GetChat(chatID string) (*Chat, error) {
	requestParameters := &chats.ChatItemRequestBuilderGetQueryParameters{
		Expand: []string{"members"},
	}
	configuration := &chats.ChatItemRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	res, err := tc.client.ChatsById(chatID).Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	chatType := ""
	if res.GetChatType() != nil && *res.GetChatType() == models.GROUP_CHATTYPE {
		chatType = "G"
	} else if res.GetChatType() != nil && *res.GetChatType() == models.ONEONONE_CHATTYPE {
		chatType = "D"
	}

	members := []ChatMember{}
	for _, member := range res.GetMembers() {
		displayName := ""
		if member.GetDisplayName() != nil {
			displayName = *member.GetDisplayName()
		}
		emptyString := ""
		userID, err := member.GetBackingStore().Get("userId")
		if err != nil || userID == nil {
			userID = &emptyString
		}
		email, err := member.GetBackingStore().Get("email")
		if err != nil || email == nil {
			email = &emptyString
		}

		members = append(members, ChatMember{
			DisplayName: displayName,
			UserID:      *(userID.(*string)),
			Email:       *(email.(*string)),
		})
	}

	return &Chat{ID: chatID, Members: members, Type: chatType}, nil
}

func convertToMessage(msg models.ChatMessageable, teamID, channelID, chatID string) *Message {
	data, _ := json.Marshal(msg)
	fmt.Println("==================", string(data), "===================")

	userID := ""
	if msg.GetFrom() != nil && msg.GetFrom().GetUser() != nil && msg.GetFrom().GetUser().GetId() != nil {
		userID = *msg.GetFrom().GetUser().GetId()
	}
	userDisplayName := ""
	if msg.GetFrom() != nil && msg.GetFrom().GetUser() != nil && msg.GetFrom().GetUser().GetDisplayName() != nil {
		userDisplayName = *msg.GetFrom().GetUser().GetDisplayName()
	}

	replyTo := ""
	if msg.GetReplyToId() != nil {
		replyTo = *msg.GetReplyToId()
	}

	text := ""
	if msg.GetBody() != nil && msg.GetBody().GetContent() != nil {
		text = *msg.GetBody().GetContent()
	}

	msgID := ""
	if msg.GetId() != nil {
		msgID = *msg.GetId()
	}

	subject := ""
	if msg.GetSubject() != nil {
		subject = *msg.GetSubject()
	}

	lastUpdateAt := time.Now()
	if msg.GetLastModifiedDateTime() != nil {
		lastUpdateAt = *msg.GetLastModifiedDateTime()
	}

	attachments := []Attachment{}
	for _, attachment := range msg.GetAttachments() {
		contentType := ""
		if attachment.GetContentType() != nil {
			contentType = *attachment.GetContentType()
		}
		content := ""
		if attachment.GetContent() != nil {
			content = *attachment.GetContent()
		}
		name := ""
		if attachment.GetName() != nil {
			name = *attachment.GetName()
		}
		contentURL := ""
		if attachment.GetContentUrl() != nil {
			contentURL = *attachment.GetContentUrl()
		}
		attachments = append(attachments, Attachment{
			ContentType: contentType,
			Content:     content,
			Name:        name,
			ContentURL:  contentURL,
		})
	}

	mentions := []Mention{}
	for _, m := range msg.GetMentions() {
		mention := Mention{}
		if m.GetId() != nil && m.GetMentionText() != nil {
			mention.ID = *m.GetId()
			mention.MentionedText = *m.GetMentionText()
		} else {
			continue
		}

		if m.GetMentioned() != nil && m.GetMentioned().GetUser() != nil && m.GetMentioned().GetUser().GetId() != nil {
			mention.UserID = *m.GetMentioned().GetUser().GetId()
		}

		mentions = append(mentions, mention)
	}

	reactions := []Reaction{}
	for _, reaction := range msg.GetReactions() {
		if reaction.GetReactionType() != nil && reaction.GetUser() != nil && reaction.GetUser().GetUser() != nil && reaction.GetUser().GetUser().GetId() != nil {
			reactions = append(reactions, Reaction{UserID: *reaction.GetUser().GetUser().GetId(), Reaction: *reaction.GetReactionType()})
		}
	}

	return &Message{
		ID:              msgID,
		UserID:          userID,
		UserDisplayName: userDisplayName,
		Text:            text,
		ReplyToID:       replyTo,
		Subject:         subject,
		Attachments:     attachments,
		Mentions:        mentions,
		TeamID:          teamID,
		ChannelID:       channelID,
		ChatID:          chatID,
		Reactions:       reactions,
		LastUpdateAt:    lastUpdateAt,
	}
}

func (tc *ClientImpl) GetMessage(teamID, channelID, messageID string) (*Message, error) {
	res, err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(messageID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetChatMessage(chatID, messageID string) (*Message, error) {
	res, err := tc.client.ChatsById(chatID).MessagesById(messageID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return convertToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) GetReply(teamID, channelID, messageID, replyID string) (*Message, error) {
	res, err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(messageID).RepliesById(replyID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetUserAvatar(userID string) ([]byte, error) {
	photo, err := tc.client.UsersById(userID).Photo().Content().Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return photo, nil
}

func (tc *ClientImpl) GetUser(userID string) (*User, error) {
	u, err := tc.client.UsersById(userID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	displayName := ""
	if u.GetDisplayName() != nil {
		displayName = *u.GetDisplayName()
	}

	email := ""
	if u.GetMail() != nil {
		email = *u.GetMail()
	} else if u.GetUserPrincipalName() != nil {
		email = *u.GetUserPrincipalName()
	}

	user := User{
		DisplayName: displayName,
		ID:          *u.GetId(),
		Mail:        strings.ToLower(email),
		Type:        *u.GetUserType(),
	}

	return &user, nil
}

func (tc *ClientImpl) GetFileContent(weburl string) ([]byte, error) {
	u, err := url.Parse(weburl)
	if err != nil {
		return nil, err
	}
	u.RawQuery = ""
	segments := strings.Split(u.Path, "/")

	var site models.Siteable
	for i := 3; i <= len(segments); i++ {
		path := strings.Join(segments[:i], "/")
		if len(path) == 0 || path[0] != '/' {
			path = "/" + path
		}

		site, err = sites.NewSiteItemRequestBuilder(tc.client.RequestAdapter.GetBaseUrl()+"/sites/"+u.Hostname()+":"+path+":", tc.client.RequestAdapter).Get(tc.ctx, nil)
		if err == nil {
			break
		}
	}
	if site == nil {
		return nil, fmt.Errorf("site for %s not found", weburl)
	}

	msDrives, err := tc.client.SitesById(*site.GetId()).Drives().Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	var itemRequest *drives.ItemItemsDriveItemItemRequestBuilder
	var driveID string
	for _, drive := range msDrives.GetValue() {
		// When certain file types are sent from MM to Teams and we get a change request from Teams, the URL is a bit different and in such cases, we don't execute the below if condition and "itemRequest" will be "nil" which is handled below. This will not cause any harm to the functionality, but we were unable to find why this is happening.
		if strings.HasPrefix(u.String(), *drive.GetWebUrl()) {
			path := u.String()[len(*drive.GetWebUrl()):]
			if len(path) == 0 || path[0] != '/' {
				path = "/" + path
			}
			driveID = *drive.GetId()
			itemRequest = drives.NewItemItemsDriveItemItemRequestBuilder(tc.client.RequestAdapter.GetBaseUrl()+"/drives/"+driveID+"/root:"+path, tc.client.RequestAdapter)
			break
		}
	}

	if itemRequest == nil {
		return nil, nil
	}

	item, err := itemRequest.Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	downloadURL, ok := item.GetAdditionalData()["@microsoft.graph.downloadUrl"]
	if !ok {
		return nil, errors.New("downloadUrl not found")
	}

	data, err := drives.NewItemItemsItemContentRequestBuilder(*(downloadURL.(*string)), tc.client.RequestAdapter).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return data, nil
}

func (tc *ClientImpl) GetHostedFileContent(activityIDs *ActivityIds) (contentData []byte, err error) {
	if activityIDs.ChatID != "" {
		contentData, err = tc.client.ChatsById(activityIDs.ChatID).MessagesById(activityIDs.MessageID).HostedContentsById(activityIDs.HostedContentsID).Content().Get(tc.ctx, nil)
	} else {
		if activityIDs.ReplyID != "" {
			contentData, err = tc.client.TeamsById(activityIDs.TeamID).ChannelsById(activityIDs.ChannelID).MessagesById(activityIDs.MessageID).RepliesById(activityIDs.ReplyID).HostedContentsById(activityIDs.HostedContentsID).Content().Get(tc.ctx, nil)
		} else {
			contentData, err = tc.client.TeamsById(activityIDs.TeamID).ChannelsById(activityIDs.ChannelID).MessagesById(activityIDs.MessageID).HostedContentsById(activityIDs.HostedContentsID).Content().Get(tc.ctx, nil)
		}
	}
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return
}

func (tc *ClientImpl) GetCodeSnippet(url string) (string, error) {
	// This is a hack to use the underneath machinery to do a plain request
	// with the proper session
	data, err := drives.NewItemItemsItemContentRequestBuilder(url, tc.client.RequestAdapter).Get(tc.ctx, nil)
	if err != nil {
		return "", NormalizeGraphAPIError(err)
	}
	return string(data), nil
}

func GetResourceIds(resource string) ActivityIds {
	result := ActivityIds{}
	data := strings.Split(resource, "/")

	if len(data) <= 1 {
		return result
	}

	if strings.HasPrefix(data[0], "chats(") {
		if len(data[0]) >= 9 {
			result.ChatID = data[0][7 : len(data[0])-2]
		}
		if len(data) > 1 && len(data[1]) >= 12 {
			result.MessageID = data[1][10 : len(data[1])-2]
		}
		return result
	}

	if len(data[0]) >= 9 {
		result.TeamID = data[0][7 : len(data[0])-2]
	}
	if len(data) > 1 && len(data[1]) >= 12 {
		result.ChannelID = data[1][10 : len(data[1])-2]
	}
	if len(data) > 2 && len(data[2]) >= 12 {
		result.MessageID = data[2][10 : len(data[2])-2]
	}
	if len(data) > 3 && len(data[3]) >= 11 {
		result.ReplyID = data[3][9 : len(data[3])-2]
	}
	return result
}

func (tc *ClientImpl) CreateOrGetChatForUsers(usersIDs []string) (string, error) {
	requestParameters := &users.ItemChatsRequestBuilderGetQueryParameters{
		Select: []string{"members", "id"},
		Expand: []string{"members"},
	}
	configuration := &users.ItemChatsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	res, err := tc.client.Me().Chats().Get(tc.ctx, configuration)
	if err != nil {
		return "", NormalizeGraphAPIError(err)
	}

	chatType := models.GROUP_CHATTYPE
	if len(usersIDs) == 2 {
		chatType = models.ONEONONE_CHATTYPE
	}

	for _, c := range res.GetValue() {
		if len(c.GetMembers()) == len(usersIDs) {
			matches := map[string]bool{}
			for _, m := range c.GetMembers() {
				for _, u := range usersIDs {
					userID, userErr := m.GetBackingStore().Get("userId")
					if userErr == nil && userID != nil && *(userID.(*string)) == u {
						matches[u] = true
						break
					}
				}
			}
			if len(matches) == len(usersIDs) {
				return *c.GetId(), nil
			}
		}
	}

	members := make([]models.ConversationMemberable, len(usersIDs))
	for idx, userID := range usersIDs {
		conversationMember := models.NewConversationMember()
		odataType := "#microsoft.graph.aadUserConversationMember"
		conversationMember.SetOdataType(&odataType)
		conversationMember.SetAdditionalData(map[string]interface{}{
			"user@odata.bind": "https://graph.microsoft.com/v1.0/users('" + userID + "')",
		})
		conversationMember.SetRoles([]string{"owner"})

		members[idx] = conversationMember
	}

	newChat := models.NewChat()
	newChat.SetChatType(&chatType)
	newChat.SetMembers(members)
	resn, err := tc.client.Chats().Post(tc.ctx, newChat, nil)
	if err != nil {
		return "", NormalizeGraphAPIError(err)
	}
	return *resn.GetId(), nil
}

func (tc *ClientImpl) SetChatReaction(chatID, messageID, userID, emoji string) error {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}
	setReaction := chats.NewItemMessagesItemSetReactionPostRequestBody()
	setReaction.SetReactionType(&emoji)
	setReaction.SetAdditionalData(userInfo)

	return NormalizeGraphAPIError(tc.client.ChatsById(chatID).MessagesById(messageID).SetReaction().Post(tc.ctx, setReaction, nil))
}

func (tc *ClientImpl) SetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}

	if parentID == "" {
		setReaction := teams.NewItemChannelsItemMessagesItemSetReactionPostRequestBody()
		setReaction.SetReactionType(&emoji)
		setReaction.SetAdditionalData(userInfo)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(messageID).SetReaction().Post(tc.ctx, setReaction, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	} else {
		setReaction := teams.NewItemChannelsItemMessagesItemRepliesItemSetReactionPostRequestBody()
		setReaction.SetReactionType(&emoji)
		setReaction.SetAdditionalData(userInfo)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(messageID).SetReaction().Post(tc.ctx, setReaction, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	}
	return nil
}

func (tc *ClientImpl) UnsetChatReaction(chatID, messageID, userID, emoji string) error {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}

	unsetReaction := chats.NewItemMessagesItemUnsetReactionPostRequestBody()
	unsetReaction.SetReactionType(&emoji)
	unsetReaction.SetAdditionalData(userInfo)

	return NormalizeGraphAPIError(tc.client.ChatsById(chatID).MessagesById(messageID).UnsetReaction().Post(tc.ctx, unsetReaction, nil))
}

func (tc *ClientImpl) UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}

	if parentID == "" {
		unsetReaction := teams.NewItemChannelsItemMessagesItemUnsetReactionPostRequestBody()
		unsetReaction.SetReactionType(&emoji)
		unsetReaction.SetAdditionalData(userInfo)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(messageID).UnsetReaction().Post(tc.ctx, unsetReaction, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	} else {
		unsetReaction := teams.NewItemChannelsItemMessagesItemRepliesItemUnsetReactionPostRequestBody()
		unsetReaction.SetReactionType(&emoji)
		unsetReaction.SetAdditionalData(userInfo)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(messageID).UnsetReaction().Post(tc.ctx, unsetReaction, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	}
	return nil
}

func (tc *ClientImpl) ListUsers() ([]User, error) {
	requestParameters := &users.UsersRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id", "mail", "userPrincipalName", "userType", "accountEnabled"},
	}
	configuration := &users.UsersRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.Users().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Userable](r, tc.client.GetAdapter(), models.CreateUserCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	users := []User{}
	err = pageIterator.Iterate(context.Background(), func(u models.Userable) bool {
		displayName := ""
		if u.GetDisplayName() != nil {
			displayName = *u.GetDisplayName()
		}

		user := User{
			DisplayName:      displayName,
			ID:               *u.GetId(),
			Type:             *u.GetUserType(),
			IsAccountEnabled: *u.GetAccountEnabled(),
		}

		if u.GetMail() != nil {
			user.Mail = strings.ToLower(*u.GetMail())
		} else if u.GetUserPrincipalName() != nil {
			user.Mail = strings.ToLower(*u.GetUserPrincipalName())
		}
		users = append(users, user)
		return true
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return users, nil
}

func (tc *ClientImpl) ListTeams() ([]*Team, error) {
	requestParameters := &users.ItemJoinedTeamsRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id", "description"},
	}
	configuration := &users.ItemJoinedTeamsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.Me().JoinedTeams().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Teamable](r, tc.client.GetAdapter(), models.CreateTeamCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	teams := []*Team{}
	err = pageIterator.Iterate(context.Background(), func(t models.Teamable) bool {
		description := ""
		if t.GetDescription() != nil {
			description = *t.GetDescription()
		}

		displayName := ""
		if t.GetDisplayName() != nil {
			displayName = *t.GetDisplayName()
		}

		teams = append(teams, &Team{
			DisplayName: displayName,
			Description: description,
			ID:          *t.GetId(),
		})
		return true
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return teams, nil
}

func (tc *ClientImpl) ListChannels(teamID string) ([]*Channel, error) {
	requestParameters := &teams.ItemChannelsRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id", "description"},
	}
	configuration := &teams.ItemChannelsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.TeamsById(teamID).Channels().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Channelable](r, tc.client.GetAdapter(), models.CreateChannelCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	channels := []*Channel{}
	err = pageIterator.Iterate(context.Background(), func(c models.Channelable) bool {
		description := ""
		if c.GetDescription() != nil {
			description = *c.GetDescription()
		}

		displayName := ""
		if c.GetDisplayName() != nil {
			displayName = *c.GetDisplayName()
		}

		channels = append(channels, &Channel{
			DisplayName: displayName,
			Description: description,
			ID:          *c.GetId(),
		})
		return true
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return channels, nil
}

func GetAuthURL(redirectURL string, tenantID string, clientID string, clientSecret string, state string, codeVerifier string) string {
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       append(teamsDefaultScopes, "offline_access"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantID),
			TokenURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantID),
		},
		RedirectURL: redirectURL,
	}

	sha2 := sha256.New()
	_, _ = io.WriteString(sha2, codeVerifier)
	codeChallenge := base64.RawURLEncoding.EncodeToString(sha2.Sum(nil))

	return conf.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "select_account"),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
	)
}
