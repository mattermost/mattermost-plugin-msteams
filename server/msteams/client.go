//go:generate mockery --name=Client
package msteams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	msgraph "github.com/yaegashi/msgraph.go/beta"
	"github.com/yaegashi/msgraph.go/msauth"
	"gitlab.com/golang-commonmark/markdown"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

type ClientImpl struct {
	client       *msgraph.GraphServiceRequestBuilder
	ctx          context.Context
	botID        string
	tenantId     string
	clientId     string
	clientSecret string
	botUsername  string
	botPassword  string
	clientType   string // can be "bot", "app" or "token"
	token        *oauth2.Token
}

type Channel struct {
	ID          string
	DisplayName string
}

type Chat struct {
	ID      string
	Members []ChatMember
	Type    string
}

type ChatMember struct {
	DisplayName string
	UserID      string
	Email       string
}

type Team struct {
	ID          string
	DisplayName string
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

type Message struct {
	ID              string
	UserID          string
	UserDisplayName string
	Text            string
	Subject         string
	ReplyToID       string
	Attachments     []Attachment
	ChannelID       string
	TeamID          string
	ChatID          string
}

type Activity struct {
	Resource       string
	SubscriptionId string
	ClientState    string
	ChangeType     string
}

type ActivityIds struct {
	ChatID    string
	TeamID    string
	ChannelID string
	MessageID string
	ReplyID   string
}

var teamsDefaultScopes = []string{"https://graph.microsoft.com/.default"}

func NewApp(tenantId, clientId, clientSecret string) *ClientImpl {
	return &ClientImpl{
		ctx:          context.Background(),
		clientType:   "app",
		tenantId:     tenantId,
		clientId:     clientId,
		clientSecret: clientSecret,
	}
}

func NewTokenClient(tenantId, clientId string, token *oauth2.Token) *ClientImpl {
	client := &ClientImpl{
		ctx:        context.Background(),
		clientType: "token",
		token:      token,
	}
	endpoint := microsoft.AzureADEndpoint(tenantId)
	endpoint.AuthStyle = oauth2.AuthStyleInParams
	config := &oauth2.Config{
		ClientID: clientId,
		Endpoint: endpoint,
		Scopes:   teamsDefaultScopes,
	}

	ts := config.TokenSource(client.ctx, client.token)
	httpClient := oauth2.NewClient(client.ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	client.client = graphClient

	return client
}

func NewBot(tenantId, clientId, clientSecret, botUsername, botPassword string) *ClientImpl {
	return &ClientImpl{
		ctx:          context.Background(),
		clientType:   "bot",
		tenantId:     tenantId,
		clientId:     clientId,
		clientSecret: clientSecret,
		botUsername:  botUsername,
		botPassword:  botPassword,
	}
}

func RequestUserToken(tenantId, clientId string, message chan string) (oauth2.TokenSource, error) {
	m := msauth.NewManager()
	ts, err := m.DeviceAuthorizationGrant(
		context.Background(),
		tenantId,
		clientId,
		append(teamsDefaultScopes, "offline_access"),
		func(dc *msauth.DeviceCode) error {
			message <- dc.Message
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	t, err := ts.Token()
	if err != nil {
		return nil, err
	}
	fmt.Println("TOKEN INFO", t)
	return ts, nil
}

func (tc *ClientImpl) Connect() error {
	var ts oauth2.TokenSource
	if tc.clientType == "token" {
		return nil
	} else if tc.clientType == "bot" {
		var err error
		m := msauth.NewManager()
		ts, err = m.ResourceOwnerPasswordGrant(
			tc.ctx,
			tc.tenantId,
			tc.clientId,
			tc.clientSecret,
			tc.botUsername,
			tc.botPassword,
			teamsDefaultScopes,
		)
		if err != nil {
			return err
		}
	} else if tc.clientType == "app" {
		var err error
		m := msauth.NewManager()
		ts, err = m.ClientCredentialsGrant(
			tc.ctx,
			tc.tenantId,
			tc.clientId,
			tc.clientSecret,
			teamsDefaultScopes,
		)
		if err != nil {
			return err
		}
	} else {
		return errors.New("not valid client type, this shouldn't happen ever.")
	}

	httpClient := oauth2.NewClient(tc.ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	tc.client = graphClient

	if tc.clientType == "bot" {
		req := graphClient.Me().Request()
		r, err := req.Get(tc.ctx)
		if err != nil {
			return err
		}
		tc.botID = *r.ID
	}

	return nil
}

func (tc *ClientImpl) GetMyID() (string, error) {
	req := tc.client.Me().Request()
	r, err := req.Get(tc.ctx)
	if err != nil {
		return "", err
	}
	return *r.ID, nil
}

func (tc *ClientImpl) SendMessage(teamID, channelID, parentID, message string) (string, error) {
	return tc.SendMessageWithAttachments(teamID, channelID, parentID, message, nil)
}

func (tc *ClientImpl) SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*Attachment) (string, error) {
	rmsg := &msgraph.ChatMessage{}
	md := markdown.New(markdown.XHTMLOutput(true))
	content := md.RenderToString([]byte(message))

	for _, attachment := range attachments {
		att := attachment
		contentType := "reference"
		rmsg.Attachments = append(rmsg.Attachments,
			msgraph.ChatMessageAttachment{
				ID:          &att.ID,
				ContentType: &contentType,
				ContentURL:  &att.ContentURL,
				Name:        &att.Name,
			},
		)
		content = "<attachment id=\"" + att.ID + "\"></attachment>" + content
	}

	contentType := msgraph.BodyTypeVHTML
	rmsg.Body = &msgraph.ItemBody{ContentType: &contentType, Content: &content}

	var res *msgraph.ChatMessage
	if len(parentID) > 0 {
		var err error
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(parentID).Replies().Request()
		res, err = ct.Add(tc.ctx, rmsg)
		if err != nil {
			return "", err
		}
	} else {
		var err error
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().Request()
		res, err = ct.Add(tc.ctx, rmsg)
		if err != nil {
			return "", err
		}
	}
	return *res.ID, nil
}

func (tc *ClientImpl) SendChat(chatID, parentID, message string) (string, error) {
	rmsg := &msgraph.ChatMessage{}
	md := markdown.New(markdown.XHTMLOutput(true))
	content := md.RenderToString([]byte(message))

	contentType := msgraph.BodyTypeVHTML
	rmsg.Body = &msgraph.ItemBody{ContentType: &contentType, Content: &content}

	var res *msgraph.ChatMessage
	ct := tc.client.Chats().ID(chatID).Messages().Request()
	res, err := ct.Add(tc.ctx, rmsg)
	if err != nil {
		return "", err
	}
	return *res.ID, nil
}

func (tc *ClientImpl) UploadFile(teamID, channelID, filename string, filesize int, mimeType string, data io.Reader) (*Attachment, error) {
	fct := tc.client.Teams().ID(teamID).Channels().ID(channelID).FilesFolder().Request()
	folderInfo, err := fct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	ct := tc.client.Drives().ID(*folderInfo.ParentReference.DriveID).Items().ID(*folderInfo.ID + ":/" + filename + ":").CreateUploadSession(
		&msgraph.DriveItemCreateUploadSessionRequestParameter{},
	).Request()

	uploadSession, err := ct.Post(tc.ctx)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", *uploadSession.UploadURL, data)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Length", fmt.Sprintf("%d", filesize))
	req.Header.Add("Content-Range", fmt.Sprintf("bytes 0-%d/%d", filesize-1, filesize))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	uploadedFileData, err := ioutil.ReadAll(res.Body)
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
	if len(parentID) > 0 {
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(parentID).Replies().ID(msgID).Request()
		if err := ct.Delete(tc.ctx); err != nil {
			return err
		}
	} else {
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(msgID).Request()
		if err := ct.Delete(tc.ctx); err != nil {
			return err
		}
	}
	return nil
}

func (tc *ClientImpl) DeleteChatMessage(chatID, msgID string) error {
	ct := tc.client.Chats().ID(chatID).Messages().ID(msgID).Request()
	if err := ct.Delete(tc.ctx); err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) UpdateMessage(teamID, channelID, parentID, msgID, message string) error {
	md := markdown.New(markdown.XHTMLOutput(true), markdown.LangPrefix("CodeMirror language-"))
	content := md.RenderToString([]byte(message))
	contentType := msgraph.BodyTypeVHTML
	body := &msgraph.ItemBody{ContentType: &contentType, Content: &content}
	rmsg := &msgraph.ChatMessage{Body: body}

	if len(parentID) > 0 {
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(parentID).Replies().ID(msgID).Request()
		if err := ct.Update(tc.ctx, rmsg); err != nil {
			return err
		}
	} else {
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(msgID).Request()
		if err := ct.Update(tc.ctx, rmsg); err != nil {
			return err
		}
	}
	return nil
}

func (tc *ClientImpl) UpdateChatMessage(chatID, msgID, message string) error {
	md := markdown.New(markdown.XHTMLOutput(true), markdown.LangPrefix("CodeMirror language-"))
	content := md.RenderToString([]byte(message))
	contentType := msgraph.BodyTypeVHTML
	body := &msgraph.ItemBody{ContentType: &contentType, Content: &content}
	rmsg := &msgraph.ChatMessage{Body: body}

	ct := tc.client.Chats().ID(chatID).Messages().ID(msgID).Request()
	if err := ct.Update(tc.ctx, rmsg); err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) Subscribe(notificationURL string, webhookSecret string) (string, error) {
	resource := "teams/getAllMessages"
	expirationDateTime := time.Now().Add(60 * time.Minute)
	changeType := "created,deleted,updated"
	subscription := msgraph.Subscription{
		Resource:           &resource,
		ExpirationDateTime: &expirationDateTime,
		NotificationURL:    &notificationURL,
		ClientState:        &webhookSecret,
		ChangeType:         &changeType,
	}
	ct := tc.client.Subscriptions().Request()
	res, err := ct.Add(tc.ctx, &subscription)
	if err != nil {
		return "", err
	}
	return *res.ID, nil
}

func (tc *ClientImpl) SubscribeToChats(notificationURL string, webhookSecret string) (string, error) {
	resource := "chats/getAllMessages"
	expirationDateTime := time.Now().Add(60 * time.Minute)
	changeType := "created,deleted,updated"
	subscription := msgraph.Subscription{
		Resource:           &resource,
		ExpirationDateTime: &expirationDateTime,
		NotificationURL:    &notificationURL,
		ClientState:        &webhookSecret,
		ChangeType:         &changeType,
	}
	ct := tc.client.Subscriptions().Request()
	res, err := ct.Add(tc.ctx, &subscription)
	if err != nil {
		return "", err
	}
	return *res.ID, nil
}

func (tc *ClientImpl) SubscribeToChannel(teamID, channelID, notificationURL string, webhookSecret string) (string, error) {
	resource := "teams/" + teamID + "/channels/" + channelID + "/messages"
	expirationDateTime := time.Now().Add(60 * time.Minute)
	changeType := "created,deleted,updated"
	subscription := msgraph.Subscription{
		Resource:           &resource,
		ExpirationDateTime: &expirationDateTime,
		NotificationURL:    &notificationURL,
		ClientState:        &webhookSecret,
		ChangeType:         &changeType,
	}
	ct := tc.client.Subscriptions().Request()
	res, err := ct.Add(tc.ctx, &subscription)
	if err != nil {
		return "", err
	}
	return *res.ID, nil
}

func (tc *ClientImpl) RefreshSubscriptionPeriodically(ctx context.Context, subscriptionID string) error {
	for {
		select {
		case <-time.After(time.Minute):
			expirationDateTime := time.Now().Add(10 * time.Minute)
			updatedSubscription := msgraph.Subscription{
				ExpirationDateTime: &expirationDateTime,
			}
			ct := tc.client.Subscriptions().ID(subscriptionID).Request()
			err := ct.Update(tc.ctx, &updatedSubscription)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			deleteSubCt := tc.client.Subscriptions().ID(subscriptionID).Request()
			err := deleteSubCt.Delete(tc.ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (tc *ClientImpl) ClearSubscription(subscriptionID string) error {
	deleteSubCt := tc.client.Subscriptions().ID(subscriptionID).Request()
	err := deleteSubCt.Delete(tc.ctx)
	if err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) ClearSubscriptions() error {
	subscriptionsCt := tc.client.Subscriptions().Request()
	subscriptionsRes, err := subscriptionsCt.Get(tc.ctx)
	if err != nil {
		return err
	}
	for _, subscription := range subscriptionsRes {
		deleteSubCt := tc.client.Subscriptions().ID(*subscription.ID).Request()
		err := deleteSubCt.Delete(tc.ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (tc *ClientImpl) GetTeam(teamID string) (*Team, error) {
	ct := tc.client.Teams().ID(teamID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	displayName := ""
	if res.DisplayName != nil {
		displayName = *res.DisplayName
	}

	return &Team{ID: teamID, DisplayName: displayName}, nil
}

func (tc *ClientImpl) GetChannel(teamID, channelID string) (*Channel, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	displayName := ""
	if res.DisplayName != nil {
		displayName = *res.DisplayName
	}

	return &Channel{ID: channelID, DisplayName: displayName}, nil
}

func (tc *ClientImpl) GetChat(chatID string) (*Chat, error) {
	ct := tc.client.Chats().ID(chatID).Request()
	ct.Expand("members")
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	chatType := ""
	if res.AdditionalData["chatType"] == "group" {
		chatType = "G"
	} else if res.AdditionalData["chatType"] == "oneOnOne" {
		chatType = "D"
	}

	members := []ChatMember{}
	for _, member := range res.Members {
		displayName := ""
		if member.DisplayName != nil {
			displayName = *member.DisplayName
		}
		userId, ok := member.AdditionalData["userId"]
		if !ok {
			userId = ""
		}
		email, ok := member.AdditionalData["email"]
		if !ok {
			email = ""
		}

		members = append(members, ChatMember{
			DisplayName: displayName,
			UserID:      userId.(string),
			Email:       email.(string),
		})
	}

	return &Chat{ID: chatID, Members: members, Type: chatType}, nil
}

func converToMessage(msg *msgraph.ChatMessage, teamID, channelID, chatID string) *Message {
	userID := ""
	if msg.From != nil && msg.From.User != nil && msg.From.User.ID != nil {
		userID = *msg.From.User.ID
	}
	userDisplayName := ""
	if msg.From != nil && msg.From.User != nil && msg.From.User.DisplayName != nil {
		userDisplayName = *msg.From.User.DisplayName
	}

	replyTo := ""
	if msg.ReplyToID != nil {
		replyTo = *msg.ReplyToID
	}

	text := ""
	if msg.Body != nil && msg.Body.Content != nil {
		text = *msg.Body.Content
	}

	msgID := ""
	if msg.ID != nil {
		msgID = *msg.ID
	}

	subject := ""
	if msg.Subject != nil {
		subject = *msg.Subject
	}

	attachments := []Attachment{}
	for _, attachment := range msg.Attachments {
		contentType := ""
		if attachment.ContentType != nil {
			contentType = *attachment.ContentType
		}
		content := ""
		if attachment.Content != nil {
			content = *attachment.Content
		}
		name := ""
		if attachment.Name != nil {
			name = *attachment.Name
		}
		contentURL := ""
		if attachment.ContentURL != nil {
			contentURL = *attachment.ContentURL
		}
		attachments = append(attachments, Attachment{
			ContentType: contentType,
			Content:     content,
			Name:        name,
			ContentURL:  contentURL,
		})
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
	}

}

func (tc *ClientImpl) GetMessage(teamID, channelID, messageID string) (*Message, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	return converToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetChatMessage(chatID, messageID string) (*Message, error) {
	ct := tc.client.Chats().ID(chatID).Messages().ID(messageID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	return converToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) GetReply(teamID, channelID, messageID, replyID string) (*Message, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Replies().ID(replyID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	return converToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetUserAvatar(userID string) ([]byte, error) {
	ctb := tc.client.Users().ID(userID).Photo()
	ctb.SetURL(ctb.URL() + "/$value")
	ct := ctb.Request()
	req, err := ct.NewRequest("GET", "", nil)
	if err != nil {
		return nil, err
	}
	res, err := ct.Client().Do(req)
	if err != nil {
		return nil, err
	}
	photo, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return photo, nil
}

func (tc *ClientImpl) GetFileURL(weburl string) (string, error) {
	itemRB, err := tc.client.GetDriveItemByURL(tc.ctx, weburl)
	if err != nil {
		return "", err
	}
	itemRB.Workbook().Worksheets()
	tc.client.Workbooks()
	item, err := itemRB.Request().Get(tc.ctx)
	if err != nil {
		return "", err
	}
	url, ok := item.GetAdditionalData("@microsoft.graph.downloadUrl")
	if !ok {
		return "", nil
	}
	return url.(string), nil
}

func (tc *ClientImpl) GetCodeSnippet(url string) (string, error) {
	resp, err := tc.client.Teams().Request().Client().Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	res, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func GetActivityIds(activity Activity) ActivityIds {
	result := ActivityIds{}
	data := strings.Split(activity.Resource, "/")

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

func (tc *ClientImpl) BotID() string {
	return tc.botID
}

func (tc *ClientImpl) CreateOrGetChatForUsers(dstUserID, srcUserID string) (string, error) {
	ct := tc.client.Chats().Request()
	// TODO: add the filter to make this more performant)
	// ct.Filter()
	ct.Expand("members")
	res, err := ct.Get(tc.ctx)
	for _, c := range res {
		if len(c.Members) == 2 {
			if c.Members[0].AdditionalData["userId"] == srcUserID || c.Members[1].AdditionalData["userId"] == dstUserID {
				return *c.ID, nil
			}
			if c.Members[1].AdditionalData["userId"] == srcUserID || c.Members[0].AdditionalData["userId"] == dstUserID {
				return *c.ID, nil
			}
		}
	}

	ctn := tc.client.Chats().Request()
	resn, err := ctn.Add(tc.ctx, &msgraph.Chat{
		Entity: msgraph.Entity{
			Object: msgraph.Object{
				AdditionalData: map[string]interface{}{"chatType": "oneOnOne"},
			},
		},
		Members: []msgraph.ConversationMember{
			{
				Entity: msgraph.Entity{
					Object: msgraph.Object{
						AdditionalData: map[string]interface{}{
							"@odata.type":     "#microsoft.graph.aadUserConversationMember",
							"user@odata.bind": "https://graph.microsoft.com/v1.0/users('" + dstUserID + "')",
						},
					},
				},
				Roles: []string{"owner"},
			},
			{
				Entity: msgraph.Entity{
					Object: msgraph.Object{
						AdditionalData: map[string]interface{}{
							"@odata.type":     "#microsoft.graph.aadUserConversationMember",
							"user@odata.bind": "https://graph.microsoft.com/v1.0/users('" + srcUserID + "')",
						},
					},
				},
				Roles: []string{"owner"},
			},
		},
	})
	if err != nil {
		return "", err
	}
	return *resn.ID, nil
}
