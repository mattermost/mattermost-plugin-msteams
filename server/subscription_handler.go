package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattn/godown"
	"github.com/pkg/errors"
	msgraph "github.com/yaegashi/msgraph.go/beta"
)

type Activities struct {
	Value []struct {
		Resource       string
		SubscriptionId string
	}
}

// HTTPHandler handles the HTTP requests from then connector service
type HTTPHandler struct {
	p *Plugin
}

func parseResourceFields(resource string) map[string]string {
	result := map[string]string{}
	data := strings.Split(resource, "/")
	result["team"] = data[0][7 : len(data[0])-2]
	result["channel"] = data[1][10 : len(data[1])-2]
	result["message"] = data[2][10 : len(data[2])-2]
	if len(data) > 3 {
		result["reply"] = data[3][9 : len(data[3])-2]
	}
	return result
}

func convertToMD(text string) string {
	if !strings.Contains(text, "<div>") {
		return text
	}
	var sb strings.Builder
	err := godown.Convert(&sb, strings.NewReader(text), nil)
	if err != nil {
		return text
	}
	return sb.String()
}

func (p *Plugin) msgToPost(link ChannelLink, msg *msgraph.ChatMessage) (*model.Post, error) {
	text := convertToMD(*msg.Body.Content)

	// TODO: Fix attachments
	// b.handleAttachments(&rmsg, msg)
	// b.Log.Debugf("<= Message is %#v", rmsg)
	// b.Remote <- rmsg

	// TODO: Pick the channel name from the right place
	channelTeamName := "test:off-topic"
	splittedName := strings.Split(channelTeamName, ":")
	if len(splittedName) != 2 {
		return nil, errors.New("Invalid channel name")
	}

	teamName := splittedName[0]
	channelName := splittedName[1]
	channel, err := p.API.GetChannelByNameForTeamName(teamName, channelName, false)
	if err != nil {
		p.API.LogError("Unable to get the channel", "error", err)
		return nil, err
	}
	props := make(map[string]interface{})
	rootID := []byte{}

	if msg.ReplyToID != nil && *msg.ReplyToID != "" {
		rootID, _ = p.API.KVGet("teams_mattermost_" + *msg.ReplyToID)
	}

	post := &model.Post{UserId: p.userID, ChannelId: channel.Id, Message: text, Props: props, RootId: string(rootID)}
	p.API.LogError("Creating new post with original id", "msgId", msg.ID)
	p.API.LogError("Creating new post with original id", "msg", msg)
	post.AddProp("matterbridge_"+p.userID, true)
	post.AddProp("override_username", *msg.From.User.DisplayName)
	post.AddProp("from_webhook", "true")
	p.API.LogError("creating post", "post", post)
	return post, nil
}

func (p *Plugin) processMessage(w http.ResponseWriter, req *http.Request) {
	p.API.LogError("PROCCESING MESSAGE")

	validationToken := req.URL.Query().Get("validationToken")
	if validationToken != "" {
		w.Write([]byte(validationToken))
		return
	}

	activities := Activities{}
	err := json.NewDecoder(req.Body).Decode(&activities)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p.API.LogInfo("Activities", "activities", activities)

	for _, activity := range activities.Value {
		p.API.LogInfo("Activity", "activity", activity)
		urlData := parseResourceFields(activity.Resource)
		p.API.LogInfo("URLDATA", "urlData", urlData)
		ct := p.msteamsBotClient.Teams().ID(urlData["team"]).Channels().ID(urlData["channel"]).Messages().ID(urlData["message"])
		var rct *msgraph.ChatMessage
		if reply, ok := urlData["reply"]; ok {
			var err error
			rct, err = ct.Replies().ID(reply).Request().Get(p.msteamsBotClientCtx)
			if err != nil {
				p.API.LogError("Unable to get original post", "error", err)
				continue
			}
		} else {
			var err error
			rct, err = ct.Request().Get(p.msteamsBotClientCtx)
			if err != nil {
				p.API.LogError("Unable to get original post", "error", err)
				continue
			}
		}
		if rct.From.User.ID != nil && *rct.From.User.ID == p.botID {
			p.API.LogInfo("Skipping messages from bot user")
			continue
		}

		p.API.LogInfo("Post info", "post_info", rct, "error", err)
		channelLink, ok := p.subscriptionsToLinks[activity.SubscriptionId]
		if !ok {
			p.API.LogError("Unable to find the subscription")
			continue
		}

		post, err := p.msgToPost(channelLink, rct)
		if err != nil {
			p.API.LogError("Unable to transform teams post in mattermost post", "post", rct, "error", err)
			continue
		}
		newPost, appErr := p.API.CreatePost(post)
		if appErr != nil {
			p.API.LogError("Unable to create post", "post", post, "error", appErr)
			continue
		}
		p.API.LogError("Storing new post metadata", "newPostId", newPost.Id, "msteamsPostId", *rct.ID)
		if newPost != nil && newPost.Id != "" && rct.ID != nil && *rct.ID != "" {
			p.API.KVSet("mattermost_teams_"+newPost.Id, []byte(*rct.ID))
			p.API.KVSet("teams_mattermost_"+*rct.ID, []byte(newPost.Id))
		}
		p.API.LogError("POST CREATED")
	}
}
