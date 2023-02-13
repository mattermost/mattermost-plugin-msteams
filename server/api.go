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
	photo, appErr := a.p.API.KVGet(avatarKey(userID))
	if appErr != nil || len(photo) == 0 {
		var err error
		photo, err = a.p.msteamsAppClient.GetUserAvatar(userID)
		if err != nil {
			a.p.API.LogError("Unable to read avatar", "error", err)
			http.Error(w, "avatar not found", http.StatusNotFound)
			return
		}

		appErr := a.p.API.KVSetWithExpiry(avatarKey(userID), photo, 300)
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
		a.p.API.LogError("unable to get the activities from the message msteams server subscription message", "error", err)
		http.Error(w, "unable to get the activities from the message", http.StatusBadRequest)
		return
	}

	errors := ""
	for _, activity := range activities.Value {
		a.p.API.LogDebug("=== Recived activity ====", "activity", activity)
		switch activity.ChangeType {
		case "created":
			err := a.p.handleCreatedActivity(activity)
			if err != nil {
				a.p.API.LogError("Unable to process created activity", "activity", activity, "error", err)
				errors = errors + err.Error() + "\n"
			}
		case "updated":
			err := a.p.handleUpdatedActivity(activity)
			if err != nil {
				a.p.API.LogError("Unable to process created activity", "activity", activity, "error", err)
				errors = errors + err.Error() + "\n"
			}
		case "deleted":
			err := a.p.handleDeletedActivity(activity)
			if err != nil {
				a.p.API.LogError("Unable to process deleted activity", "activity", activity, "error", err)
				errors = errors + err.Error() + "\n"
			}
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
