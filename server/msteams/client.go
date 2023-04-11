//go:generate mockery --name=Client
package msteams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	azidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/enescakir/emoji"
	msgraphsdk "github.com/microsoftgraph/msgraph-beta-sdk-go"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/chats"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/models"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/teams"
	"github.com/microsoftgraph/msgraph-beta-sdk-go/users"
	az "github.com/microsoftgraph/msgraph-sdk-go-core/authentication"
	"gitlab.com/golang-commonmark/markdown"
)

type ClientImpl struct {
	client       *msgraphsdk.GraphServiceClient
	ctx          context.Context
	tenantID     string
	clientID     string
	clientSecret string
	clientType   string // can be "app" or "token"
	token        *Token
	logError     func(msg string, keyValuePairs ...any)
}

type Channel struct {
	ID          string
	DisplayName string
	Description string
}

type Chat struct {
	ID      string
	Members []ChatMember
	Type    string
}

type User struct {
	DisplayName string
	ID          string
}

type ChatMember struct {
	DisplayName string
	UserID      string
	Email       string
}

type Team struct {
	ID          string
	DisplayName string
	Description string
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

type Message struct {
	ID              string
	UserID          string
	UserDisplayName string
	Text            string
	Subject         string
	ReplyToID       string
	Attachments     []Attachment
	Reactions       []Reaction
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
	ChatID    string
	TeamID    string
	ChannelID string
	MessageID string
	ReplyID   string
}

type Token struct {
	Token     string
	ExpiresOn time.Time
}

func (cc *Token) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{
		Token:     cc.Token,
		ExpiresOn: cc.ExpiresOn,
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

func NewTokenClient(tenantID, clientID string, token *Token, logError func(string, ...any)) Client {
	client := &ClientImpl{
		ctx:        context.Background(),
		clientType: "token",
		token:      token,
		logError:   logError,
	}

	auth, err := az.NewAzureIdentityAuthenticationProviderWithScopes(client.token, teamsDefaultScopes)
	if err != nil {
		return nil
	}

	adapter, err := msgraphsdk.NewGraphRequestAdapter(auth)
	if err != nil {
		return nil
	}

	graphClient := msgraphsdk.NewGraphServiceClient(adapter)
	client.client = graphClient

	return client
}

func NewUnauthenticatedClient(tenantID, clientID string, logError func(string, ...any)) Client {
	return &ClientImpl{
		ctx:        context.Background(),
		clientType: "unauthenticated",
		tenantID:   tenantID,
		clientID:   clientID,
		logError:   logError,
	}
}

func (tc *ClientImpl) RequestUserToken(message chan string) (*Token, error) {
	cred, err := azidentity.NewDeviceCodeCredential(&azidentity.DeviceCodeCredentialOptions{
		TenantID: tc.tenantID,
		ClientID: tc.clientID,
		UserPrompt: func(ctx context.Context, dc azidentity.DeviceCodeMessage) error {
			message <- dc.Message
			return nil
		},
		ClientOptions: azcore.ClientOptions{
			Retry: policy.RetryOptions{
				MaxRetries:    3,
				RetryDelay:    4 * time.Second,
				MaxRetryDelay: 120 * time.Second,
			},
		},
	})

	_, err = msgraphsdk.NewGraphServiceClientWithCredentials(cred, teamsDefaultScopes)
	if err != nil {
		return nil, err
	}

	token, err := cred.GetToken(tc.ctx, policy.TokenRequestOptions{Scopes: teamsDefaultScopes})
	if err != nil {
		return nil, err
	}

	return &Token{Token: token.Token, ExpiresOn: token.ExpiresOn}, nil
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
		return "", err
	}
	return *r.GetId(), nil
}

func (tc *ClientImpl) SendMessage(teamID, channelID, parentID, message string) (*Message, error) {
	return tc.SendMessageWithAttachments(teamID, channelID, parentID, message, nil)
}

func (tc *ClientImpl) SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*Attachment) (*Message, error) {
	rmsg := models.NewChatMessage()
	md := markdown.New(markdown.XHTMLOutput(true))
	content := md.RenderToString([]byte(emoji.Parse(message)))

	msteamsAttachments := []models.ChatMessageAttachmentable{}
	for _, a := range attachments {
		att := a
		contentType := "reference"
		attachment := models.NewChatMessageAttachment()
		attachment.SetId(&att.ID)
		attachment.SetContentType(&contentType)
		attachment.SetContentUrl(&att.ContentURL)
		attachment.SetName(&att.Name)
		msteamsAttachments = append(msteamsAttachments, attachment)
		content = "<attachment id=\"" + att.ID + "\"></attachment>" + content
	}
	rmsg.SetAttachments(msteamsAttachments)

	contentType := models.HTML_BODYTYPE

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&content)
	rmsg.SetBody(body)

	var res models.ChatMessageable
	if parentID != "" {
		var err error
		res, err = tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).Replies().Post(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		res, err = tc.client.TeamsById(teamID).ChannelsById(channelID).Messages().Post(tc.ctx, rmsg, nil)
		if err != nil {
			return nil, err
		}
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) SendChat(chatID, parentID, message string) (*Message, error) {
	rmsg := models.NewChatMessage()
	md := markdown.New(markdown.XHTMLOutput(true))
	content := md.RenderToString([]byte(emoji.Parse(message)))

	contentType := models.HTML_BODYTYPE

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&content)
	rmsg.SetBody(body)

	res, err := tc.client.ChatsById(chatID).Messages().Post(tc.ctx, rmsg, nil)
	if err != nil {
		return nil, err
	}

	return convertToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) UploadFile(teamID, channelID, filename string, filesize int, mimeType string, data io.Reader) (*Attachment, error) {
	folderInfo, err := tc.client.TeamsById(teamID).ChannelsById(channelID).FilesFolder().Get(tc.ctx, nil)
	if err != nil {
		return nil, err
	}

	uploadSession, err := tc.client.DrivesById(*folderInfo.GetParentReference().GetDriveId()).ItemsById(*folderInfo.GetId()+":/"+filename+":").CreateUploadSession().Post(tc.ctx, nil, nil)
	if err != nil {
		return nil, err
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
			return err
		}
	} else {
		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(msgID).Delete(tc.ctx, nil); err != nil {
			return err
		}
	}
	return nil
}

