package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
)

type API struct {
	p      *Plugin
	router *mux.Router
}

type Activities struct {
	Value []msteams.Activity
}

func NewAPI(p *Plugin) *API {
	router := mux.NewRouter()
	api := &API{p: p, router: router}

	router.HandleFunc("/avatar/{userId:.*}", api.getAvatar).Methods("GET")
	router.HandleFunc("/", api.processMessage).Methods("POST")

	return api
}

func (a *API) getAvatar(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	userID := params["userId"]
	photo, appErr := a.p.API.KVGet("avatar_" + userID)
	if appErr != nil || len(photo) == 0 {
		var err error
		photo, err = a.p.msteamsAppClient.GetUserAvatar(userID)
		if err != nil {
			a.p.API.LogError("Unable to read avatar", "error", err)
			http.Error(w, "avatar not found", http.StatusNotFound)
			return
		}

		appErr := a.p.API.KVSetWithExpiry("avatar_"+userID, photo, 300)
		if appErr != nil {
			a.p.API.LogError("Unable to cache the new avatar", "error", appErr)
			return
		}
	}
	w.Write(photo)
}

func (a *API) processMessage(w http.ResponseWriter, req *http.Request) {
	validationToken := req.URL.Query().Get("validationToken")
	if validationToken != "" {
		w.Write([]byte(validationToken))
		return
	}

	activities := Activities{}
	err := json.NewDecoder(req.Body).Decode(&activities)
	if err != nil {
		http.Error(w, "unable to get the activities from the message", http.StatusBadRequest)
		return
	}

	errors := ""
	for _, activity := range activities.Value {
		// TODO: Remove this debug line
		a.p.API.LogError("Acivity Info", "activity", activity)
		if activity.ClientState != a.p.configuration.WebhookSecret {
			a.p.API.LogError("Invalid webhook secret", "activity", activity)
			errors = errors + "Invalid webhook secret\n"
			continue
		}
		err := a.p.handleActivity(activity)
		if err != nil {
			a.p.API.LogError("Unable to process activity", "activity", activity, "error", err)
			errors = errors + err.Error() + "\n"
		}
	}
	if errors != "" {
		http.Error(w, errors, http.StatusBadRequest)
		return
	}
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}
