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

	"github.com/enescakir/emoji"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"github.com/yaegashi/msgraph.go/msauth"
	"gitlab.com/golang-commonmark/markdown"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

type ClientImpl struct {
	client       *msgraph.GraphServiceRequestBuilder
	ctx          context.Context
	tenantID     string
	clientID     string
	clientSecret string
	clientType   string // can be "app" or "token"
	token        *oauth2.Token
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

func NewTokenClient(tenantID, clientID string, token *oauth2.Token, logError func(string, ...any)) Client {
	client := &ClientImpl{
		ctx:        context.Background(),
		clientType: "token",
		token:      token,
		logError:   logError,
	}
	endpoint := microsoft.AzureADEndpoint(tenantID)
	endpoint.AuthStyle = oauth2.AuthStyleInParams
	config := &oauth2.Config{
		ClientID: clientID,
		Endpoint: endpoint,
		Scopes:   teamsDefaultScopes,
	}

	ts := config.TokenSource(client.ctx, client.token)
	httpClient := oauth2.NewClient(client.ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
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

func (tc *ClientImpl) RequestUserToken(message chan string) (oauth2.TokenSource, error) {
	m := msauth.NewManager()
	ts, err := m.DeviceAuthorizationGrant(
		context.Background(),
		tc.tenantID,
		tc.clientID,
		append(teamsDefaultScopes, "offline_access"),
		func(dc *msauth.DeviceCode) error {
			message <- dc.Message
			return nil
		},
	)
	if err != nil {
		return nil, err
	}
	return ts, nil
}

func (tc *ClientImpl) Connect() error {
	var ts oauth2.TokenSource
	switch tc.clientType {
	case "token":
		return nil
	case "app":
		var err error
		m := msauth.NewManager()
		ts, err = m.ClientCredentialsGrant(
			tc.ctx,
			tc.tenantID,
			tc.clientID,
			tc.clientSecret,
			teamsDefaultScopes,
		)
		if err != nil {
			return err
		}
	default:
		return errors.New("not valid client type, this shouldn't happen ever")
	}

	httpClient := oauth2.NewClient(tc.ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	tc.client = graphClient

	return nil
}

func (tc *ClientImpl) GetMyID() (string, error) {
	req := tc.client.Me().Request()
	req.Select("id")
	r, err := req.Get(tc.ctx)
	if err != nil {
		return "", err
	}
	return *r.ID, nil
}

func (tc *ClientImpl) SendMessage(teamID, channelID, parentID, message string) (*Message, error) {
	return tc.SendMessageWithAttachments(teamID, channelID, parentID, message, nil)
}

func (tc *ClientImpl) SendMessageWithAttachments(teamID, channelID, parentID, message string, attachments []*Attachment) (*Message, error) {
	rmsg := &msgraph.ChatMessage{}
	md := markdown.New(markdown.XHTMLOutput(true))
	content := md.RenderToString([]byte(emoji.Parse(message)))

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
	if parentID != "" {
		var err error
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(parentID).Replies().Request()
		res, err = ct.Add(tc.ctx, rmsg)
		if err != nil {
			return nil, err
		}
	} else {
		var err error
		ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().Request()
		res, err = ct.Add(tc.ctx, rmsg)
		if err != nil {
			return nil, err
		}
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) SendChat(chatID, parentID, message string) (*Message, error) {
	rmsg := &msgraph.ChatMessage{}
	md := markdown.New(markdown.XHTMLOutput(true))
	content := md.RenderToString([]byte(emoji.Parse(message)))

	contentType := msgraph.BodyTypeVHTML
	rmsg.Body = &msgraph.ItemBody{ContentType: &contentType, Content: &content}

	var res *msgraph.ChatMessage
	ct := tc.client.Chats().ID(chatID).Messages().Request()
	res, err := ct.Add(tc.ctx, rmsg)
	if err != nil {
		return nil, err
	}

	return convertToMessage(res, "", "", chatID), nil
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
	content := md.RenderToString([]byte(emoji.Parse(message)))
	contentType := msgraph.BodyTypeVHTML
	body := &msgraph.ItemBody{ContentType: &contentType, Content: &content}
	rmsg := &msgraph.ChatMessage{Body: body}

	if parentID != "" {
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
	content := md.RenderToString([]byte(emoji.Parse(message)))
	contentType := msgraph.BodyTypeVHTML
	body := &msgraph.ItemBody{ContentType: &contentType, Content: &content}
	rmsg := &msgraph.ChatMessage{Body: body}

	ct := tc.client.Chats().ID(chatID).Messages().ID(msgID).Request()
	if err := ct.Update(tc.ctx, rmsg); err != nil {
		return err
	}
	return nil
}

func (tc *ClientImpl) subscribe(baseURL, webhookSecret, resource, changeType string) (string, error) {
	expirationDateTime := time.Now().Add(30 * time.Minute)

	subscriptionsCt := tc.client.Subscriptions().Request()
	subscriptionsRes, err := subscriptionsCt.Get(tc.ctx)
	if err != nil {
		tc.logError("Unable to get the subcscriptions list", err)
		return "", err
	}

	var existingSubscription *msgraph.Subscription
	for _, s := range subscriptionsRes {
		subscription := s
		if *subscription.Resource == resource || *subscription.Resource+"?model=B" == resource {
			existingSubscription = &subscription
			break
		}
	}

	lifecycleNotificationURL := baseURL + "lifecycle"
	notificationURL := baseURL + "changes"

	subscription := msgraph.Subscription{
		Resource:                 &resource,
		ExpirationDateTime:       &expirationDateTime,
		NotificationURL:          &notificationURL,
		LifecycleNotificationURL: &lifecycleNotificationURL,
		ClientState:              &webhookSecret,
		ChangeType:               &changeType,
	}

	if existingSubscription != nil {
		if *existingSubscription.ChangeType != changeType || *existingSubscription.LifecycleNotificationURL != lifecycleNotificationURL || *existingSubscription.NotificationURL != notificationURL || *existingSubscription.ClientState != webhookSecret {
			ct := tc.client.Subscriptions().ID(*existingSubscription.ID).Request()
			if err = ct.Delete(tc.ctx); err != nil {
				tc.logError("Unable to delete the subscription", "error", err, "subscription", existingSubscription)
			}
		} else {
			expirationDateTime := time.Now().Add(30 * time.Minute)
			updatedSubscription := msgraph.Subscription{
				ExpirationDateTime: &expirationDateTime,
			}
			ct := tc.client.Subscriptions().ID(*existingSubscription.ID).Request()
			err = ct.Update(tc.ctx, &updatedSubscription)
			if err == nil {
				return *existingSubscription.ID, nil
			}

			tc.logError("Unable to refresh the subscription", "error", err, "subscription", existingSubscription)
			if err = ct.Delete(tc.ctx); err != nil {
				tc.logError("Unable to delete the subscription", "error", err, "subscription", existingSubscription)
			}
		}
	}

	ct := tc.client.Subscriptions().Request()
	res, err := ct.Add(tc.ctx, &subscription)
	if err != nil {
		tc.logError("Unable to create the new subscription", "error", err)
		return "", err
	}

	return *res.ID, nil
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
	updatedSubscription := msgraph.Subscription{
		ExpirationDateTime: &expirationDateTime,
	}
	ct := tc.client.Subscriptions().ID(subscriptionID).Request()
	err := ct.Update(tc.ctx, &updatedSubscription)
	if err != nil {
		tc.logError("Unable to refresh the subscription", "error", err, "subscriptionID", subscriptionID)
		return err
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
		userID, ok := member.AdditionalData["userId"]
		if !ok {
			userID = ""
		}
		email, ok := member.AdditionalData["email"]
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

func convertToMessage(msg *msgraph.ChatMessage, teamID, channelID, chatID string) *Message {
	data, _ := json.Marshal(msg)
	fmt.Println("==================", string(data), "===================")

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

	lastUpdateAt := time.Now()
	if msg.LastModifiedDateTime != nil {
		lastUpdateAt = *msg.LastModifiedDateTime
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

	reactions := []Reaction{}
	for _, reaction := range msg.Reactions {
		if reaction.ReactionType != nil && reaction.User != nil && reaction.User.User != nil && reaction.User.User.ID != nil {
			reactions = append(reactions, Reaction{UserID: *reaction.User.User.ID, Reaction: *reaction.ReactionType})
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
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	return convertToMessage(res, teamID, channelID, ""), nil
}

func (tc *ClientImpl) GetChatMessage(chatID, messageID string) (*Message, error) {
	ct := tc.client.Chats().ID(chatID).Messages().ID(messageID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	return convertToMessage(res, "", "", chatID), nil
}

func (tc *ClientImpl) GetReply(teamID, channelID, messageID, replyID string) (*Message, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Replies().ID(replyID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	return convertToMessage(res, teamID, channelID, ""), nil
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
	defer res.Body.Close()

	photo, err := io.ReadAll(res.Body)
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
	res, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(res), nil
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
	ct := tc.client.Chats().Request()
	ct.Expand("members")
	ct.Select("members,id")
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return "", err
	}

	chatType := "group"
	if len(usersIDs) == 2 {
		chatType = "oneOnOne"
	}

	for _, c := range res {
		if len(c.Members) == len(usersIDs) {
			matches := map[string]bool{}
			for _, m := range c.Members {
				for _, u := range usersIDs {
					if m.AdditionalData["userId"] == u {
						matches[u] = true
						break
					}
				}
			}
			if len(matches) == len(usersIDs) {
				return *c.ID, nil
			}
		}
	}

	members := make([]msgraph.ConversationMember, len(usersIDs))
	for idx, userID := range usersIDs {
		members[idx] = msgraph.ConversationMember{
			Entity: msgraph.Entity{
				Object: msgraph.Object{
					AdditionalData: map[string]interface{}{
						"@odata.type":     "#microsoft.graph.aadUserConversationMember",
						"user@odata.bind": "https://graph.microsoft.com/v1.0/users('" + userID + "')",
					},
				},
			},
			Roles: []string{"owner"},
		}
	}

	ctn := tc.client.Chats().Request()
	resn, err := ctn.Add(tc.ctx, &msgraph.Chat{
		Entity: msgraph.Entity{
			Object: msgraph.Object{
				AdditionalData: map[string]interface{}{"chatType": chatType},
			},
		},
		Members: members,
	})
	if err != nil {
		return "", err
	}
	return *resn.ID, nil
}

func (tc *ClientImpl) SetChatReaction(chatID, messageID, userID, emoji string) error {
	ctb := tc.client.Chats().ID(chatID).Messages().ID(messageID)
	ctb.SetURL(ctb.URL() + "/setReaction")
	ct := ctb.Request()
	req, err := ct.NewJSONRequest("POST", "", map[string]interface{}{
		"reactionType": emoji,
		"user": map[string]string{
			"id": userID,
		},
	})
	if err != nil {
		return err
	}
	res, err := ct.Client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 204 {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	return nil
}

func (tc *ClientImpl) SetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error {
	if parentID == "" {
		ctb := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID)
		ctb.SetURL(ctb.URL() + "/setReaction")
		ct := ctb.Request()
		req, err := ct.NewJSONRequest("POST", "", map[string]interface{}{
			"reactionType": emoji,
			"user": map[string]string{
				"id": userID,
			},
		})
		if err != nil {
			return err
		}
		res, err := ct.Client().Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()

		if res.StatusCode != 204 {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return errors.New(string(body))
		}
	} else {
		ctb := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(parentID).Replies().ID(messageID)
		ctb.SetURL(ctb.URL() + "/setReaction")
		ct := ctb.Request()
		req, err := ct.NewJSONRequest("POST", "", map[string]interface{}{
			"reactionType": emoji,
			"user": map[string]string{
				"id": userID,
			},
		})
		if err != nil {
			return err
		}
		res, err := ct.Client().Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 204 {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return errors.New(string(body))
		}
	}
	return nil
}

func (tc *ClientImpl) UnsetChatReaction(chatID, messageID, userID, emoji string) error {
	ctb := tc.client.Chats().ID(chatID).Messages().ID(messageID)
	ctb.SetURL(ctb.URL() + "/unsetReaction")
	ct := ctb.Request()
	req, err := ct.NewJSONRequest("POST", "", map[string]interface{}{
		"reactionType": emoji,
		"user": map[string]string{
			"id": userID,
		},
	})
	if err != nil {
		return err
	}
	res, err := ct.Client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != 204 {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return errors.New(string(body))
	}
	return nil
}

func (tc *ClientImpl) UnsetReaction(teamID, channelID, parentID, messageID, userID, emoji string) error {
	if parentID == "" {
		ctb := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID)
		ctb.SetURL(ctb.URL() + "/unsetReaction")
		ct := ctb.Request()
		req, err := ct.NewJSONRequest("POST", "", map[string]interface{}{
			"reactionType": emoji,
			"user": map[string]string{
				"id": userID,
			},
		})
		if err != nil {
			return err
		}
		res, err := ct.Client().Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 204 {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return errors.New(string(body))
		}
	} else {
		ctb := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(parentID).Replies().ID(messageID)
		ctb.SetURL(ctb.URL() + "/unsetReaction")
		ct := ctb.Request()
		req, err := ct.NewJSONRequest("POST", "", map[string]interface{}{
			"reactionType": emoji,
			"user": map[string]string{
				"id": userID,
			},
		})
		if err != nil {
			return err
		}
		res, err := ct.Client().Do(req)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.StatusCode != 204 {
			body, err := io.ReadAll(res.Body)
			if err != nil {
				return err
			}
			return errors.New(string(body))
		}
	}
	return nil
}

func (tc *ClientImpl) ListUsers() ([]User, error) {
	req := tc.client.Users().Request()
	req.Select("displayName,id")
	r, err := req.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	users := make([]User, len(r))
	for i, u := range r {
		users[i] = User{
			DisplayName: *u.DisplayName,
			ID:          *u.ID,
		}
	}
	return users, nil
}

func (tc *ClientImpl) ListTeams() ([]Team, error) {
	req := tc.client.Me().JoinedTeams().Request()
	req.Select("displayName,id, description")
	r, err := req.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	teams := make([]Team, len(r))

	for i, t := range r {
		description := ""
		if t.Description != nil {
			description = *t.Description
		}

		displayName := ""
		if t.DisplayName != nil {
			displayName = *t.DisplayName
		}

		teams[i] = Team{
			DisplayName: displayName,
			Description: description,
			ID:          *t.ID,
		}
	}
	return teams, nil
}

func (tc *ClientImpl) ListChannels(teamID string) ([]Channel, error) {
	req := tc.client.Teams().ID(teamID).Channels().Request()
	req.Select("displayName,id,description")
	r, err := req.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	channels := make([]Channel, len(r))
	for i, c := range r {
		description := ""
		if c.Description != nil {
			description = *c.Description
		}

		displayName := ""
		if c.DisplayName != nil {
			displayName = *c.DisplayName
		}

		channels[i] = Channel{
			DisplayName: displayName,
			Description: description,
			ID:          *c.ID,
		}
	}
	return channels, nil
}
