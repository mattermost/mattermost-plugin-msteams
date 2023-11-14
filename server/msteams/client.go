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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	azidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	pluginapi "github.com/mattermost/mattermost-plugin-api"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/microsoft/kiota-abstractions-go/serialization"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	a "github.com/microsoftgraph/msgraph-sdk-go-core/authentication"
	"github.com/microsoftgraph/msgraph-sdk-go/chats"
	"github.com/microsoftgraph/msgraph-sdk-go/drives"
	"github.com/microsoftgraph/msgraph-sdk-go/groups"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
	"github.com/microsoftgraph/msgraph-sdk-go/sites"
	"github.com/microsoftgraph/msgraph-sdk-go/teams"
	"github.com/microsoftgraph/msgraph-sdk-go/users"
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
	logService   *pluginapi.LogService
}

type Activity struct {
	Resource                       string
	ClientState                    string
	ChangeType                     string
	LifecycleEvent                 string
	SubscriptionExpirationDateTime time.Time
	SubscriptionID                 string
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
		if terr := e.GetErrorEscaped(); terr != nil {
			code, message := "", ""
			if terr.GetCode() != nil {
				code = *terr.GetCode()
			}
			if terr.GetMessage() != nil {
				message = *terr.GetMessage()
			}
			return &GraphAPIError{
				Code:    code,
				Message: message,
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

func NewApp(tenantID, clientID, clientSecret string, logService *pluginapi.LogService) Client {
	return &ClientImpl{
		ctx:          context.Background(),
		clientType:   "app",
		tenantID:     tenantID,
		clientID:     clientID,
		clientSecret: clientSecret,
		logService:   logService,
	}
}

func NewTokenClient(redirectURL, tenantID, clientID, clientSecret string, token *oauth2.Token, logService *pluginapi.LogService) Client {
	client := &ClientImpl{
		ctx:        context.Background(),
		clientType: "token",
		tenantID:   tenantID,
		clientID:   clientID,
		token:      token,
		logService: logService,
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
		logService.Error("Unable to create the client from the token", "error", err)
		return nil
	}

	adapter, err := msgraphsdk.NewGraphRequestAdapter(auth)
	if err != nil {
		logService.Error("Unable to create the client from the token", "error", err)
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

	if r.GetId() == nil {
		tc.logService.Debug("Received nil user ID from MS Graph for current user")
		return "", errors.New("empty user ID")
	}

	return *r.GetId(), nil
}

func (tc *ClientImpl) GetMe() (*clientmodels.User, error) {
	requestParameters := &users.UserItemRequestBuilderGetQueryParameters{
		Select: []string{"id", "mail", "userPrincipalName"},
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

	if mail == nil || *mail == "" {
		tc.logService.Debug("User principal name and mail both are empty for current user")
		return nil, errors.New("empty user mail and principal name")
	}

	if r.GetId() == nil {
		tc.logService.Debug("Received nil user ID from MS Graph for current user")
		return nil, errors.New("empty user ID")
	}

	displayName := r.GetDisplayName()
	user := &clientmodels.User{ID: *r.GetId()}
	user.Mail = strings.ToLower(*mail)
	if displayName != nil {
		user.DisplayName = *displayName
	}

	return user, nil
}

func (tc *ClientImpl) SendMessage(teamID, channelID, parentID, message string) (*clientmodels.Message, error) {
	return tc.SendMessageWithAttachments(teamID, channelID, parentID, message, nil, nil)
}

func (tc *ClientImpl) SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
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
				tc.logService.Error("Unable to parse URL", "Error", err.Error())
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
		res, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().Post(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		var err error
		res, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().Post(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) SendChat(chatID, message string, parentMessage *clientmodels.Message, attachments []*clientmodels.Attachment, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	rmsg := models.NewChatMessage()

	msteamsAttachments := []models.ChatMessageAttachmentable{}
	if parentMessage != nil && parentMessage.ID != "" {
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
			tc.logService.Error("Unable to convert content to JSON", "error", err)
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
				tc.logService.Error("Unable to parse URL", "Error", err.Error())
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

	res, err := tc.client.Chats().ByChatId(chatID).Messages().Post(tc.ctx, rmsg, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return convertToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) UploadFile(teamID, channelID, filename string, filesize int, mimeType string, data io.Reader, chat *clientmodels.Chat) (*clientmodels.Attachment, error) {
	driveID := ""
	itemID := ""
	if teamID != "" && channelID != "" {
		folderInfo, err := tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).FilesFolder().Get(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}

		if folderInfo.GetParentReference().GetDriveId() != nil {
			driveID = *folderInfo.GetParentReference().GetDriveId()
		}
		if folderInfo.GetId() != nil {
			itemID = *folderInfo.GetId() + ":/" + filename + ":"
		}
	} else {
		drive, err := tc.client.Me().Drive().Get(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}

		if drive.GetId() != nil {
			driveID = *drive.GetId()
		}
		rootDirectory, err := tc.client.Drives().ByDriveId(driveID).Root().Get(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}

		var chatFolder models.DriveItemable
		var cErr error
		folderName := "Microsoft Teams Chat Files"
		rootDirectoryID := ""
		if rootDirectory.GetId() != nil {
			rootDirectoryID = *rootDirectory.GetId()
			itemID = rootDirectoryID + ":/" + folderName
		}
		chatFolder, cErr = tc.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemID).Get(tc.ctx, nil)
		if cErr != nil {
			err := NormalizeGraphAPIError(cErr)
			if !strings.Contains(err.Error(), "itemNotFound") {
				return nil, err
			}

			// Create chat folder
			folderRequestBody := models.NewDriveItem()
			folderRequestBody.SetName(&folderName)
			folder := models.NewFolder()
			folderRequestBody.SetFolder(folder)
			additionalData := map[string]interface{}{
				"microsoftGraphConflictBehavior": "fail",
			}

			folderRequestBody.SetAdditionalData(additionalData)
			chatFolder, cErr = tc.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(rootDirectoryID).Children().Post(tc.ctx, folderRequestBody, nil)
			if cErr != nil {
				return nil, NormalizeGraphAPIError(cErr)
			}
		}

		if chatFolder.GetId() != nil {
			itemID = *chatFolder.GetId() + ":/" + filename + ":"
		}
	}

	uploadSession, err := tc.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemID).CreateUploadSession().Post(tc.ctx, nil, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if uploadSession.GetUploadUrl() == nil {
		return nil, errors.New("unable to upload file as upload URL is empty")
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

	if chat != nil {
		requestBody := drives.NewItemItemsItemInvitePostRequestBody()
		recipients := []models.DriveRecipientable{}
		for _, chatUser := range chat.Members {
			driveRecipient := models.NewDriveRecipient()
			email := chatUser.Email
			driveRecipient.SetEmail(&email)
			recipients = append(recipients, driveRecipient)
		}

		requestBody.SetRecipients(recipients)
		requireSignIn := true
		requestBody.SetRequireSignIn(&requireSignIn)
		roles := []string{"read", "write"}
		requestBody.SetRoles(roles)
		if _, err = tc.client.Drives().ByDriveId(driveID).Items().ByDriveItemId(itemID).Invite().Post(tc.ctx, requestBody, nil); err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}

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

	attachment := clientmodels.Attachment{
		ID:          uploadedFile.ETag[2:38],
		Name:        uploadedFile.Name,
		ContentURL:  uploadedFile.WebURL,
		ContentType: mimeType,
	}

	return &attachment, nil
}

func (tc *ClientImpl) DeleteMessage(teamID, channelID, parentID, msgID string) error {
	if parentID != "" {
		if err := tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(msgID).Delete(tc.ctx, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	} else {
		if err := tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(msgID).Delete(tc.ctx, nil); err != nil {
			return NormalizeGraphAPIError(err)
		}
	}
	return nil
}

func (tc *ClientImpl) DeleteChatMessage(chatID, msgID string) error {
	return NormalizeGraphAPIError(tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(msgID).Delete(tc.ctx, nil))
}

func (tc *ClientImpl) UpdateMessage(teamID, channelID, parentID, msgID, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	rmsg := models.NewChatMessage()

	contentType := models.HTML_BODYTYPE
	rmsg.SetMentions(mentions)

	var originalMessage models.ChatMessageable
	var err error
	if parentID != "" {
		originalMessage, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(msgID).Get(tc.ctx, nil)
	} else {
		originalMessage, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(msgID).Get(tc.ctx, nil)
	}
	if err != nil {
		tc.logService.Error("Error in getting original message from Teams", "error", NormalizeGraphAPIError(err))
	}

	if originalMessage != nil {
		attachments := originalMessage.GetAttachments()
		for _, a := range attachments {
			if a.GetId() != nil {
				message = fmt.Sprintf("<attachment id=%q></attachment> %s", *a.GetId(), message)
			}
		}
		rmsg.SetAttachments(attachments)
	}

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&message)
	rmsg.SetBody(body)

	var updateMessageRequest *abstractions.RequestInformation
	if parentID != "" {
		updateMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(msgID).ToPatchRequestInformation(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		updateMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(msgID).ToPatchRequestInformation(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}

	if updateMessageRequest == nil {
		return nil, errors.New("received nil updateMessageRequest from MS Graph")
	}

	var getMessageRequest *abstractions.RequestInformation
	if parentID != "" {
		getMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(msgID).ToGetRequestInformation(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		getMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(msgID).ToGetRequestInformation(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}

	if getMessageRequest == nil {
		return nil, errors.New("received nil getMessageRequest from MS Graph")
	}

	batchRequest := msgraphcore.NewBatchRequest(tc.client.GetAdapter())
	updateMessageRequestItem, err := batchRequest.AddBatchRequestStep(*updateMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	getMessageRequestItem, err := batchRequest.AddBatchRequestStep(*getMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	getMessageRequestItem.DependsOnItem(updateMessageRequestItem)

	return tc.SendBatchRequestAndGetMessage(batchRequest, getMessageRequestItem)
}

func (tc *ClientImpl) UpdateChatMessage(chatID, msgID, message string, mentions []models.ChatMessageMentionable) (*clientmodels.Message, error) {
	rmsg := models.NewChatMessage()

	originalMessage, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(msgID).Get(tc.ctx, nil)
	if err != nil {
		tc.logService.Error("Error in getting original message from Teams", "error", NormalizeGraphAPIError(err))
	}

	if originalMessage != nil {
		attachments := originalMessage.GetAttachments()
		for _, a := range attachments {
			if a.GetId() != nil {
				message = fmt.Sprintf("<attachment id=%q></attachment> %s", *a.GetId(), message)
			}
		}
		rmsg.SetAttachments(attachments)
	}

	contentType := models.HTML_BODYTYPE

	rmsg.SetMentions(mentions)

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&message)
	rmsg.SetBody(body)

	updateMessageRequest, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(msgID).ToPatchRequestInformation(tc.ctx, rmsg, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if updateMessageRequest == nil {
		return nil, errors.New("received nil updateMessageRequest from MS Graph")
	}

	getMessageRequest, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(msgID).ToGetRequestInformation(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if getMessageRequest == nil {
		return nil, errors.New("received nil getMessageRequest from MS Graph")
	}

	batchRequest := msgraphcore.NewBatchRequest(tc.client.GetAdapter())
	updateMessageRequestItem, err := batchRequest.AddBatchRequestStep(*updateMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	getMessageRequestItem, err := batchRequest.AddBatchRequestStep(*getMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	getMessageRequestItem.DependsOnItem(updateMessageRequestItem)

	return tc.SendBatchRequestAndGetMessage(batchRequest, getMessageRequestItem)
}

func (tc *ClientImpl) subscribe(baseURL, webhookSecret, resource, changeType string) (*clientmodels.Subscription, error) {
	expirationDateTime := time.Now().Add(30 * time.Minute)

	lifecycleNotificationURL := baseURL + "lifecycle"
	notificationURL := baseURL + "changes"

	subscription := models.NewSubscription()
	subscription.SetResource(&resource)
	subscription.SetExpirationDateTime(&expirationDateTime)
	subscription.SetNotificationUrl(&notificationURL)
	subscription.SetLifecycleNotificationUrl(&lifecycleNotificationURL)
	subscription.SetClientState(&webhookSecret)
	subscription.SetChangeType(&changeType)

	res, err := tc.client.Subscriptions().Post(tc.ctx, subscription, nil)
	if err != nil {
		tc.logService.Error("Unable to create the new subscription", "error", NormalizeGraphAPIError(err))
		return nil, NormalizeGraphAPIError(err)
	}

	if res.GetId() == nil {
		return nil, errors.New("empty subscription ID received from MS Graph while creating subscription")
	}
	if res.GetExpirationDateTime() == nil {
		return nil, errors.New("empty subscription expiration time received from MS Graph while creating subscription")
	}

	return &clientmodels.Subscription{
		ID:        *res.GetId(),
		ExpiresOn: *res.GetExpirationDateTime(),
	}, nil
}

func (tc *ClientImpl) SubscribeToChannels(baseURL, webhookSecret string, pay bool) (*clientmodels.Subscription, error) {
	resource := "teams/getAllMessages"
	if pay {
		resource = "teams/getAllMessages?model=B"
	}
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) SubscribeToChannel(teamID, channelID, baseURL, webhookSecret string) (*clientmodels.Subscription, error) {
	resource := fmt.Sprintf("/teams/%s/channels/%s/messages", teamID, channelID)
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) SubscribeToChats(baseURL, webhookSecret string, pay bool) (*clientmodels.Subscription, error) {
	resource := "chats/getAllMessages"
	if pay {
		resource = "chats/getAllMessages?model=B"
	}
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) SubscribeToUserChats(userID, baseURL, webhookSecret string, pay bool) (*clientmodels.Subscription, error) {
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
	if _, err := tc.client.Subscriptions().BySubscriptionId(subscriptionID).Patch(tc.ctx, updatedSubscription, nil); err != nil {
		tc.logService.Error("Unable to refresh the subscription", "error", NormalizeGraphAPIError(err), "subscriptionID", subscriptionID)
		return nil, NormalizeGraphAPIError(err)
	}
	return &expirationDateTime, nil
}

func (tc *ClientImpl) DeleteSubscription(subscriptionID string) error {
	if err := tc.client.Subscriptions().BySubscriptionId(subscriptionID).Delete(tc.ctx, nil); err != nil {
		tc.logService.Error("Unable to delete the subscription", "error", NormalizeGraphAPIError(err), "subscriptionID", subscriptionID)
		return NormalizeGraphAPIError(err)
	}
	return nil
}

func (tc *ClientImpl) ListSubscriptions() ([]*clientmodels.Subscription, error) {
	r, err := tc.client.Subscriptions().Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Subscriptionable](r, tc.client.GetAdapter(), models.CreateSubscriptionCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	subscriptions := []*clientmodels.Subscription{}
	err = pageIterator.Iterate(tc.ctx, func(subscription models.Subscriptionable) bool {
		subscriptionID := ""
		resource := ""
		notificationURL := ""
		var expiresOn time.Time
		if subscription != nil {
			if subscription.GetId() != nil {
				subscriptionID = *subscription.GetId()
			}

			if subscription.GetExpirationDateTime() != nil {
				expiresOn = *subscription.GetExpirationDateTime()
			}

			if subscription.GetResource() != nil {
				resource = *subscription.GetResource()
			}

			if subscription.GetNotificationUrl() != nil {
				notificationURL = *subscription.GetNotificationUrl()
			}
		}

		subscriptions = append(subscriptions, &clientmodels.Subscription{
			ID:              subscriptionID,
			Resource:        resource,
			NotificationURL: notificationURL,
			ExpiresOn:       expiresOn,
		})

		return true
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return subscriptions, nil
}

func (tc *ClientImpl) GetTeam(teamID string) (*clientmodels.Team, error) {
	res, err := tc.client.Teams().ByTeamId(teamID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	displayName := ""
	if res.GetDisplayName() != nil {
		displayName = *res.GetDisplayName()
	}

	return &clientmodels.Team{ID: teamID, DisplayName: displayName}, nil
}

func (tc *ClientImpl) GetTeams(filterQuery string) ([]*clientmodels.Team, error) {
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
	teams := make([]*clientmodels.Team, len(msTeamsGroups))
	for idx, group := range msTeamsGroups {
		if group.GetId() == nil {
			continue
		}

		team := &clientmodels.Team{ID: *group.GetId()}
		if group.GetDisplayName() != nil {
			team.DisplayName = *group.GetDisplayName()
		}
		teams[idx] = team
	}

	return teams, nil
}

func (tc *ClientImpl) GetChannelInTeam(teamID, channelID string) (*clientmodels.Channel, error) {
	res, err := tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	displayName := ""
	if res.GetDisplayName() != nil {
		displayName = *res.GetDisplayName()
	}

	return &clientmodels.Channel{ID: channelID, DisplayName: displayName}, nil
}

func (tc *ClientImpl) GetChannelsInTeam(teamID, filterQuery string) ([]*clientmodels.Channel, error) {
	requestParameters := &teams.ItemChannelsRequestBuilderGetQueryParameters{
		Filter: &filterQuery,
		Select: []string{"id", "displayName"},
	}

	configuration := &teams.ItemChannelsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}

	res, err := tc.client.Teams().ByTeamId(teamID).Channels().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	msTeamsChannels := res.GetValue()
	channels := make([]*clientmodels.Channel, len(msTeamsChannels))
	for idx, teamsChannel := range msTeamsChannels {
		if teamsChannel.GetId() == nil {
			continue
		}

		channel := &clientmodels.Channel{ID: *teamsChannel.GetId()}
		if teamsChannel.GetDisplayName() != nil {
			channel.DisplayName = *teamsChannel.GetDisplayName()
		}
		channels[idx] = channel
	}

	return channels, nil
}

func (tc *ClientImpl) GetChat(chatID string) (*clientmodels.Chat, error) {
	requestParameters := &chats.ChatItemRequestBuilderGetQueryParameters{
		Expand: []string{"members"},
	}
	configuration := &chats.ChatItemRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	res, err := tc.client.Chats().ByChatId(chatID).Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	chatType := ""
	if res.GetChatType() != nil && *res.GetChatType() == models.GROUP_CHATTYPE {
		chatType = "G"
	} else if res.GetChatType() != nil && *res.GetChatType() == models.ONEONONE_CHATTYPE {
		chatType = "D"
	}

	members := []clientmodels.ChatMember{}
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

		members = append(members, clientmodels.ChatMember{
			DisplayName: displayName,
			UserID:      *(userID.(*string)),
			Email:       *(email.(*string)),
		})
	}

	return &clientmodels.Chat{ID: chatID, Members: members, Type: chatType}, nil
}

func convertToMessage(msg models.ChatMessageable, teamID, channelID, chatID string) *clientmodels.Message {
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

	createAt := time.Now()
	if msg.GetCreatedDateTime() != nil {
		createAt = *msg.GetCreatedDateTime()
	}

	lastUpdateAt := time.Now()
	if msg.GetLastModifiedDateTime() != nil {
		lastUpdateAt = *msg.GetLastModifiedDateTime()
	}

	attachments := []clientmodels.Attachment{}
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
		attachments = append(attachments, clientmodels.Attachment{
			ContentType: contentType,
			Content:     content,
			Name:        name,
			ContentURL:  contentURL,
		})
	}

	mentions := []clientmodels.Mention{}
	for _, m := range msg.GetMentions() {
		mention := clientmodels.Mention{}
		if m.GetId() != nil && m.GetMentionText() != nil {
			mention.ID = *m.GetId()
			mention.MentionedText = *m.GetMentionText()
		} else {
			continue
		}

		if m.GetMentioned() != nil {
			if m.GetMentioned().GetUser() != nil && m.GetMentioned().GetUser().GetId() != nil {
				mention.UserID = *m.GetMentioned().GetUser().GetId()
			}

			if m.GetMentioned().GetConversation() != nil && m.GetMentioned().GetConversation().GetId() != nil {
				mention.ConversationID = *m.GetMentioned().GetConversation().GetId()
			}
		}

		mentions = append(mentions, mention)
	}

	reactions := []clientmodels.Reaction{}
	for _, reaction := range msg.GetReactions() {
		if reaction.GetReactionType() != nil && reaction.GetUser() != nil && reaction.GetUser().GetUser() != nil && reaction.GetUser().GetUser().GetId() != nil {
			reactions = append(reactions, clientmodels.Reaction{UserID: *reaction.GetUser().GetUser().GetId(), Reaction: *reaction.GetReactionType()})
		}
	}

	return &clientmodels.Message{
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
		CreateAt:        createAt,
		LastUpdateAt:    lastUpdateAt,
	}
}

func (tc *ClientImpl) GetMessage(teamID, channelID, messageID string) (*clientmodels.Message, error) {
	res, err := tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(messageID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetChatMessage(chatID, messageID string) (*clientmodels.Message, error) {
	res, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return convertToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) GetReply(teamID, channelID, messageID, replyID string) (*clientmodels.Message, error) {
	res, err := tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(messageID).Replies().ByChatMessageId1(replyID).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetUserAvatar(userID string) ([]byte, error) {
	photo, err := tc.client.Users().ByUserId(userID).Photo().Content().Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return photo, nil
}

func (tc *ClientImpl) GetUser(userID string) (*clientmodels.User, error) {
	requestParameters := &users.UserItemRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id", "mail", "userPrincipalName", "userType"},
	}

	configuration := &users.UserItemRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}

	u, err := tc.client.Users().ByUserId(userID).Get(tc.ctx, configuration)
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

	userType := ""
	if u.GetUserType() != nil {
		userType = *u.GetUserType()
	}

	if u.GetId() == nil {
		tc.logService.Debug("Received empty user ID from MS Graph", "UserID", userID)
		return nil, errors.New("received empty user ID from MS Graph")
	}
	user := clientmodels.User{
		DisplayName: displayName,
		ID:          *u.GetId(),
		Mail:        strings.ToLower(email),
		Type:        userType,
	}

	return &user, nil
}

func (tc *ClientImpl) GetFileSizeAndDownloadURL(weburl string) (int64, string, error) {
	u, err := url.Parse(weburl)
	if err != nil {
		return 0, "", err
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
		return 0, "", fmt.Errorf("site for %s not found", weburl)
	}

	siteID := ""
	if site.GetId() != nil {
		siteID = *site.GetId()
	}
	msDrives, err := tc.client.Sites().BySiteId(siteID).Drives().Get(tc.ctx, nil)
	if err != nil {
		return 0, "", NormalizeGraphAPIError(err)
	}
	var itemRequest *drives.ItemItemsDriveItemItemRequestBuilder
	var driveID string
	for _, drive := range msDrives.GetValue() {
		if drive.GetWebUrl() == nil {
			continue
		}

		// When certain file types are sent from MM to Teams and we get a change request from Teams, the URL is a bit different and in such cases, we don't execute the below if condition and "itemRequest" will be "nil" which is handled below. This will not cause any harm to the functionality, but we were unable to find why this is happening.
		if strings.HasPrefix(u.String(), *drive.GetWebUrl()) {
			path := u.String()[len(*drive.GetWebUrl()):]
			if len(path) == 0 || path[0] != '/' {
				path = "/" + path
			}

			if drive.GetId() == nil {
				continue
			}

			driveID = *drive.GetId()
			itemRequest = drives.NewItemItemsDriveItemItemRequestBuilder(tc.client.RequestAdapter.GetBaseUrl()+"/drives/"+driveID+"/root:"+path, tc.client.RequestAdapter)
			break
		}
	}

	if itemRequest == nil {
		return 0, "", nil
	}

	item, err := itemRequest.Get(tc.ctx, nil)
	if err != nil {
		return 0, "", NormalizeGraphAPIError(err)
	}
	downloadURL, ok := item.GetAdditionalData()["@microsoft.graph.downloadUrl"]
	if !ok || downloadURL == nil {
		return 0, "", errors.New("downloadUrl not found")
	}

	resultDownloadURL := ""
	if downloadURL.(*string) != nil {
		resultDownloadURL = *(downloadURL.(*string))
	}

	fileSize := item.GetSize()
	if fileSize == nil {
		return 0, resultDownloadURL, nil
	}

	return *fileSize, resultDownloadURL, nil
}

func (tc *ClientImpl) GetFileContent(downloadURL string) ([]byte, error) {
	data, err := drives.NewItemItemsItemContentRequestBuilder(downloadURL, tc.client.RequestAdapter).Get(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return data, nil
}

func (tc *ClientImpl) GetFileContentStream(downloadURL string, writer *io.PipeWriter, bufferSize int64) {
	rangeStart := int64(0)
	// Get only limited amount of data from the API call in one iteration
	rangeIncrement := bufferSize
	for {
		req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
		if err != nil {
			tc.logService.Error("unable to create new request", "error", err.Error())
			return
		}

		contentRange := fmt.Sprintf("bytes=%d-%d", rangeStart, rangeStart+rangeIncrement-1)
		req.Header.Add("Range", contentRange)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			tc.logService.Error("unable to send request for getting file content", "error", err.Error())
			return
		}

		if _, err = io.Copy(writer, res.Body); err != nil {
			tc.logService.Error("unable to copy response body to the writer", "error", err.Error())
			return
		}

		res.Body.Close()
		contentLengthHeader := res.Header.Get("Content-Length")
		contentLength, err := strconv.ParseInt(contentLengthHeader, 10, 64)
		if (err == nil && contentLength < rangeIncrement) || res.StatusCode != http.StatusPartialContent {
			writer.Close()
			break
		}

		rangeStart += rangeIncrement
	}
}

func (tc *ClientImpl) GetHostedFileContent(activityIDs *clientmodels.ActivityIds) (contentData []byte, err error) {
	if activityIDs.ChatID != "" {
		contentData, err = tc.client.Chats().ByChatId(activityIDs.ChatID).Messages().ByChatMessageId(activityIDs.MessageID).HostedContents().ByChatMessageHostedContentId(activityIDs.HostedContentsID).Content().Get(tc.ctx, nil)
	} else {
		if activityIDs.ReplyID != "" {
			contentData, err = tc.client.Teams().ByTeamId(activityIDs.TeamID).Channels().ByChannelId(activityIDs.ChannelID).Messages().ByChatMessageId(activityIDs.MessageID).Replies().ByChatMessageId1(activityIDs.ReplyID).HostedContents().ByChatMessageHostedContentId(activityIDs.HostedContentsID).Content().Get(tc.ctx, nil)
		} else {
			contentData, err = tc.client.Teams().ByTeamId(activityIDs.TeamID).Channels().ByChannelId(activityIDs.ChannelID).Messages().ByChatMessageId(activityIDs.MessageID).HostedContents().ByChatMessageHostedContentId(activityIDs.HostedContentsID).Content().Get(tc.ctx, nil)
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

func GetResourceIds(resource string) clientmodels.ActivityIds {
	result := clientmodels.ActivityIds{}
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

func (tc *ClientImpl) CreateOrGetChatForUsers(userIDs []string) (*clientmodels.Chat, error) {
	if len(userIDs) == 2 {
		return tc.CreateChat(models.ONEONONE_CHATTYPE, userIDs)
	}

	requestParameters := &users.ItemChatsRequestBuilderGetQueryParameters{
		Select: []string{"members", "id"},
		Expand: []string{"members"},
	}

	configuration := &users.ItemChatsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}

	res, err := tc.client.Me().Chats().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	for _, c := range res.GetValue() {
		chat := checkGroupChat(c, userIDs)
		if chat != nil {
			return chat, nil
		}
	}

	pageIterator, err := msgraphcore.NewPageIterator[*models.Chat](res, tc.client.GetAdapter(), models.CreateChatCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	var chat *clientmodels.Chat
	err = pageIterator.Iterate(tc.ctx, func(c *models.Chat) bool {
		chat = checkGroupChat(c, userIDs)
		return chat == nil
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if chat != nil {
		return chat, nil
	}

	return tc.CreateChat(models.GROUP_CHATTYPE, userIDs)
}

func (tc *ClientImpl) CreateChat(chatType models.ChatType, userIDs []string) (*clientmodels.Chat, error) {
	members := make([]models.ConversationMemberable, len(userIDs))
	for idx, userID := range userIDs {
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
	newChat.SetMembers(members)
	newChat.SetChatType(&chatType)
	chat, err := tc.client.Chats().Post(tc.ctx, newChat, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if chat.GetId() == nil {
		return nil, errors.New("received empty chat ID from MS Graph while creating chat")
	}

	chatDetails, err := tc.GetChat(*chat.GetId())
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	return chatDetails, nil
}

func (tc *ClientImpl) SetChatReaction(chatID, messageID, userID, emoji string) (*clientmodels.Message, error) {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}
	setReaction := chats.NewItemMessagesItemSetReactionPostRequestBody()
	setReaction.SetReactionType(&emoji)
	setReaction.SetAdditionalData(userInfo)

	setReactionRequest, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).SetReaction().ToPostRequestInformation(tc.ctx, setReaction, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if setReactionRequest == nil {
		return nil, errors.New("received nil setReactionRequest from MS Graph")
	}

	getMessageRequest, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).ToGetRequestInformation(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if getMessageRequest == nil {
		return nil, errors.New("received nil getMessageRequest from MS Graph")
	}

	batchRequest := msgraphcore.NewBatchRequest(tc.client.GetAdapter())
	setReactionRequestItem, err := batchRequest.AddBatchRequestStep(*setReactionRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	getMessageRequestItem, err := batchRequest.AddBatchRequestStep(*getMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	getMessageRequestItem.DependsOnItem(setReactionRequestItem)

	return tc.SendBatchRequestAndGetMessage(batchRequest, getMessageRequestItem)
}

func (tc *ClientImpl) SetReaction(teamID, channelID, parentID, messageID, userID, emoji string) (*clientmodels.Message, error) {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}

	var setReactionRequest *abstractions.RequestInformation
	var err error
	if parentID == "" {
		setReaction := teams.NewItemChannelsItemMessagesItemSetReactionPostRequestBody()
		setReaction.SetReactionType(&emoji)
		setReaction.SetAdditionalData(userInfo)

		setReactionRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(messageID).SetReaction().ToPostRequestInformation(tc.ctx, setReaction, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		setReaction := teams.NewItemChannelsItemMessagesItemRepliesItemSetReactionPostRequestBody()
		setReaction.SetReactionType(&emoji)
		setReaction.SetAdditionalData(userInfo)

		setReactionRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(messageID).SetReaction().ToPostRequestInformation(tc.ctx, setReaction, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}

	if setReactionRequest == nil {
		return nil, errors.New("received nil setReactionRequest from MS Graph")
	}

	var getMessageRequest *abstractions.RequestInformation
	if parentID != "" {
		getMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(messageID).ToGetRequestInformation(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		getMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(messageID).ToGetRequestInformation(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}

	if getMessageRequest == nil {
		return nil, errors.New("received nil getMessageRequest from MS Graph")
	}

	batchRequest := msgraphcore.NewBatchRequest(tc.client.GetAdapter())
	setReactionRequestItem, err := batchRequest.AddBatchRequestStep(*setReactionRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	getMessageRequestItem, err := batchRequest.AddBatchRequestStep(*getMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	getMessageRequestItem.DependsOnItem(setReactionRequestItem)

	return tc.SendBatchRequestAndGetMessage(batchRequest, getMessageRequestItem)
}

func (tc *ClientImpl) UnsetChatReaction(chatID, messageID, userID, emoji string) (*clientmodels.Message, error) {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}

	unsetReaction := chats.NewItemMessagesItemUnsetReactionPostRequestBody()
	unsetReaction.SetReactionType(&emoji)
	unsetReaction.SetAdditionalData(userInfo)

	unsetReactionRequest, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).UnsetReaction().ToPostRequestInformation(tc.ctx, unsetReaction, nil)
	if err != nil {
		return nil, err
	}

	if unsetReactionRequest == nil {
		return nil, errors.New("received nil unsetReactionRequest from MS Graph")
	}

	getMessageRequest, err := tc.client.Chats().ByChatId(chatID).Messages().ByChatMessageId(messageID).ToGetRequestInformation(tc.ctx, nil)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if getMessageRequest == nil {
		return nil, errors.New("received nil getMessageRequest from MS Graph")
	}

	batchRequest := msgraphcore.NewBatchRequest(tc.client.GetAdapter())
	unsetReactionRequestItem, err := batchRequest.AddBatchRequestStep(*unsetReactionRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	getMessageRequestItem, err := batchRequest.AddBatchRequestStep(*getMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	getMessageRequestItem.DependsOnItem(unsetReactionRequestItem)

	return tc.SendBatchRequestAndGetMessage(batchRequest, getMessageRequestItem)
}

func (tc *ClientImpl) UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji string) (*clientmodels.Message, error) {
	userInfo := map[string]any{
		"user": map[string]string{
			"id": userID,
		},
	}

	var unsetReactionRequest *abstractions.RequestInformation
	var err error
	if parentID == "" {
		unsetReaction := teams.NewItemChannelsItemMessagesItemUnsetReactionPostRequestBody()
		unsetReaction.SetReactionType(&emoji)
		unsetReaction.SetAdditionalData(userInfo)

		unsetReactionRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(messageID).UnsetReaction().ToPostRequestInformation(tc.ctx, unsetReaction, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		unsetReaction := teams.NewItemChannelsItemMessagesItemRepliesItemUnsetReactionPostRequestBody()
		unsetReaction.SetReactionType(&emoji)
		unsetReaction.SetAdditionalData(userInfo)

		unsetReactionRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(messageID).UnsetReaction().ToPostRequestInformation(tc.ctx, unsetReaction, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}

	if unsetReactionRequest == nil {
		return nil, errors.New("received nil unsetReactionRequest from MS Graph")
	}

	var getMessageRequest *abstractions.RequestInformation
	if parentID != "" {
		getMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(parentID).Replies().ByChatMessageId1(messageID).ToGetRequestInformation(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	} else {
		getMessageRequest, err = tc.client.Teams().ByTeamId(teamID).Channels().ByChannelId(channelID).Messages().ByChatMessageId(messageID).ToGetRequestInformation(tc.ctx, nil)
		if err != nil {
			return nil, NormalizeGraphAPIError(err)
		}
	}

	if getMessageRequest == nil {
		return nil, errors.New("received nil getMessageRequest from MS Graph")
	}

	batchRequest := msgraphcore.NewBatchRequest(tc.client.GetAdapter())
	unsetReactionRequestItem, err := batchRequest.AddBatchRequestStep(*unsetReactionRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	getMessageRequestItem, err := batchRequest.AddBatchRequestStep(*getMessageRequest)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	getMessageRequestItem.DependsOnItem(unsetReactionRequestItem)

	return tc.SendBatchRequestAndGetMessage(batchRequest, getMessageRequestItem)
}

func (tc *ClientImpl) ListUsers() ([]clientmodels.User, error) {
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

	users := []clientmodels.User{}
	err = pageIterator.Iterate(context.Background(), func(u models.Userable) bool {
		user := clientmodels.User{}
		if u.GetDisplayName() != nil {
			user.DisplayName = *u.GetDisplayName()
		}
		if u.GetId() != nil {
			user.ID = *u.GetId()
		}
		if u.GetUserType() != nil {
			user.Type = *u.GetUserType()
		}
		if u.GetAccountEnabled() != nil {
			user.IsAccountEnabled = *u.GetAccountEnabled()
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

func (tc *ClientImpl) ListTeams() ([]clientmodels.Team, error) {
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

	teams := []clientmodels.Team{}
	err = pageIterator.Iterate(context.Background(), func(t models.Teamable) bool {
		team := clientmodels.Team{}
		if t.GetId() != nil {
			team.ID = *t.GetId()
		}
		if t.GetDescription() != nil {
			team.Description = *t.GetDescription()
		}
		if t.GetDisplayName() != nil {
			team.DisplayName = *t.GetDisplayName()
		}

		teams = append(teams, team)
		return true
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return teams, nil
}

func (tc *ClientImpl) ListChannels(teamID string) ([]clientmodels.Channel, error) {
	requestParameters := &teams.ItemChannelsRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id", "description"},
	}
	configuration := &teams.ItemChannelsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.Teams().ByTeamId(teamID).Channels().Get(tc.ctx, configuration)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	pageIterator, err := msgraphcore.NewPageIterator[models.Channelable](r, tc.client.GetAdapter(), models.CreateChannelCollectionResponseFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	channels := []clientmodels.Channel{}
	err = pageIterator.Iterate(context.Background(), func(c models.Channelable) bool {
		channel := clientmodels.Channel{}
		if c.GetId() != nil {
			channel.ID = *c.GetId()
		}
		if c.GetDescription() != nil {
			channel.Description = *c.GetDescription()
		}
		if c.GetDisplayName() != nil {
			channel.DisplayName = *c.GetDisplayName()
		}

		channels = append(channels, channel)
		return true
	})
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}
	return channels, nil
}

func (tc *ClientImpl) SendBatchRequestAndGetMessage(batchRequest msgraphcore.BatchRequest, getMessageRequestItem msgraphcore.BatchItem) (*clientmodels.Message, error) {
	batchResponse, err := batchRequest.Send(tc.ctx, tc.client.GetAdapter())
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	resp, err := msgraphcore.GetBatchResponseById[*models.ChatMessage](batchResponse, *getMessageRequestItem.GetId(), models.CreateChatMessageFromDiscriminatorValue)
	if err != nil {
		return nil, NormalizeGraphAPIError(err)
	}

	if resp == nil {
		return nil, errors.New("received nil response from MS Graph for the message")
	}

	if resp.GetLastModifiedDateTime() == nil {
		return nil, errors.New("received nil last modified date time from MS Graph for the message")
	}

	return &clientmodels.Message{LastUpdateAt: *resp.GetLastModifiedDateTime()}, nil
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

// Function to match already existing group chats
func checkGroupChat(c models.Chatable, userIDs []string) *clientmodels.Chat {
	if c.GetId() == nil {
		return nil
	}

	if c.GetMembers() != nil && len(c.GetMembers()) == len(userIDs) {
		matches := map[string]bool{}
		members := []clientmodels.ChatMember{}
		for _, m := range c.GetMembers() {
			for _, u := range userIDs {
				userID, userErr := m.GetBackingStore().Get("userId")
				if userErr == nil && userID != nil && userID.(*string) != nil && *(userID.(*string)) == u {
					matches[u] = true
					userEmail, emailErr := m.GetBackingStore().Get("email")
					if emailErr == nil && userEmail != nil && userEmail.(*string) != nil {
						members = append(members, clientmodels.ChatMember{
							Email:  *(userEmail.(*string)),
							UserID: *(userID.(*string)),
						})
					}

					break
				}
			}
		}

		if len(matches) == len(userIDs) {
			return &clientmodels.Chat{
				ID:      *c.GetId(),
				Members: members,
				Type:    "G",
			}
		}
	}

	return nil
}
