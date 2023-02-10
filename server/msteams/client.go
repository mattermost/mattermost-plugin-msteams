//go:generate mockery --name=Client
package msteams

import (
	"context"
	"io/ioutil"
	"strings"
	"time"

	msgraph "github.com/yaegashi/msgraph.go/beta"
	"github.com/yaegashi/msgraph.go/msauth"
	"golang.org/x/oauth2"
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
	clientType   string // can be "bot" or "app"
}

type Channel struct {
	ID          string
	DisplayName string
}

type Team struct {
	ID          string
	DisplayName string
}

type Attachment struct {
	ContentType string
	Content     string
	Name        string
	ContentURL  string
}

type Message struct {
	ID              string
	UserID          string
	UserDisplayName string
	Text            string
	Subject         string
	ReplyToID       string
	Attachments     []Attachment
}

type Activity struct {
	Resource       string
	SubscriptionId string
	ClientState    string
}

type ActivityIds struct {
	TeamID    string
	ChannelID string
	MessageID string
	ReplyID   string
}

const teamsDefaultScope = "https://graph.microsoft.com/.default"

func NewApp(tenantId, clientId, clientSecret string) *ClientImpl {
	return &ClientImpl{
		ctx:          context.Background(),
		clientType:   "app",
		tenantId:     tenantId,
		clientId:     clientId,
		clientSecret: clientSecret,
	}
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

func (tc *ClientImpl) Connect() error {
	var ts oauth2.TokenSource
	if tc.clientType == "bot" {
		var err error
		m := msauth.NewManager()
		ts, err = m.ResourceOwnerPasswordGrant(
			tc.ctx,
			tc.tenantId,
			tc.clientId,
			tc.clientSecret,
			tc.botUsername,
			tc.botPassword,
			[]string{teamsDefaultScope},
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
			[]string{teamsDefaultScope},
		)
		if err != nil {
			return err
		}
	} else {
		panic("Not valid client type, this shouldn't happen ever.")
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

func (tc *ClientImpl) SendMessage(teamID, channelID, parentID, message string) (string, error) {
	content := &msgraph.ItemBody{Content: &message}
	rmsg := &msgraph.ChatMessage{Body: content}

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

func (tc *ClientImpl) SubscribeToChannel(teamID, channelID, notificationURL string, webhookSecret string) (string, error) {
	resource := "teams/" + teamID + "/channels/" + channelID + "/messages"
	expirationDateTime := time.Now().Add(60 * time.Minute)
	changeType := "created"
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
			return nil
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

func converToMessage(msg *msgraph.ChatMessage) *Message {
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
	}

}

func (tc *ClientImpl) GetMessage(teamID, channelID, messageID string) (*Message, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}
	return converToMessage(res), nil
}

func (tc *ClientImpl) GetReply(teamID, channelID, messageID, replyID string) (*Message, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Replies().ID(replyID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	return converToMessage(res), nil
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
