package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-matterbridge/server/msteams"
)

type Activities struct {
	Value []msteams.Activity
}

/////////////////////////////////////////////////////////
// Handlers
/////////////////////////////////////////////////////////

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
		err := p.handleActivity(activity)
		if err != nil {
			p.API.LogError("Unable to process activity", "activity", activity, "error", err)
		}
	}
}
