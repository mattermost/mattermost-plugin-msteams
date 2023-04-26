package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-server/v6/model"
	"golang.org/x/oauth2"
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
	router.HandleFunc("/oauth-redirect", api.oauthRedirectHandler).Methods("GET", "OPTIONS")

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
	_, _ = w.Write(photo)
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

	state, _ := json.Marshal(map[string]string{})

	codeVerifier := model.NewId()
	_ = a.p.API.KVSet("_code_verifier_"+userID, []byte(codeVerifier))

	connectURL := msteams.GetAuthURL(a.p.GetURL()+"/oauth-redirect", a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.configuration.ClientSecret, string(state), codeVerifier)

	data, _ := json.Marshal(map[string]string{"message": fmt.Sprintf("Visit the URL for the auth dialog: %v", connectURL)})
	_, _ = w.Write(data)
}

func (a *API) oauthRedirectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}

	userID := r.Header.Get("Mattermost-User-ID")

	teamsDefaultScopes := []string{"https://graph.microsoft.com/.default"}
	conf := &oauth2.Config{
		ClientID:     a.p.configuration.ClientID,
		ClientSecret: a.p.configuration.ClientSecret,
		Scopes:       append(teamsDefaultScopes, "offline_access"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", a.p.configuration.TenantID),
			TokenURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", a.p.configuration.TenantID),
		},
		RedirectURL: a.p.GetURL() + "/oauth-redirect",
	}

	code := r.URL.Query().Get("code")
	stateData := r.URL.Query().Get("state")
	state := map[string]string{}
	err := json.Unmarshal([]byte(stateData), &state)
	if err != nil {
		a.p.API.LogError("Unable to get the code", "error", err.Error())
		http.Error(w, "failed to get the code", http.StatusBadRequest)
		return
	}

	mmUserID := userID
	if authType, ok := state["auth_type"]; ok && authType == "bot" {
		mmUserID = a.p.GetBotUserID()
	}

	codeVerifierBytes, appErr := a.p.API.KVGet("_code_verifier_" + mmUserID)
	if appErr != nil {
		a.p.API.LogError("Unable to get the code verifier", "error", appErr.Error())
		http.Error(w, "failed to get the code verifier", http.StatusBadRequest)
		return
	}
	appErr = a.p.API.KVDelete("_code_verifier_" + mmUserID)
	if appErr != nil {
		a.p.API.LogError("Unable to delete the used code verifier", "error", appErr.Error())
	}

	ctx := context.Background()
	token, err := conf.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", string(codeVerifierBytes)))
	if err != nil {
		a.p.API.LogError("Unable to read avatar", "error", err.Error())
		http.Error(w, "failed to get the code", http.StatusBadRequest)
		return
	}

	client := msteams.NewTokenClient(a.p.configuration.TenantID, a.p.configuration.ClientID, token, a.p.API.LogError)
	if err = client.Connect(); err != nil {
		a.p.API.LogError("Unable connect to the client", "error", err.Error())
		http.Error(w, "failed to connect to the client", http.StatusBadRequest)
		return
	}

	msteamsUserID, err := client.GetMyID()
	if err != nil {
		a.p.API.LogError("Unable to get the MS Teams user id", "error", err.Error())
		http.Error(w, "failed to get the MS Teams user id", http.StatusBadRequest)
		return
	}

	err = a.p.store.SetUserInfo(mmUserID, msteamsUserID, token)
	if err != nil {
		a.p.API.LogError("Unable to store the token", "error", err.Error())
		http.Error(w, "failed to store the token", http.StatusBadRequest)
		return
	}

	w.Header().Add("Content-Type", "text/html")
	_, _ = w.Write([]byte("<html><body><h1>Your account has been connected</h1><p>You can close this window.</p></body></html>"))
}
