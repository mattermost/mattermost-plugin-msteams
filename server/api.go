package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-matterbridge/server/msteams"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattn/godown"
	"github.com/pkg/errors"
)

type Activity struct {
	Resource       string
	SubscriptionId string
}

type Activities struct {
	Value []Activity
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

func (p *Plugin) msgToPost(link ChannelLink, msg *msteams.Message) (*model.Post, error) {
	text := convertToMD(msg.Text)

	channel, err := p.API.GetChannel(link.MattermostChannel)
	if err != nil {
		p.API.LogError("Unable to get the channel", "error", err)
		return nil, err
	}
	props := make(map[string]interface{})
	rootID := []byte{}

	if msg.ReplyToID != "" {
		rootID, _ = p.API.KVGet("teams_mattermost_" + msg.ReplyToID)
	}

	newText, attachments := p.handleAttachments(channel.Id, text, msg)
	text = newText

	if len(rootID) == 0 && msg.Subject != "" {
		text = "## " + msg.Subject + "\n" + text
	}

	post := &model.Post{UserId: p.userID, ChannelId: channel.Id, Message: text, Props: props, RootId: string(rootID), FileIds: attachments}
	post.AddProp("matterbridge_"+p.userID, true)
	post.AddProp("override_username", msg.UserDisplayName)
	post.AddProp("override_icon_url", p.getURL()+"/avatar/"+msg.UserID)
	post.AddProp("from_webhook", "true")
	return post, nil
}

func (p *Plugin) getAvatar(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	userID := params["userId"]
	photo, appErr := p.API.KVGet("avatar_" + userID)
	if appErr != nil || len(photo) == 0 {
		var err error
		photo, err = p.msteamsAppClient.GetUserAvatar(userID)
		if err != nil {
			p.API.LogError("Unable to read avatar", "error", err)
			return
		}

		appErr := p.API.KVSetWithExpiry("avatar_"+userID, photo, 300)
		if appErr != nil {
			p.API.LogError("Unable to cache the new avatar", "error", appErr)
			return
		}
	}
	w.Write(photo)
}

func (p *Plugin) processActivity(activity Activity) error {
	urlData := parseResourceFields(activity.Resource)
	var msg *msteams.Message
	if reply, ok := urlData["reply"]; ok {
		var err error
		msg, err = p.msteamsBotClient.GetReply(urlData["team"], urlData["channel"], urlData["message"], reply)
		if err != nil {
			p.API.LogError("Unable to get original post", "error", err)
			return err
		}
	} else {
		var err error
		msg, err = p.msteamsBotClient.GetMessage(urlData["team"], urlData["channel"], urlData["message"])
		if err != nil {
			p.API.LogError("Unable to get original post", "error", err)
			return err
		}
	}

	if msg.UserID == "" {
		p.API.LogDebug("Skipping not user event", "msg", msg)
		return nil
	}

	if msg.UserID == p.botID {
		p.API.LogDebug("Skipping messages from bot user")
		return nil
	}

	channelLink, ok := p.subscriptionsToLinks[activity.SubscriptionId]
	if !ok {
		p.API.LogError("Unable to find the subscription")
		return errors.New("Unable to find the subscription")
	}

	post, err := p.msgToPost(channelLink, msg)
	if err != nil {
		p.API.LogError("Unable to transform teams post in mattermost post", "message", msg, "error", err)
		return err
	}

	// Avoid possible duplication
	data, _ := p.API.KVGet("teams_mattermost_" + msg.ID)
	if len(data) != 0 {
		return nil
	}

	newPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		p.API.LogError("Unable to create post", "post", post, "error", appErr)
		return appErr
	}

	if newPost != nil && newPost.Id != "" && msg.ID != "" {
		p.API.KVSet("mattermost_teams_"+newPost.Id, []byte(msg.ID))
		p.API.KVSet("teams_mattermost_"+msg.ID, []byte(newPost.Id))
	}
	return nil
}

func (p *Plugin) processMessage(w http.ResponseWriter, req *http.Request) {
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
		err := p.processActivity(activity)
		if err != nil {
			p.API.LogError("Unable to process activity", "activity", activity, "error", err)
		}
	}
}
