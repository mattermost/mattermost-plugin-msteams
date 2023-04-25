package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/model"
)

type API struct {
	p      *Plugin
	store  store.Store
	router *mux.Router
}

type Activities struct {
	Value []msteams.Activity
}

func NewAPI(p *Plugin, store store.Store) *API {
	router := mux.NewRouter()
	api := &API{p: p, router: router, store: store}

	router.HandleFunc("/avatar/{userId:.*}", api.getAvatar).Methods("GET")
	router.HandleFunc("/changes", api.processActivity).Methods("POST")
	router.HandleFunc("/lifecycle", api.processLifecycle).Methods("POST")
	router.HandleFunc("/autocomplete/teams", api.autocompleteTeams).Methods("GET")
	router.HandleFunc("/autocomplete/channels", api.autocompleteChannels).Methods("GET")
	router.HandleFunc("/needsConnect", api.needsConnect).Methods("GET", "OPTIONS")
	router.HandleFunc("/connect", api.connect).Methods("GET", "OPTIONS")

	return api
}

// getAvatar returns the microsoft teams avatar
func (a *API) getAvatar(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	userID := params["userId"]
	photo, appErr := a.store.GetAvatarCache(userID)
	if appErr != nil || len(photo) == 0 {
		var err error
		photo, err = a.p.msteamsAppClient.GetUserAvatar(userID)
		if err != nil {
			a.p.API.LogError("Unable to read avatar", "error", err.Error())
			http.Error(w, "avatar not found", http.StatusNotFound)
			return
		}

		err = a.store.SetAvatarCache(userID, photo)
		if err != nil {
			a.p.API.LogError("Unable to cache the new avatar", "error", err.Error())
			return
		}
	}

	if _, err := w.Write(photo); err != nil {
		a.p.API.LogError("Unable to write the response", "Error", err.Error())
	}
}

// processActivity handles the activity received from teams subscriptions
func (a *API) processActivity(w http.ResponseWriter, req *http.Request) {
	validationToken := req.URL.Query().Get("validationToken")
	if validationToken != "" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validationToken))
		return
	}

	activities := Activities{}
	err := json.NewDecoder(req.Body).Decode(&activities)
	if err != nil {
		http.Error(w, "unable to get the activities from the message", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	a.p.API.LogDebug("Change activity request", "activities", activities)
	errors := ""
	for _, activity := range activities.Value {
		if activity.ClientState != a.p.configuration.WebhookSecret {
			errors += "Invalid webhook secret"
			continue
		}
		a.refreshSubscriptionIfNeeded(activity)
		err := a.p.activityHandler.Handle(activity)
		if err != nil {
			a.p.API.LogError("Unable to process created activity", "activity", activity, "error", err.Error())
			errors += err.Error() + "\n"
		}
	}
	if errors != "" {
		http.Error(w, errors, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// TODO: Deduplicate this calls in case multiple activities are sent after the subscription receives the notification
func (a *API) refreshSubscriptionIfNeeded(activity msteams.Activity) {
	if time.Until(activity.SubscriptionExpirationDateTime) < (5 * time.Minute) {
		err := a.p.msteamsAppClient.RefreshSubscription(activity.SubscriptionID)
		if err != nil {
			a.p.API.LogError("Unable to refresh the subscription", "error", err.Error())
		}
	}
}

// processLifecycle handles the lifecycle events received from teams subscriptions
func (a *API) processLifecycle(w http.ResponseWriter, req *http.Request) {
	validationToken := req.URL.Query().Get("validationToken")
	if validationToken != "" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(validationToken))
		return
	}

	lifecycleEvents := Activities{}
	err := json.NewDecoder(req.Body).Decode(&lifecycleEvents)
	if err != nil {
		http.Error(w, "unable to get the lifecycle events from the message", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	a.p.API.LogDebug("Lifecycle activity request", "activities", lifecycleEvents)

	for _, event := range lifecycleEvents.Value {
		if event.ClientState != a.p.configuration.WebhookSecret {
			a.p.API.LogError("Invalid webhook secret recevied in lifecycle event")
			continue
		}
		if event.LifecycleEvent == "reauthorizationRequired" {
			err := a.p.msteamsAppClient.RefreshSubscription(event.SubscriptionID)
			if err != nil {
				a.p.API.LogError("Unable to refresh the subscription", "error", err.Error())
			}
		}
		// TODO: handle "missed" lifecycle event to resync
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

func (a *API) autocompleteTeams(w http.ResponseWriter, r *http.Request) {
	out := []model.AutocompleteListItem{}
	userID := r.Header.Get("Mattermost-User-ID")

	client, err := a.p.GetClientForUser(userID)
	if err != nil {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	teams, err := client.ListTeams()
	if err != nil {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	for _, t := range teams {
		s := model.AutocompleteListItem{
			Item:     t.ID,
			Hint:     t.DisplayName,
			HelpText: t.Description,
		}

		out = append(out, s)
	}
	data, _ := json.Marshal(out)
	_, _ = w.Write(data)
}

func (a *API) autocompleteChannels(w http.ResponseWriter, r *http.Request) {
	out := []model.AutocompleteListItem{}
	userID := r.Header.Get("Mattermost-User-ID")
	args := strings.Fields(r.URL.Query().Get("parsed"))
	if len(args) < 3 {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	client, err := a.p.GetClientForUser(userID)
	if err != nil {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	teamID := args[2]
	channels, err := client.ListChannels(teamID)
	if err != nil {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	for _, c := range channels {
		s := model.AutocompleteListItem{
			Item:     c.ID,
			Hint:     c.DisplayName,
			HelpText: c.Description,
		}

		out = append(out, s)
	}
	data, _ := json.Marshal(out)
	_, _ = w.Write(data)
}

func (a *API) needsConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}

	response := map[string]bool{
		"canSkip":      a.p.configuration.AllowSkipConnectUsers,
		"needsConnect": false,
	}

	if a.p.configuration.EnforceConnectedUsers {
		userID := r.Header.Get("Mattermost-User-ID")
		client, _ := a.p.GetClientForUser(userID)
		if client == nil {
			if a.p.configuration.EnabledTeams == "" {
				response["needsConnect"] = true
			} else {
				enabledTeams := strings.Fields(a.p.configuration.EnabledTeams)

				teams, _ := a.p.API.GetTeamsForUser(userID)
				for _, enabledTeam := range enabledTeams {
					for _, team := range teams {
						if team.Id == enabledTeam {
							response["needsConnect"] = true
							break
						}
					}
				}
			}
		}
	}

	data, _ := json.Marshal(response)
	_, _ = w.Write(data)
}

func (a *API) connect(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}
	userID := r.Header.Get("Mattermost-User-ID")

	messageChan := make(chan string)
	go func(userID string, messageChan chan string) {
		tokenSource, err := msteams.NewUnauthenticatedClient(a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.API.LogError).RequestUserToken(messageChan)
		if err != nil {
			return
		}

		token, err := tokenSource.Token()
		if err != nil {
			return
		}

		client := msteams.NewTokenClient(a.p.configuration.TenantID, a.p.configuration.ClientID, token, a.p.API.LogError)
		if err = client.Connect(); err != nil {
			return
		}

		msteamsUserID, err := client.GetMyID()
		if err != nil {
			return
		}

		err = a.p.store.SetUserInfo(userID, msteamsUserID, token)
		if err != nil {
			return
		}
	}(userID, messageChan)

	message := <-messageChan

	data, _ := json.Marshal(map[string]string{"message": message})
	_, _ = w.Write(data)
}
