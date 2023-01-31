package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattn/godown"
	"github.com/pkg/errors"
	msgraph "github.com/yaegashi/msgraph.go/beta"
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

func (p *Plugin) msgToPost(link ChannelLink, msg *msgraph.ChatMessage) (*model.Post, error) {
	text := convertToMD(*msg.Body.Content)

	// TODO: Fix attachments
	// b.handleAttachments(&rmsg, msg)
	// b.Log.Debugf("<= Message is %#v", rmsg)
	// b.Remote <- rmsg

	channel, err := p.API.GetChannel(link.MattermostChannel)
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
	post.AddProp("matterbridge_"+p.userID, true)
	post.AddProp("override_username", *msg.From.User.DisplayName)
	// TODO: Make this more robust
	post.AddProp("override_icon_url", "https://matterbridge-jespino.eu.ngrok.io/plugins/com.mattermost.matterbridge-plugin/avatar/"+*msg.From.User.ID)
	post.AddProp("from_webhook", "true")
	p.API.LogError("creating post", "post", post)
	return post, nil
}

func (p *Plugin) getMSTeamsUserAvatar(userID string) ([]byte, error) {
	ctb := p.msteamsAppClient.Users().ID(userID).Photo()
	ctb.SetURL(ctb.URL() + "/$value")
	ct := ctb.Request()
	req, err := ct.NewRequest("GET", "", nil)
	if err != nil {
		p.API.LogError("Unable to generate avatar request", "error", err)
		return nil, err
	}
	res, err := ct.Client().Do(req)
	if err != nil {
		p.API.LogError("Unable to get user avatar", "error", err)
		return nil, err
	}
	photo, err := ioutil.ReadAll(res.Body)
	if err != nil {
		p.API.LogError("Unable to read avatar response body", "error", err)
		return nil, err
	}

	return photo, nil
}

func (p *Plugin) getAvatar(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	userID := params["userId"]
	photo, appErr := p.API.KVGet("avatar_" + userID)
	if appErr != nil || len(photo) == 0 {
		var err error
		photo, err = p.getMSTeamsUserAvatar(userID)
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
			return err
		}
	} else {
		var err error
		rct, err = ct.Request().Get(p.msteamsBotClientCtx)
		if err != nil {
			p.API.LogError("Unable to get original post", "error", err)
			return err
		}
	}

	if rct.From == nil || rct.From.User == nil || rct.From.User.ID == nil {
		p.API.LogDebug("Skipping not user event")
		return nil
	}

	if *rct.From.User.ID == p.botID {
		p.API.LogDebug("Skipping messages from bot user")
		return nil
	}

	channelLink, ok := p.subscriptionsToLinks[activity.SubscriptionId]
	if !ok {
		p.API.LogError("Unable to find the subscription")
		return errors.New("Unable to find the subscription")
	}

	post, err := p.msgToPost(channelLink, rct)
	if err != nil {
		p.API.LogError("Unable to transform teams post in mattermost post", "post", rct, "error", err)
		return err
	}

	newPost, appErr := p.API.CreatePost(post)
	if appErr != nil {
		p.API.LogError("Unable to create post", "post", post, "error", appErr)
		return appErr
	}

	if newPost != nil && newPost.Id != "" && rct.ID != nil && *rct.ID != "" {
		p.API.KVSet("mattermost_teams_"+newPost.Id, []byte(*rct.ID))
		p.API.KVSet("teams_mattermost_"+*rct.ID, []byte(newPost.Id))
	}
	return nil
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
		err := p.processActivity(activity)
		if err != nil {
			p.API.LogError("Unable to process activity", "activity", activity, "error", err)
		}
	}
}
