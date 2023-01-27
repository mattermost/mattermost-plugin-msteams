package bmsteams

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/42wim/matterbridge/bridge"
	"github.com/42wim/matterbridge/bridge/config"
	"github.com/davecgh/go-spew/spew"
	"github.com/mattermost/mattermost-server/v6/plugin"

	"github.com/mattn/godown"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"github.com/yaegashi/msgraph.go/msauth"

	"golang.org/x/oauth2"
)

var (
	defaultScopes = []string{"openid", "profile", "offline_access", "Group.Read.All", "Group.ReadWrite.All"}
	attachRE      = regexp.MustCompile(`<attachment id=.*?attachment>`)
)

type Bmsteams struct {
	gc    *msgraph.GraphServiceRequestBuilder
	ctx   context.Context
	botID string
	*bridge.Config
	api plugin.API
}

func New(cfg *bridge.Config, api plugin.API) bridge.Bridger {
	return &Bmsteams{Config: cfg, api: api}
}

func (b *Bmsteams) Connect() error {
	tokenCachePath := b.GetString("sessionFile")
	if tokenCachePath == "" {
		tokenCachePath = "msteams_session.json"
	}
	ctx := context.Background()
	m := msauth.NewManager()
	m.LoadFile(tokenCachePath) //nolint:errcheck
	ts, err := m.DeviceAuthorizationGrant(ctx, b.GetString("TenantID"), b.GetString("ClientID"), defaultScopes, nil)
	if err != nil {
		return err
	}
	err = m.SaveFile(tokenCachePath)
	if err != nil {
		b.Log.Errorf("Couldn't save sessionfile in %s: %s", tokenCachePath, err)
	}
	// make file readable only for matterbridge user
	err = os.Chmod(tokenCachePath, 0o600)
	if err != nil {
		b.Log.Errorf("Couldn't change permissions for %s: %s", tokenCachePath, err)
	}
	httpClient := oauth2.NewClient(ctx, ts)
	graphClient := msgraph.NewClient(httpClient)
	b.gc = graphClient
	b.ctx = ctx

	err = b.setBotID()
	if err != nil {
		return err
	}
	b.Log.Info("Connection succeeded")
	return nil
}

func (b *Bmsteams) Disconnect() error {
	return nil
}

func (b *Bmsteams) JoinChannel(channel config.ChannelInfo) error {
	go func(name string) {
		for {
			err := b.poll(name)
			if err != nil {
				b.Log.Errorf("polling failed for %s: %s. retrying in 5 seconds", name, err)
			}
			time.Sleep(time.Second * 5)
		}
	}(channel.Name)
	return nil
}

func (b *Bmsteams) Send(msg config.Message) (string, error) {
	b.Log.Debugf("\n\n\n=> Receiving %#v", msg)
	if id, ok := msg.Extra["OriginalID"]; ok {
		if len(id) == 1 && id[0].(string) != "" {
			msg.ID = id[0].(string)
		}
	}
	if id, ok := msg.Extra["ParentID"]; ok {
		if len(id) == 1 && id[0].(string) != "" {
			msg.ParentID = id[0].(string)
			return b.sendReply(msg)
		}
	}

	// Handle prefix hint for unthreaded messages.
	if msg.ParentNotFound() {
		msg.ParentID = ""
		msg.Text = fmt.Sprintf("[thread]: %s", msg.Text)
	}

	ct := b.gc.Teams().ID(b.GetString("TeamID")).Channels().ID(msg.Channel).Messages().Request()
	text := msg.Username + msg.Text
	content := &msgraph.ItemBody{Content: &text}
	rmsg := &msgraph.ChatMessage{Body: content}
	res, err := ct.Add(b.ctx, rmsg)
	if err != nil {
		return "", err
	}
	b.api.LogError("STORING KEY VALUES FOR ", "msgID", msg.ID, "resID", *res.ID)
	if msg.ID != "" && *res.ID != "" {
		b.api.LogError("YES!!")
		b.api.KVSet("mattermost_teams_"+msg.ID, []byte(*res.ID))
		b.api.KVSet("teams_mattermost_"+*res.ID, []byte(msg.ID))
	}
	return *res.ID, nil
}