func (tc *ClientImpl) DeleteChatMessage(chatID, msgID string) error {
	if err := tc.client.ChatsById(chatID).MessagesById(msgID).Delete(tc.ctx, nil); err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) UpdateMessage(teamID, channelID, parentID, msgID, message string) error {
	rmsg := models.NewChatMessage()
	md := markdown.New(markdown.XHTMLOutput(true), markdown.LangPrefix("CodeMirror language-"))
	content := md.RenderToString([]byte(emoji.Parse(message)))

	contentType := models.HTML_BODYTYPE

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&content)
	rmsg.SetBody(body)

	if parentID != "" {
		if _, err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(msgID).Patch(tc.ctx, rmsg, nil); err != nil {
			return err
		}
	} else {
		if _, err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(msgID).Patch(tc.ctx, rmsg, nil); err != nil {
			return err
		}
	}
	return nil
}

func (tc *ClientImpl) UpdateChatMessage(chatID, msgID, message string) error {
	rmsg := models.NewChatMessage()
	md := markdown.New(markdown.XHTMLOutput(true), markdown.LangPrefix("CodeMirror language-"))
	content := md.RenderToString([]byte(emoji.Parse(message)))

	contentType := models.HTML_BODYTYPE

	body := models.NewItemBody()
	body.SetContentType(&contentType)
	body.SetContent(&content)
	rmsg.SetBody(body)

	if _, err := tc.client.ChatsById(chatID).MessagesById(msgID).Patch(tc.ctx, rmsg, nil); err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) subscribe(baseURL, webhookSecret, resource, changeType string) (string, error) {
	subscriptionsRes, err := tc.client.Subscriptions().Get(tc.ctx, nil)
	if err != nil {
		tc.logError("Unable to get the subcscriptions list", err)
		return "", err
	}

	var existingSubscription models.Subscriptionable
	for _, s := range subscriptionsRes.GetValue() {
		subscription := s
		if subscription.GetResource() != nil && (*subscription.GetResource() == resource || *subscription.GetResource()+"?model=B" == resource) {
			existingSubscription = subscription
			break
		}
	}

	lifecycleNotificationURL := baseURL + "lifecycle"
	notificationURL := baseURL + "changes"

	expirationDateTime := time.Now().Add(30 * time.Minute)
	subscription := models.NewSubscription()
	subscription.SetResource(&resource)
	subscription.SetExpirationDateTime(&expirationDateTime)
	subscription.SetNotificationUrl(&notificationURL)
	subscription.SetLifecycleNotificationUrl(&lifecycleNotificationURL)
	subscription.SetClientState(&webhookSecret)
	subscription.SetChangeType(&changeType)

	if existingSubscription != nil {
		if *existingSubscription.GetChangeType() != changeType || *existingSubscription.GetLifecycleNotificationUrl() != lifecycleNotificationURL || *existingSubscription.GetNotificationUrl() != notificationURL || *existingSubscription.GetClientState() != webhookSecret {
			if err := tc.client.SubscriptionsById(*existingSubscription.GetId()).Delete(tc.ctx, nil); err != nil {
				tc.logError("Unable to delete the subscription", "error", err, "subscription", existingSubscription)
			}
		} else {
			expirationDateTime := time.Now().Add(30 * time.Minute)
			updatedSubscription := models.NewSubscription()
			updatedSubscription.SetExpirationDateTime(&expirationDateTime)
			if _, err := tc.client.SubscriptionsById(*existingSubscription.GetId()).Patch(tc.ctx, updatedSubscription, nil); err != nil {
				return *existingSubscription.GetId(), nil
			}

			tc.logError("Unable to refresh the subscription", "error", err, "subscription", existingSubscription)
			if err := tc.client.SubscriptionsById(*existingSubscription.GetId()).Delete(tc.ctx, nil); err != nil {
				tc.logError("Unable to delete the subscription", "error", err, "subscription", existingSubscription)
			}
		}
	}

	res, err := tc.client.Subscriptions().Post(tc.ctx, subscription, nil)
	if err != nil {
		tc.logError("Unable to create the new subscription", "error", err)
		return "", err
	}

	return *res.GetId(), nil
}

