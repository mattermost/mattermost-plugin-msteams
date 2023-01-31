package msteams

import (
	"context"
	"io/ioutil"
	"time"

	msgraph "github.com/yaegashi/msgraph.go/beta"
	"github.com/yaegashi/msgraph.go/msauth"
	"golang.org/x/oauth2"
)

type Client struct {
	client *msgraph.GraphServiceRequestBuilder
	ctx    context.Context
	botID  string
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

const teamsDefaultScope = "https://graph.microsoft.com/.default"

func NewApp(tenantId, clientId, clientSecret string) (*Client, error) {
	client := Client{
		ctx: context.Background(),
	}

	m := msauth.NewManager()
	ts, err := m.ClientCredentialsGrant(
		client.ctx,
		tenantId,
		clientId,
		clientSecret,
		[]string{teamsDefaultScope},
	)
	if err != nil {
		return nil, err
	}

	httpClient := oauth2.NewClient(client.ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	client.client = graphClient

	return &client, nil
}

func NewBot(tenantId, clientId, clientSecret, botUsername, botPassword string) (*Client, error) {
	client := Client{
		ctx: context.Background(),
	}

	m := msauth.NewManager()
	ts, err := m.ResourceOwnerPasswordGrant(
		client.ctx,
		tenantId,
		clientId,
		clientSecret,
		botUsername,
		botPassword,
		[]string{teamsDefaultScope},
	)
	if err != nil {
		return nil, err
	}

	httpClient := oauth2.NewClient(client.ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	client.client = graphClient

	req := graphClient.Me().Request()
	r, err := req.Get(client.ctx)
	if err != nil {
		return nil, err
	}
	client.botID = *r.ID

	return &client, nil
}

func (tc *Client) SendMessage(teamID, channelID, parentID, message string) (string, error) {
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

func (tc *Client) SubscribeToChannel(teamID, channelID, notificationURL string) (string, error) {
	resource := "teams/" + teamID + "/channels/" + channelID + "/messages"
	expirationDateTime := time.Now().Add(60 * time.Minute)
	clientState := "secret"
	changeType := "created"
	subscription := msgraph.Subscription{
		Resource:           &resource,
		ExpirationDateTime: &expirationDateTime,
		NotificationURL:    &notificationURL,
		ClientState:        &clientState,
		ChangeType:         &changeType,
	}
	ct := tc.client.Subscriptions().Request()
	res, err := ct.Add(tc.ctx, &subscription)
	if err != nil {
		return "", err
	}
	return *res.ID, nil
}

func (tc *Client) RefreshSubscriptionPeriodically(ctx context.Context, subscriptionID string) error {
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

func (tc *Client) ClearSubscriptions() error {
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

func (tc *Client) GetTeam(teamID string) (*Team, error) {
	ct := tc.client.Teams().ID(teamID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	return &Team{ID: *res.ID, DisplayName: *res.DisplayName}, nil
}

func (tc *Client) GetChannel(teamID, channelID string) (*Channel, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	return &Channel{ID: *res.ID, DisplayName: *res.DisplayName}, nil
}

func (tc *Client) GetMessage(teamID, channelID, messageID string) (*Message, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	userID := ""
	if res.From == nil || res.From.User == nil || res.From.User.ID == nil {
		userID = *res.From.User.ID
	}
	userDisplayName := ""
	if res.From == nil || res.From.User == nil || res.From.User.DisplayName == nil {
		userDisplayName = *res.From.User.ID
	}

	msg := &Message{
		ID:              *res.ID,
		UserID:          userID,
		UserDisplayName: userDisplayName,
		Text:            *res.Body.Content,
		ReplyToID:       *res.ReplyToID,
	}

	return msg, nil
}

func (tc *Client) GetReply(teamID, channelID, messageID, replyID string) (*Message, error) {
	ct := tc.client.Teams().ID(teamID).Channels().ID(channelID).Messages().ID(messageID).Replies().ID(replyID).Request()
	res, err := ct.Get(tc.ctx)
	if err != nil {
		return nil, err
	}

	userID := ""
	if res.From == nil || res.From.User == nil || res.From.User.ID == nil {
		userID = *res.From.User.ID
	}
	userDisplayName := ""
	if res.From == nil || res.From.User == nil || res.From.User.DisplayName == nil {
		userDisplayName = *res.From.User.ID
	}

	msg := &Message{
		ID:              *res.ID,
		UserID:          userID,
		UserDisplayName: userDisplayName,
		Text:            *res.Body.Content,
		ReplyToID:       *res.ReplyToID,
	}

	return msg, nil
}

func (tc *Client) GetUserAvatar(userID string) ([]byte, error) {
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

func (tc *Client) GetFileURL(weburl string) (string, error) {
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

func (tc *Client) GetCodeSnippet(url string) (string, error) {
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
