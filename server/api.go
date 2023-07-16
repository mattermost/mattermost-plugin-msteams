package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"golang.org/x/oauth2"

	"github.com/mattermost/mattermost-server/v6/model"
)

const (
	HeaderMattermostUserID = "Mattermost-User-ID"

	// Query params
	QueryParamTeamID     = "team_id"
	QueryParamSearchTerm = "search_term"
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

	autocompleteRouter := router.PathPrefix("/autocomplete").Subrouter()

	router.HandleFunc("/avatar/{userId:.*}", api.getAvatar).Methods(http.MethodGet)
	router.HandleFunc("/changes", api.processActivity).Methods(http.MethodPost)
	router.HandleFunc("/lifecycle", api.processLifecycle).Methods(http.MethodPost)
	router.HandleFunc("/needsConnect", api.handleAuthRequired(api.needsConnect)).Methods(http.MethodGet)
	router.HandleFunc("/connect", api.handleAuthRequired(api.connect)).Methods(http.MethodGet)
	router.HandleFunc("/disconnect", api.handleAuthRequired(api.checkUserConnected(api.disconnect))).Methods(http.MethodGet)
	router.HandleFunc("/connected-channels", api.handleAuthRequired(api.getConnectedChannels)).Methods(http.MethodGet)
	router.HandleFunc("/ms-teams-team-list", api.handleAuthRequired(api.checkUserConnected(api.getMSTeamsTeamList))).Methods(http.MethodGet)
	router.HandleFunc("/ms-teams-team-channels", api.handleAuthRequired(api.checkUserConnected(api.getMSTeamsTeamChannels))).Methods(http.MethodGet)
	router.HandleFunc("/oauth-redirect", api.oauthRedirectHandler).Methods(http.MethodGet)

	// Command autocomplete APIs
	autocompleteRouter.HandleFunc("/teams", api.autocompleteTeams).Methods(http.MethodGet)
	autocompleteRouter.HandleFunc("/channels", api.autocompleteChannels).Methods(http.MethodGet)

	// iFrame support
	router.HandleFunc("/iframe/mattermostTab", api.iFrame).Methods(http.MethodGet)
	router.HandleFunc("/iframe-manifest", api.iFrameManifest).Methods(http.MethodGet)

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
	refreshedSubscriptions := make(map[string]bool)
	for _, activity := range activities.Value {
		if activity.ClientState != a.p.getConfiguration().WebhookSecret {
			errors += "Invalid webhook secret"
			continue
		}

		if !refreshedSubscriptions[activity.SubscriptionID] {
			refreshedSubscriptions[activity.SubscriptionID] = true
			a.refreshSubscriptionIfNeeded(activity)
		}

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

func (a *API) refreshSubscriptionIfNeeded(activity msteams.Activity) {
	if time.Until(activity.SubscriptionExpirationDateTime) < (5 * time.Minute) {
		expiresOn, err := a.p.msteamsAppClient.RefreshSubscription(activity.SubscriptionID)
		if err != nil {
			a.p.API.LogError("Unable to refresh the subscription", "error", err.Error())
		} else {
			if err2 := a.p.store.UpdateSubscriptionExpiresOn(activity.SubscriptionID, *expiresOn); err2 != nil {
				a.p.API.LogError("Unable to store the subscription new expires date", "error", err2.Error())
			}
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
		if event.ClientState != a.p.getConfiguration().WebhookSecret {
			a.p.API.LogError("Invalid webhook secret recevied in lifecycle event")
			continue
		}
		a.p.activityHandler.HandleLifecycleEvent(event, a.p.getConfiguration().WebhookSecret, a.p.getConfiguration().EvaluationAPI)
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

func (a *API) autocompleteTeams(w http.ResponseWriter, r *http.Request) {
	out := []model.AutocompleteListItem{}
	userID := r.Header.Get(HeaderMattermostUserID)

	teams, _, err := a.p.GetMSTeamsTeamList(userID)
	if err != nil {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	a.p.API.LogDebug("Successfully fetched the list of teams", "Count", len(teams))
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
	userID := r.Header.Get(HeaderMattermostUserID)
	args := strings.Fields(r.URL.Query().Get("parsed"))
	if len(args) < 3 {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	client, err := a.p.GetClientForUser(userID)
	if err != nil {
		a.p.API.LogError("Unable to get the client for user", "Error", err.Error())
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	teamID := args[2]
	channels, err := client.ListChannels(teamID)
	if err != nil {
		a.p.API.LogError("Unable to get the channels for MS Teams team", "TeamID", teamID, "Error", err.Error())
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	a.p.API.LogDebug("Successfully fetched the list of channels for MS Teams team", "TeamID", teamID, "Count", len(channels))
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
	response := map[string]bool{
		"canSkip":      a.p.getConfiguration().AllowSkipConnectUsers,
		"needsConnect": false,
	}

	if a.p.getConfiguration().EnforceConnectedUsers {
		userID := r.Header.Get(HeaderMattermostUserID)
		client, _ := a.p.GetClientForUser(userID)
		if client == nil {
			if a.p.getConfiguration().EnabledTeams == "" {
				response["needsConnect"] = true
			} else {
				enabledTeams := strings.Fields(a.p.getConfiguration().EnabledTeams)

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

// TODO: Add unit tests
func (a *API) connect(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(HeaderMattermostUserID)

	state := fmt.Sprintf("%s_%s", model.NewId(), userID)
	if err := a.store.StoreOAuth2State(state); err != nil {
		a.p.API.LogError("Error in storing the OAuth state", "error", err.Error())
		http.Error(w, "Error trying to connect the account, please try again.", http.StatusInternalServerError)
		return
	}

	codeVerifier := model.NewId()
	if appErr := a.p.API.KVSet("_code_verifier_"+userID, []byte(codeVerifier)); appErr != nil {
		a.p.API.LogError("Error in storing the code verifier", "error", appErr.Error())
		http.Error(w, "Error trying to connect the account, please try again.", http.StatusInternalServerError)
		return
	}

	connectURL := msteams.GetAuthURL(a.p.GetURL()+"/oauth-redirect", a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.configuration.ClientSecret, state, codeVerifier)

	data, _ := json.Marshal(map[string]string{"connectUrl": connectURL})
	_, _ = w.Write(data)
}

func (a *API) disconnect(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(HeaderMattermostUserID)
	teamsUserID, err := a.p.store.MattermostToTeamsUserID(userID)
	if err != nil {
		a.p.API.LogError("Unable to get Teams user ID from Mattermost user ID.", "UserID", userID, "Error", err.Error())
		http.Error(w, "Unable to get Teams user ID from Mattermost user ID.", http.StatusInternalServerError)
		return
	}

	if err = a.p.store.SetUserInfo(userID, teamsUserID, nil); err != nil {
		a.p.API.LogError("Error occurred while disconnecting the user.", "UserID", userID, "Error", err.Error())
		http.Error(w, "Error occurred while disconnecting the user.", http.StatusInternalServerError)
		return
	}

	if _, err = w.Write([]byte("Your account has been disconnected.")); err != nil {
		a.p.API.LogError("Failed to write response", "Error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *API) getConnectedChannels(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(HeaderMattermostUserID)
	links, err := a.p.store.ListChannelLinksWithNames()
	if err != nil {
		a.p.API.LogError("Error occurred while getting the linked channels", "Error", err.Error())
		http.Error(w, "Error occurred while getting the linked channels", http.StatusInternalServerError)
		return
	}

	paginatedLinks := []*storemodels.ChannelLink{}
	if len(links) > 0 {
		sort.Slice(links, func(i, j int) bool {
			return fmt.Sprintf("%s_%s", links[i].MattermostChannelName, links[i].MattermostChannelID) < fmt.Sprintf("%s_%s", links[j].MattermostChannelName, links[j].MattermostChannelID)
		})

		msTeamsTeamIDsVsNames, msTeamsChannelIDsVsNames, errorsFound := a.p.GetMSTeamsTeamAndChannelDetailsFromChannelLinks(links, userID, true)
		if errorsFound {
			http.Error(w, "Unable to get the MS Teams teams details", http.StatusInternalServerError)
			return
		}

		offset, limit := a.p.GetOffsetAndLimitFromQueryParams(r.URL.Query())
		for index := offset; index < offset+limit && index < len(links); index++ {
			link := links[index]
			if msTeamsChannelIDsVsNames[link.MSTeamsChannelID] != "" && msTeamsTeamIDsVsNames[link.MSTeamsTeamID] != "" {
				channel, appErr := a.p.API.GetChannel(link.MattermostChannelID)
				if appErr != nil {
					a.p.API.LogError("Error occurred while getting the channel details", "ChannelID", link.MattermostChannelID, "Error", appErr.Message)
					http.Error(w, "Error occurred while getting the channel details", http.StatusInternalServerError)
					return
				}

				paginatedLinks = append(paginatedLinks, &storemodels.ChannelLink{
					MattermostTeamID:      link.MattermostTeamID,
					MattermostChannelID:   link.MattermostChannelID,
					MSTeamsTeamID:         link.MSTeamsTeamID,
					MSTeamsChannelID:      link.MSTeamsChannelID,
					MattermostChannelName: link.MattermostChannelName,
					MattermostTeamName:    link.MattermostTeamName,
					MSTeamsChannelName:    msTeamsChannelIDsVsNames[link.MSTeamsChannelID],
					MSTeamsTeamName:       msTeamsTeamIDsVsNames[link.MSTeamsTeamID],
					MattermostChannelType: string(channel.Type),
				})
			}
		}
	}

	a.writeJSON(w, http.StatusOK, paginatedLinks)
}

func (a *API) getMSTeamsTeamList(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get(HeaderMattermostUserID)
	teams, statusCode, err := a.p.GetMSTeamsTeamList(userID)
	if err != nil {
		http.Error(w, "Error occurred while fetching the MS Teams team list.", statusCode)
		return
	}

	sort.Slice(teams, func(i, j int) bool {
		return fmt.Sprintf("%s_%s", teams[i].DisplayName, teams[i].ID) < fmt.Sprintf("%s_%s", teams[j].DisplayName, teams[j].ID)
	})

	searchTerm := r.URL.Query().Get(QueryParamSearchTerm)
	offset, limit := a.p.GetOffsetAndLimitFromQueryParams(r.URL.Query())
	paginatedTeams := []msteams.Team{}
	matchCount := 0
	for _, team := range teams {
		if len(paginatedTeams) == limit {
			break
		}

		if strings.HasPrefix(strings.ToLower(team.DisplayName), strings.ToLower(searchTerm)) {
			matchCount++
			if matchCount > offset {
				paginatedTeams = append(paginatedTeams, team)
			}
		}
	}

	a.writeJSON(w, http.StatusOK, paginatedTeams)
}

func (a *API) getMSTeamsTeamChannels(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	teamID := r.URL.Query().Get(QueryParamTeamID)
	if teamID == "" {
		a.p.API.LogError("Error missing team ID query param.")
		http.Error(w, "Error missing team ID query param.", http.StatusBadRequest)
		return
	}

	channels, err := a.p.GetMSTeamsTeamChannels(teamID, userID)
	if err != nil {
		http.Error(w, "Error occurred while fetching the MS Teams team channels.", http.StatusInternalServerError)
		return
	}

	sort.Slice(channels, func(i, j int) bool {
		return channels[i].ID < channels[j].ID
	})

	offset, limit := a.p.GetOffsetAndLimitFromQueryParams(r.URL.Query())
	paginatedChannels := []msteams.Channel{}
	for index := offset; index < offset+limit && index < len(channels); index++ {
		channel := channels[index]
		paginatedChannels = append(paginatedChannels, channel)
	}

	a.writeJSON(w, http.StatusOK, paginatedChannels)
}

// TODO: Add unit tests
func (a *API) oauthRedirectHandler(w http.ResponseWriter, r *http.Request) {
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
	state := r.URL.Query().Get("state")

	stateArr := strings.Split(state, "_")
	if len(stateArr) != 2 {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	mmUserID := stateArr[1]
	if err := a.store.VerifyOAuth2State(state); err != nil {
		a.p.API.LogError("Unable to complete OAuth.", "UserID", mmUserID, "Error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	client := msteams.NewTokenClient(a.p.GetURL()+"/oauth-redirect", a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.configuration.ClientSecret, token, a.p.API.LogError)
	if err = client.Connect(); err != nil {
		a.p.API.LogError("Unable connect to the client", "error", err.Error())
		http.Error(w, "failed to connect to the client", http.StatusBadRequest)
		return
	}

	msteamsUser, err := client.GetMe()
	if err != nil {
		a.p.API.LogError("Unable to get the MS Teams user", "error", err.Error())
		http.Error(w, "failed to get the MS Teams user", http.StatusInternalServerError)
		return
	}

	mmUser, userErr := a.p.API.GetUser(mmUserID)
	if userErr != nil {
		a.p.API.LogError("Unable to get the MM user", "error", userErr.Error())
		http.Error(w, "failed to get the MM user", http.StatusInternalServerError)
		return
	}

	if mmUser.Id != a.p.GetBotUserID() && msteamsUser.Mail != mmUser.Email {
		a.p.API.LogError("Unable to connect users with different emails")
		http.Error(w, "cannot connect users with different emails", http.StatusBadRequest)
		return
	}

	storedToken, err := a.p.store.GetTokenForMSTeamsUser(msteamsUser.ID)
	if err != nil {
		a.p.API.LogError("Unable to get the token for MS Teams user", "Error", err.Error())
	}

	if storedToken != nil {
		a.p.API.LogError("This Teams user is already connected to another user on Mattermost.")
		http.Error(w, "This Teams user is already connected to another user on Mattermost.", http.StatusInternalServerError)
		return
	}

	err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, token)
	if err != nil {
		a.p.API.LogError("Unable to store the token", "error", err.Error())
		http.Error(w, "failed to store the token", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "text/html")
	_, _ = w.Write([]byte("<html><body><h1>Your account has been connected</h1><p>You can close this window.</p></body></html>"))
}

// handleAuthRequired verifies if the provided request is performed by an authorized source.
func (a *API) handleAuthRequired(handleFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mattermostUserID := r.Header.Get(HeaderMattermostUserID)
		if mattermostUserID == "" {
			a.p.API.LogError("Not authorized")
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		handleFunc(w, r)
	}
}

// checkUserConnected verifies if the user account is connected to MS Teams.
func (a *API) checkUserConnected(handleFunc http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mattermostUserID := r.Header.Get(HeaderMattermostUserID)
		if _, err := a.p.store.GetTokenForMattermostUser(mattermostUserID); err != nil {
			a.p.API.LogError("Unable to get the oauth token for the user.", "UserID", mattermostUserID, "Error", err.Error())
			http.Error(w, "The account is not connected.", http.StatusBadRequest)
			return
		}

		handleFunc(w, r)
	}
}

func (a *API) writeJSON(w http.ResponseWriter, statusCode int, v interface{}) {
	if statusCode == 0 {
		statusCode = http.StatusOK
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(v)
	if err != nil {
		a.p.API.LogError("Failed to marshal JSON response", "Error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err = w.Write(b); err != nil {
		a.p.API.LogError("Failed to write JSON response", "Error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(statusCode)
}