func (tc *ClientImpl) SubscribeToChannels(baseURL string, webhookSecret string, pay bool) (string, error) {
	resource := "teams/getAllMessages"
	if pay {
		resource = "teams/getAllMessages?model=B"
	}
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) SubscribeToChats(baseURL string, webhookSecret string, pay bool) (string, error) {
	resource := "chats/getAllMessages"
	if pay {
		resource = "chats/getAllMessages?model=B"
	}
	changeType := "created,deleted,updated"
	return tc.subscribe(baseURL, webhookSecret, resource, changeType)
}

func (tc *ClientImpl) RefreshSubscription(subscriptionID string) error {
	expirationDateTime := time.Now().Add(30 * time.Minute)
	updatedSubscription := models.NewSubscription()
	updatedSubscription.SetExpirationDateTime(&expirationDateTime)
	if _, err := tc.client.SubscriptionsById(subscriptionID).Patch(tc.ctx, updatedSubscription, nil); err != nil {
		tc.logError("Unable to refresh the subscription", "error", err, "subscriptionID", subscriptionID)
		return err
	}
	return nil
}

func (tc *ClientImpl) GetTeam(teamID string) (*Team, error) {
	res, err := tc.client.TeamsById(teamID).Get(tc.ctx, nil)
	if err != nil {
		return nil, err
	}

	displayName := ""
	if res.GetDisplayName() != nil {
		displayName = *res.GetDisplayName()
	}

	return &Team{ID: teamID, DisplayName: displayName}, nil
}