func (b *Bmsteams) sendReply(msg config.Message) (string, error) {
	b.api.LogError("WRITTING A REPLY")
	ct := b.gc.Teams().ID(b.GetString("TeamID")).Channels().ID(msg.Channel).Messages().ID(msg.ParentID).Replies().Request()
	// Handle prefix hint for unthreaded messages.

	text := msg.Username + msg.Text
	content := &msgraph.ItemBody{Content: &text}
	rmsg := &msgraph.ChatMessage{Body: content}

	b.Log.Infof("\n\n\n ===== sent message reply %#v request", ct)
	res, err := ct.Add(b.ctx, rmsg)
	if err != nil {
		return "", err
	}
	b.api.LogError("STORING KEY VALUES FOR ", "msgID", msg.ID, "resID", *res.ID)
	if msg.ID != "" && *res.ID != "" {
		b.api.LogError("YES!!")
		b.api.KVSet("mattermost_teams_"+msg.ID, []byte(*res.ID))
		b.api.KVSet("teams_mattermost_"+*res.ID, []byte(msg.ID))
	}
	return *res.ID, nil
}

func (b *Bmsteams) getMessages(channel string) ([]msgraph.ChatMessage, error) {
	ct := b.gc.Teams().ID(b.GetString("TeamID")).Channels().ID(channel).Messages().Request()
	ct.Expand("replies")
	rct, err := ct.Get(b.ctx)
	if err != nil {
		return nil, err
	}
	b.Log.Debugf("got %#v messages", len(rct))
	allMessages := []msgraph.ChatMessage{}
	for _, msg := range rct {
		allMessages = append(allMessages, msg)
		allMessages = append(allMessages, msg.Replies...)
	}
	b.Log.Infof("got %#v messages with replies", len(allMessages))
	return allMessages, nil
}

//nolint:gocognit
func (b *Bmsteams) poll(channelName string) error {
	msgmap := make(map[string]time.Time)
	b.Log.Debug("getting initial messages")
	res, err := b.getMessages(channelName)
	if err != nil {
		return err
	}
	for _, msg := range res {
		msgmap[*msg.ID] = *msg.CreatedDateTime
		if msg.LastModifiedDateTime != nil {
			msgmap[*msg.ID] = *msg.LastModifiedDateTime
		}
	}
	time.Sleep(time.Second * 5)
	b.Log.Debug("polling for messages")
	for {
		res, err := b.getMessages(channelName)
		if err != nil {
			return err
		}
		for i := len(res) - 1; i >= 0; i-- {
			msg := res[i]
			if mtime, ok := msgmap[*msg.ID]; ok {
				if mtime == *msg.CreatedDateTime && msg.LastModifiedDateTime == nil {
					continue
				}
				if msg.LastModifiedDateTime != nil && mtime == *msg.LastModifiedDateTime {
					continue
				}
			}

			if b.GetBool("debug") {
				b.Log.Debug("Msg dump: ", spew.Sdump(msg))
			}

			// skip non-user message for now.
			if msg.From == nil || msg.From.User == nil {
				continue
			}

			if *msg.From.User.ID == b.botID {
				b.Log.Debug("skipping own message")
				msgmap[*msg.ID] = *msg.CreatedDateTime
				continue
			}

			msgmap[*msg.ID] = *msg.CreatedDateTime
			if msg.LastModifiedDateTime != nil {
				msgmap[*msg.ID] = *msg.LastModifiedDateTime
			}
			b.Log.Debugf("<= Sending message from %s on %s to gateway", *msg.From.User.DisplayName, b.Account)
			text := b.convertToMD(*msg.Body.Content)

			replyID := ""
			if msg.ReplyToID != nil {
				replyID = *msg.ReplyToID
			}

			rmsg := config.Message{
				Username: *msg.From.User.DisplayName,
				Text:     text,
				Channel:  channelName,
				Account:  b.Account,
				Avatar:   "",
				UserID:   *msg.From.User.ID,
				ID:       *msg.ID,
				ParentID: replyID,
				Extra: map[string][]interface{}{
					"OriginalID": {*msg.ID},
					"ParentID":   {replyID},
				},
			}

			b.handleAttachments(&rmsg, msg)
			b.Log.Debugf("<= Message is %#v", rmsg)
			b.Remote <- rmsg
		}
		time.Sleep(time.Second * 5)
	}
}

func (b *Bmsteams) setBotID() error {
	req := b.gc.Me().Request()
	r, err := req.Get(b.ctx)
	if err != nil {
		return err
	}
	b.botID = *r.ID
	return nil
}

func (b *Bmsteams) convertToMD(text string) string {
	if !strings.Contains(text, "<div>") {
		return text
	}
	var sb strings.Builder
	err := godown.Convert(&sb, strings.NewReader(text), nil)
	if err != nil {
		b.Log.Errorf("Couldn't convert message to markdown %s", text)
		return text
	}
	return sb.String()
}