func (tc *ClientImpl) GetChannel(teamID, channelID string) (*Channel, error) {
	res, err := tc.client.TeamsById(teamID).ChannelsById(channelID).Get(tc.ctx, nil)
	if err != nil {
		return nil, err
	}

	displayName := ""
	if res.GetDisplayName() != nil {
		displayName = *res.GetDisplayName()
	}

	return &Channel{ID: channelID, DisplayName: displayName}, nil
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
		return nil, err
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
		userID, ok := member.GetAdditionalData()["userId"]
		if !ok {
			userID = ""
		}
		email, ok := member.GetAdditionalData()["email"]
		if !ok {
			email = ""
		}

		members = append(members, ChatMember{
			DisplayName: displayName,
			UserID:      userID.(string),
			Email:       email.(string),
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
		return nil, err
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetChatMessage(chatID, messageID string) (*Message, error) {
	res, err := tc.client.ChatsById(chatID).MessagesById(messageID).Get(tc.ctx, nil)
	if err != nil {
		return nil, err
	}
	return convertToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) GetReply(teamID, channelID, messageID, replyID string) (*Message, error) {
	res, err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(messageID).RepliesById(replyID).Get(tc.ctx, nil)
	if err != nil {
		return nil, err
	}

	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetUserAvatar(userID string) ([]byte, error) {
	photo, err := tc.client.UsersById(userID).Photo().Content().Get(tc.ctx, nil)
	if err != nil {
		return nil, err
	}

	return photo, nil
}

func (tc *ClientImpl) GetFileURL(weburl string) (string, error) {
	// TODO: I have to investigate how to do this
	// itemRB, err := tc.client.GetDriveItemByURL(tc.ctx, weburl)
	// if err != nil {
	// 	return "", err
	// }
	// itemRB.Workbook().Worksheets()
	// tc.client.Workbooks()
	// item, err := itemRB.Request().Get(tc.ctx)
	// if err != nil {
	// 	return "", err
	// }
	// url, ok := item.GetAdditionalData("@microsoft.graph.downloadUrl")
	// if !ok {
	// 	return "", nil
	// }
	// return url.(string), nil
	return "", errors.New("NOT IMPLEMENTED YET")
}

func (tc *ClientImpl) GetCodeSnippet(url string) (string, error) {
	// TODO: I have to investigate how to do this
	// resp, err := tc.client.Teams().Request().Client().Get(url)
	// if err != nil {
	// 	return "", err
	// }
	// defer resp.Body.Close()
	// res, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	return "", err
	// }
	// return string(res), nil
	return "", errors.New("NOT IMPLEMENTED YET")
}

func GetResourceIds(resource string) ActivityIds {
	result := ActivityIds{}
	data := strings.Split(resource, "/")

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
	requestParameters := &chats.ChatsRequestBuilderGetQueryParameters{
		Select: []string{"members", "id"},
		Expand: []string{"members"},
	}
	configuration := &chats.ChatsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	res, err := tc.client.Chats().Get(tc.ctx, configuration)
	if err != nil {
		return "", err
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
					if m.GetAdditionalData()["userId"] == u {
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
		return "", err
	}
	return *resn.GetId(), nil
}

func (tc *ClientImpl) SetChatReaction(chatID, messageID, userID, emoji string) error {
	setReaction := chats.NewItemMessagesItemSetReactionPostRequestBody()
	setReaction.SetReactionType(&emoji)

	if err := tc.client.ChatsById(chatID).MessagesById(messageID).SetReaction().Post(tc.ctx, setReaction, nil); err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) SetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error {
	if parentID == "" {
		setReaction := teams.NewItemChannelsItemMessagesItemSetReactionPostRequestBody()
		setReaction.SetReactionType(&emoji)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(messageID).SetReaction().Post(tc.ctx, setReaction, nil); err != nil {
			return err
		}
	} else {
		setReaction := teams.NewItemChannelsItemMessagesItemRepliesItemSetReactionPostRequestBody()
		setReaction.SetReactionType(&emoji)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(messageID).SetReaction().Post(tc.ctx, setReaction, nil); err != nil {
			return err
		}
	}
	return nil
}

func (tc *ClientImpl) UnsetChatReaction(chatID, messageID, userID, emoji string) error {
	unsetReaction := chats.NewItemMessagesItemUnsetReactionPostRequestBody()
	unsetReaction.SetReactionType(&emoji)

	if err := tc.client.ChatsById(chatID).MessagesById(messageID).UnsetReaction().Post(tc.ctx, unsetReaction, nil); err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error {
	if parentID == "" {
		unsetReaction := teams.NewItemChannelsItemMessagesItemUnsetReactionPostRequestBody()
		unsetReaction.SetReactionType(&emoji)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(messageID).UnsetReaction().Post(tc.ctx, unsetReaction, nil); err != nil {
			return err
		}
	} else {
		unsetReaction := teams.NewItemChannelsItemMessagesItemRepliesItemUnsetReactionPostRequestBody()
		unsetReaction.SetReactionType(&emoji)

		if err := tc.client.TeamsById(teamID).ChannelsById(channelID).MessagesById(parentID).RepliesById(messageID).UnsetReaction().Post(tc.ctx, unsetReaction, nil); err != nil {
			return err
		}
	}
	return nil
}

func (tc *ClientImpl) ListUsers() ([]User, error) {
	requestParameters := &users.UsersRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id"},
	}
	configuration := &users.UsersRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.Users().Get(tc.ctx, configuration)
	if err != nil {
		return nil, err
	}

	users := make([]User, len(r.GetValue()))
	for i, u := range r.GetValue() {
		displayName := ""
		if u.GetDisplayName() != nil {
			displayName = *u.GetDisplayName()
		}

		users[i] = User{
			DisplayName: displayName,
			ID:          *u.GetId(),
		}
	}
	return users, nil
}

func (tc *ClientImpl) ListTeams() ([]Team, error) {
	requestParameters := &users.ItemJoinedTeamsRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id", "description"},
	}
	configuration := &users.ItemJoinedTeamsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.Me().JoinedTeams().Get(tc.ctx, configuration)
	if err != nil {
		return nil, err
	}
	teams := make([]Team, len(r.GetValue()))

	for i, t := range r.GetValue() {
		description := ""
		if t.GetDescription() != nil {
			description = *t.GetDescription()
		}

		displayName := ""
		if t.GetDisplayName() != nil {
			displayName = *t.GetDisplayName()
		}

		teams[i] = Team{
			DisplayName: displayName,
			Description: description,
			ID:          *t.GetId(),
		}
	}
	return teams, nil
}

func (tc *ClientImpl) ListChannels(teamID string) ([]Channel, error) {
	requestParameters := &teams.ItemChannelsRequestBuilderGetQueryParameters{
		Select: []string{"displayName", "id", "description"},
	}
	configuration := &teams.ItemChannelsRequestBuilderGetRequestConfiguration{
		QueryParameters: requestParameters,
	}
	r, err := tc.client.TeamsById(teamID).Channels().Get(tc.ctx, configuration)
	if err != nil {
		return nil, err
	}
	channels := make([]Channel, len(r.GetValue()))
	for i, c := range r.GetValue() {
		description := ""
		if c.GetDescription() != nil {
			description = *c.GetDescription()
		}

		displayName := ""
		if c.GetDisplayName() != nil {
			displayName = *c.GetDisplayName()
		}

		channels[i] = Channel{
			DisplayName: displayName,
			Description: description,
			ID:          *c.GetId(),
		}
	}
	return channels, nil
}
