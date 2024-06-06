package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams/server/store"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/pkg/errors"
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

const (
	DefaultPage                               = 0
	MaxPerPage                                = 100
	UpdateWhitelistCsvParseErrThreshold       = 0
	UpdateWhitelistNotFoundEmailsErrThreshold = 10
	QueryParamPage                            = "page"
	QueryParamPerPage                         = "per_page"
	QueryParamChannelID                       = "channel_id"
	QueryParamPostID                          = "post_id"
	QueryParamFromPreferences                 = "from_preferences"
)

type UpdateWhitelistResult struct {
	Count       int      `json:"count"`
	Failed      []string `json:"failed"`
	FailedLines []string `json:"failedLines"`
}

func NewAPI(p *Plugin, store store.Store) *API {
	router := mux.NewRouter()
	p.handleStaticFiles(router)

	api := &API{p: p, router: router, store: store}

	if p.GetMetrics() != nil {
		// set error counter middleware handler
		router.Use(api.metricsMiddleware)
	}

	router.HandleFunc("/changes", api.processActivity).Methods("POST")
	router.HandleFunc("/lifecycle", api.processLifecycle).Methods("POST")
	router.HandleFunc("/autocomplete/teams", api.autocompleteTeams).Methods("GET")
	router.HandleFunc("/autocomplete/channels", api.autocompleteChannels).Methods("GET")
	router.HandleFunc("/connection-status", api.connectionStatus).Methods("GET")
	router.HandleFunc("/connect", api.connect).Methods("GET", "OPTIONS")
	router.HandleFunc("/oauth-redirect", api.oauthRedirectHandler).Methods("GET", "OPTIONS")
	router.HandleFunc("/connected-users", api.getConnectedUsers).Methods(http.MethodGet)
	router.HandleFunc("/connected-users/download", api.getConnectedUsersFile).Methods(http.MethodGet)
	router.HandleFunc("/whitelist", api.updateWhitelist).Methods(http.MethodPut)
	router.HandleFunc("/whitelist/download", api.getWhitelistEmailsFile).Methods(http.MethodGet)
	router.HandleFunc("/notify-connect", api.notifyConnect).Methods("GET")
	router.HandleFunc("/account-connected", api.accountConnectedPage).Methods(http.MethodGet)
	router.HandleFunc("/stats/site", api.siteStats).Methods("GET")
	router.HandleFunc("/enable-notifications", api.enableNotifications).Methods("POST")
	router.HandleFunc("/dismiss-notifications", api.dismissNotifications).Methods("POST")

	// iFrame support
	router.HandleFunc("/iframe/mattermostTab", api.iFrame).Methods("GET")
	router.HandleFunc("/iframe-manifest", api.iFrameManifest).Methods("GET")

	return api
}

func (a *API) decryptEncryptedContentData(key []byte, encryptedContent msteams.EncryptedContent) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedContent.Data)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to decode encrypted data")
	}
	msDataSignature, err := base64.StdEncoding.DecodeString(encryptedContent.DataSignature)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to decode data signature")
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(ciphertext)
	expectedMac := mac.Sum(nil)
	if !hmac.Equal(expectedMac, msDataSignature) {
		return nil, errors.New("The key signature doesn't match")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}
	iv := key[:16]

	if len(ciphertext)%block.BlockSize() != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	result := make([]byte, len(ciphertext))
	mode.CryptBlocks(result, ciphertext)
	resultPadding := int(result[len(result)-1])
	result = result[:len(result)-resultPadding]
	return result, nil
}

func (a *API) decryptEncryptedContentDataKey(encryptedContent msteams.EncryptedContent) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedContent.DataKey)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to decode key")
	}

	key, err := a.p.getPrivateKey()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to get private key")
	}
	hash := sha1.New() //nolint:gosec
	plaintext, err := rsa.DecryptOAEP(hash, nil, key, ciphertext, nil)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to decrypt data")
	}
	return plaintext, nil
}

func (a *API) processEncryptedContent(encryptedContent msteams.EncryptedContent) ([]byte, error) {
	msKey, err := a.decryptEncryptedContentDataKey(encryptedContent)
	if err != nil {
		a.p.API.LogWarn("Unable to decrypt key", "error", err.Error())
		return nil, err
	}

	data, err := a.decryptEncryptedContentData(msKey, encryptedContent)
	if err != nil {
		a.p.API.LogWarn("Unable to decrypt data", "error", err.Error())
		return nil, err
	}
	return data, nil
}

// processActivity handles the activity received from teams subscriptions
func (a *API) processActivity(w http.ResponseWriter, req *http.Request) {
	validationToken := req.URL.Query().Get("validationToken")
	if validationToken != "" {
		w.Header().Add("Content-Type", "text/plain")
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

	requireEncryptedContent := a.p.getConfiguration().CertificateKey != ""
	errors := ""
	for _, activity := range activities.Value {
		if activity.EncryptedContent != nil {
			content, err := a.processEncryptedContent(*activity.EncryptedContent)
			if err != nil {
				errors += err.Error() + "\n"
				continue
			}
			activity.Content = content
		} else if requireEncryptedContent {
			errors += "Not encrypted content for encrypted subscription"
			continue
		}

		if activity.ClientState != a.p.getConfiguration().WebhookSecret {
			errors += "Invalid webhook secret"
			continue
		}

		if err := a.p.activityHandler.Handle(activity); err != nil {
			a.p.API.LogWarn("Unable to process created activity", "activity", activity, "error", err.Error())
			errors += err.Error() + "\n"
		}
	}
	if errors != "" {
		http.Error(w, errors, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// processLifecycle handles the lifecycle events received from teams subscriptions
func (a *API) processLifecycle(w http.ResponseWriter, req *http.Request) {
	validationToken := req.URL.Query().Get("validationToken")
	if validationToken != "" {
		w.Header().Add("Content-Type", "text/plain")
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

	errors := ""
	for _, event := range lifecycleEvents.Value {
		// Check the webhook secret using ContantTimeCompare to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(event.ClientState), []byte(a.p.getConfiguration().WebhookSecret)) == 0 {
			a.p.metricsService.ObserveLifecycleEvent(event.LifecycleEvent, metrics.DiscardedReasonInvalidWebhookSecret)
			errors += "Invalid webhook secret"
			continue
		}
		a.p.activityHandler.HandleLifecycleEvent(event)
	}
	if errors != "" {
		http.Error(w, errors, http.StatusBadRequest)
		return
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
		a.p.API.LogWarn("Unable to get the client for user", "user_id", userID, "error", err.Error())
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	teams, err := client.ListTeams()
	if err != nil {
		a.p.API.LogWarn("Unable to get the MS Teams teams", "error", err.Error())
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
		a.p.API.LogWarn("Unable to get the client for user", "user_id", userID, "error", err.Error())
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	teamID := args[2]
	channels, err := client.ListChannels(teamID)
	if err != nil {
		a.p.API.LogWarn("Unable to get the channels for MS Teams team", "team_id", teamID, "error", err.Error())
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

func (a *API) connect(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}
	query := r.URL.Query()
	userID := r.Header.Get("Mattermost-User-ID")
	connectBot := query.Has("isBot")
	if connectBot {
		if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
			a.p.API.LogWarn("Attempt to connect the bot account, by non system admin.", "user_id", userID)
			http.Error(w, "Error in trying to connect the account, please try again.", http.StatusInternalServerError)
			return
		}
		userID = a.p.GetBotUserID()
	}

	stateSuffix := ""
	fromPreferences := query.Get(QueryParamFromPreferences)
	channelID := query.Get(QueryParamChannelID)
	postID := query.Get(QueryParamPostID)
	if fromPreferences == "true" {
		stateSuffix = "fromPreferences:true"
	} else if channelID != "" && postID != "" {
		stateSuffix = fmt.Sprintf("fromBotMessage:%s|%s", channelID, postID)
	} else {
		a.p.API.LogWarn("could not determine origin of the connect request", "channel_id", channelID, "post_id", postID, "from_preferences", fromPreferences)
		http.Error(w, "Missing required query parameters.", http.StatusBadRequest)
		return
	}

	if storedToken, _ := a.p.store.GetTokenForMattermostUser(userID); storedToken != nil {
		a.p.API.LogWarn("The account is already connected to MS Teams", "user_id", userID)
		http.Error(w, "Error in trying to connect the account, please try again.", http.StatusForbidden)
		return
	}

	state := fmt.Sprintf("%s_%s_%s", model.NewId(), userID, stateSuffix)
	if err := a.store.StoreOAuth2State(state); err != nil {
		a.p.API.LogWarn("Error in storing the OAuth state", "error", err.Error())
		http.Error(w, "Error in trying to connect the account, please try again.", http.StatusInternalServerError)
		return
	}

	codeVerifier := model.NewId()
	codeVerifierKey := "_code_verifier_" + userID
	if appErr := a.p.API.KVSet(codeVerifierKey, []byte(codeVerifier)); appErr != nil {
		a.p.API.LogWarn("Error in storing the code verifier", "error", appErr.Message)
		http.Error(w, "Error in trying to connect the account, please try again.", http.StatusInternalServerError)
		return
	}

	connectURL := msteams.GetAuthURL(a.p.GetURL()+"/oauth-redirect", a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.configuration.ClientSecret, state, codeVerifier)
	http.Redirect(w, r, connectURL, http.StatusSeeOther)
}

func (a *API) connectionStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	response := map[string]bool{"connected": false}
	if storedToken, _ := a.p.store.GetTokenForMattermostUser(userID); storedToken != nil {
		response["connected"] = true
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		a.p.API.LogWarn("Error while writing response", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func (a *API) accountConnectedPage(w http.ResponseWriter, r *http.Request) {
	message := "Your account is now connected to MS Teams."
	query := r.URL.Query()
	if query.Has("isBot") {
		message = "The bot account is now connected."
	}

	bundlePath, err := a.p.API.GetBundlePath()
	if err != nil {
		a.p.API.LogWarn("Failed to get bundle path.", "error", err.Error())
		return
	}

	t, err := template.ParseFiles(filepath.Join(bundlePath, "assets/account-connected/index.html"))
	if err != nil {
		a.p.API.LogError("unable to parse the template", "error", err.Error())
		http.Error(w, "unable to view the connected page", http.StatusInternalServerError)
	}

	err = t.Execute(w, struct {
		Message string
	}{
		Message: message,
	})
	if err != nil {
		a.p.API.LogError("unable to execute the template", "error", err.Error())
		http.Error(w, "unable to view the connected page", http.StatusInternalServerError)
	}
}

func (a *API) notifyConnect(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	now := time.Now()

	if inviteWasSent, err := a.p.MaybeSendInviteMessage(userID, now); err != nil {
		a.p.API.LogWarn("Error in connection invite flow", "user_id", userID, "error", err.Error())
	} else if inviteWasSent {
		a.p.API.LogInfo("Successfully sent connection invite", "user_id", userID)
	}
}

func (a *API) oauthRedirectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		return
	}

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
	if len(stateArr) != 3 {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	// determine origin of the connect request
	// if the state is fromPreferences, the user is connecting from the preferences page
	// if the state is fromBotMessage, the user is connecting from a bot message
	originInfo := strings.Split(stateArr[2], ":")
	if len(originInfo) != 2 {
		a.p.API.LogWarn("unable to get origin info from state", "state", stateArr[2])
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	channelID := ""
	postID := ""
	switch originInfo[0] {
	case "fromPreferences":
		// do nothing
	case "fromBotMessage":
		fromBotMessageArgs := strings.Split(originInfo[1], "|")
		if len(fromBotMessageArgs) != 2 {
			a.p.API.LogWarn("unable to get args from bot message", "origin_info", originInfo[1])
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}
		channelID = fromBotMessageArgs[0]
		postID = fromBotMessageArgs[1]
	default:
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	mmUserID := stateArr[1]
	if err := a.store.VerifyOAuth2State(state); err != nil {
		a.p.API.LogWarn("Unable to verify OAuth state", "user_id", mmUserID, "error", err.Error())
		http.Error(w, "Unable to complete authentication.", http.StatusInternalServerError)
		return
	}

	codeVerifierBytes, appErr := a.p.API.KVGet("_code_verifier_" + mmUserID)
	if appErr != nil {
		a.p.API.LogWarn("Unable to get the code verifier", "error", appErr.Error())
		http.Error(w, "failed to get the code verifier", http.StatusBadRequest)
		return
	}
	appErr = a.p.API.KVDelete("_code_verifier_" + mmUserID)
	if appErr != nil {
		a.p.API.LogWarn("Unable to delete the used code verifier", "error", appErr.Error())
	}

	ctx := context.Background()
	token, err := conf.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", string(codeVerifierBytes)))
	if err != nil {
		a.p.API.LogWarn("Unable to get OAuth2 token", "error", err.Error())
		http.Error(w, "Unable to complete authentication", http.StatusInternalServerError)
		return
	}

	client := msteams.NewTokenClient(a.p.GetURL()+"/oauth-redirect", a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.configuration.ClientSecret, token, &a.p.apiClient.Log)
	if err = client.Connect(); err != nil {
		a.p.API.LogWarn("Unable to connect to the client", "error", err.Error())
		http.Error(w, "failed to connect to the client", http.StatusInternalServerError)
		return
	}

	msteamsUser, err := client.GetMe()
	if err != nil {
		a.p.API.LogWarn("Unable to get the MS Teams user", "error", err.Error())
		http.Error(w, "failed to get the MS Teams user", http.StatusInternalServerError)
		return
	}

	mmUser, userErr := a.p.API.GetUser(mmUserID)
	if userErr != nil {
		a.p.API.LogWarn("Unable to get the MM user", "error", userErr.Error())
		http.Error(w, "failed to get the MM user", http.StatusInternalServerError)
		return
	}

	if mmUser.Id != a.p.GetBotUserID() && msteamsUser.Mail != mmUser.Email {
		a.p.API.LogWarn("Unable to connect users with different emails")
		http.Error(w, "cannot connect users with different emails", http.StatusBadRequest)
		return
	}

	storedToken, err := a.p.store.GetTokenForMSTeamsUser(msteamsUser.ID)
	if err != nil {
		a.p.API.LogWarn("Unable to get the token for MS Teams user", "error", err.Error())
	}

	if storedToken != nil {
		a.p.API.LogWarn("This Teams user is already connected to another user on Mattermost.", "teams_user_id", msteamsUser.ID)
		http.Error(w, "This Teams user is already connected to another user on Mattermost.", http.StatusBadRequest)
		return
	}

	a.p.connectClusterMutex.Lock()
	defer a.p.connectClusterMutex.Unlock()

	hasRightToConnect, err := a.p.UserHasRightToConnect(mmUserID)
	if err != nil {
		a.p.API.LogWarn("Unable to check if user has the right to connect", "error", err.Error())
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	if !hasRightToConnect {
		canOpenlyConnect, nAvailable, openConnectErr := a.p.UserCanOpenlyConnect(mmUserID)
		if openConnectErr != nil {
			a.p.API.LogWarn("Unable to check if user can openly connect", "error", openConnectErr.Error())
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}

		if !canOpenlyConnect {
			if err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, nil); err != nil {
				a.p.API.LogWarn("Unable to delete the OAuth token for user", "user_id", mmUserID, "error", err.Error())
			}

			if nAvailable > 0 {
				http.Error(w, "You cannot connect your account at this time because an invitation is required. Please contact your system administrator to request an invitation.", http.StatusBadRequest)
			} else {
				http.Error(w, "You cannot connect your account because the maximum limit of users allowed to connect has been reached. Please contact your system administrator.", http.StatusBadRequest)
			}
			return
		}
	}

	if err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, token); err != nil {
		a.p.API.LogWarn("Unable to store the token", "error", err.Error())
		http.Error(w, "failed to store the token", http.StatusInternalServerError)
		return
	}

	a.p.API.PublishWebSocketEvent(WSEventUserConnected, map[string]any{}, &model.WebsocketBroadcast{
		UserId: mmUserID,
	})

	if err = a.p.store.DeleteUserInvite(mmUserID); err != nil {
		a.p.API.LogWarn("Unable to clear user invite", "user_id", mmUserID, "error", err.Error())
	}

	if err = a.p.store.DeleteUserFromWhitelist(mmUserID); err != nil {
		a.p.API.LogWarn("Unable to remove user from whitelist", "user_id", mmUserID, "error", err.Error())
	}

	w.Header().Add("Content-Type", "text/html")
	if mmUser.Id == a.p.GetBotUserID() {
		http.Redirect(w, r, a.p.GetURL()+"/account-connected?isBot", http.StatusSeeOther)
		return
	}

	switch {
	case a.p.getConfiguration().SyncNotifications:
		a.handleSyncNotificationsWelcomeMessage(originInfo[0], mmUserID, channelID, postID)
	default:
		a.handleDefaultWelcomeMessage(originInfo[0], mmUserID, postID, channelID)
	}

	http.Redirect(w, r, a.p.GetURL()+"/account-connected", http.StatusSeeOther)
}

func (a *API) handleDefaultWelcomeMessage(originInfo, mmUserID, postID, channelID string) {
	const userConnectedMessage = "Welcome to Mattermost for Microsoft Teams! Your conversations with MS Teams users are now synchronized."
	switch originInfo {
	case "fromBotMessage":
		post := &model.Post{
			Id:        postID,
			Message:   userConnectedMessage,
			ChannelId: channelID,
			UserId:    a.p.GetBotUserID(),
			CreateAt:  model.GetMillis(),
		}

		_, appErr := a.p.GetAPI().GetPost(post.Id)
		if appErr == nil {
			_, appErr = a.p.GetAPI().UpdatePost(post)
			if appErr != nil {
				a.p.API.LogWarn("Unable to update post", "post", post.Id, "error", appErr.Error())
			}
		} else {
			_ = a.p.GetAPI().UpdateEphemeralPost(mmUserID, post)
		}
	case "fromPreferences":
		err := a.p.botSendDirectMessage(mmUserID, userConnectedMessage)
		if err != nil {
			a.p.API.LogWarn("Unable to send welcome direct message to user from preference", "error", err.Error())
		}
	}
}

func (a *API) handleSyncNotificationsWelcomeMessage(originInfo string, mmUserID, channelID, postID string) {
	switch originInfo {
	case "fromPreferences":
		err := a.p.SendWelcomeMessageWithNotificationAction(mmUserID)
		if err != nil {
			a.p.API.LogWarn("Unable to send welcome post with notifications", "error", err.Error())
		}
	case "fromBotMessage":
		welcomePost := a.p.makeWelcomeMessageWithNotificationActionPost()
		var originalPost *model.Post
		originalPost, appErr := a.p.GetAPI().GetPost(postID)
		if appErr == nil {
			originalPost.Message = welcomePost.Message
			originalPost.SetProps(welcomePost.Props)
			_, appErr = a.p.GetAPI().UpdatePost(originalPost)
			if appErr != nil {
				a.p.API.LogWarn("Unable to update post", "post", postID, "error", appErr.Error())
			}
		} else {
			// Update the original connection prompt and remove any calls to action.
			// This occurs where the ephemeral message was sent, and likely where the
			// user returned to after closing the oAuth2 flow.
			a.p.GetAPI().UpdateEphemeralPost(mmUserID, &model.Post{
				Id:        postID,
				ChannelId: channelID,
				UserId:    a.p.GetBotUserID(),
				Message:   "Your account is now connected to MS Teams.",
				CreateAt:  model.GetMillis(),
				UpdateAt:  model.GetMillis(),
			})

			// Send the welcome message in the bot channel.
			err := a.p.SendWelcomeMessageWithNotificationAction(mmUserID)
			if err != nil {
				a.p.API.LogWarn("Unable to send welcome post with notifications", "error", err.Error())
			}
		}
	}
}

func (a *API) getConnectedUsers(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogWarn("Insufficient permissions", "user_id", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	page, perPage := GetPageAndPerPage(r)
	connectedUsersList, err := a.p.store.GetConnectedUsers(page, perPage)
	if err != nil {
		a.p.API.LogWarn("Unable to get connected users list", "error", err.Error())
		http.Error(w, "unable to get connected users list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(connectedUsersList)
	if err != nil {
		a.p.API.LogWarn("Failed to marshal JSON response", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("[]"))
		return
	}

	if _, err = w.Write(b); err != nil {
		a.p.API.LogWarn("Error while writing response", "error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) getConnectedUsersFile(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogWarn("Insufficient permissions", "user_id", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	connectedUsersList, err := a.p.getConnectedUsersList()
	if err != nil {
		a.p.API.LogWarn("Unable to get connected users list", "error", err.Error())
		http.Error(w, "unable to get connected users list", http.StatusInternalServerError)
		return
	}

	b := &bytes.Buffer{}
	csvWriter := csv.NewWriter(b)
	if err := csvWriter.Write([]string{"First Name", "Last Name", "Email", "Mattermost User Id", "Teams User Id"}); err != nil {
		a.p.API.LogWarn("Unable to write headers in CSV file", "error", err.Error())
		http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
		return
	}

	for _, connectedUser := range connectedUsersList {
		if err := csvWriter.Write([]string{connectedUser.FirstName, connectedUser.LastName, connectedUser.Email, connectedUser.MattermostUserID, connectedUser.TeamsUserID}); err != nil {
			a.p.API.LogWarn("Unable to write data in CSV file", "error", err.Error())
			http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
			return
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		a.p.API.LogWarn("Unable to flush the data in writer", "error", err.Error())
		http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=connected-users.csv")
	if _, err := w.Write(b.Bytes()); err != nil {
		a.p.API.LogWarn("Unable to write the data", "error", err.Error())
		http.Error(w, "unable to write the data", http.StatusInternalServerError)
	}
}

func (a *API) getWhitelistEmailsFile(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogWarn("Insufficient permissions", "user_id", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	whitelist, err := a.p.getWhitelistEmails()
	if err != nil {
		a.p.API.LogWarn("Unable to get whitelist", "error", err.Error())
		http.Error(w, "unable to get whitelist", http.StatusInternalServerError)
		return
	}

	b := &bytes.Buffer{}
	csvWriter := csv.NewWriter(b)
	if err := csvWriter.Write([]string{"Email"}); err != nil {
		a.p.API.LogWarn("Unable to write headers in CSV file", "error", err.Error())
		http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
		return
	}

	for _, email := range whitelist {
		if err := csvWriter.Write([]string{email}); err != nil {
			a.p.API.LogWarn("Unable to write data in CSV file", "error", err.Error())
			http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
			return
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		a.p.API.LogWarn("Unable to flush the data in writer", "error", err.Error())
		http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=invite-whitelist.csv")
	if _, err := w.Write(b.Bytes()); err != nil {
		a.p.API.LogWarn("Unable to write the data", "error", err.Error())
		http.Error(w, "unable to write the data", http.StatusInternalServerError)
	}
}

func (a *API) updateWhitelist(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogWarn("Insufficient permissions", "user_id", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		a.p.API.LogWarn("Error reading whitelist file")
		http.Error(w, "error reading whitelist", http.StatusBadRequest)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	columns, err := reader.Read()
	if err != nil || strings.ToLower(columns[0]) != "email" || len(columns) != 1 {
		a.p.API.LogWarn("Error parsing whitelist csv header")
		http.Error(w, "error parsing whitelist - please check header and try again", http.StatusBadRequest)
		return
	}

	var ids []string
	var failed []string

	var csvLineErrs []string
	var i = 1 // offset, start line 1
	for {
		i++
		row, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			csvLineErrs = append(csvLineErrs, strconv.Itoa(i))
			continue
		}
		if len(csvLineErrs) > UpdateWhitelistCsvParseErrThreshold {
			break
		}
		email := row[0]
		user, err := a.p.API.GetUserByEmail(email)
		if err != nil {
			a.p.API.LogWarn("Error could not find user with email", "line", i)
			failed = append(failed, email)
			continue
		}

		ids = append(ids, user.Id)
	}

	if len(csvLineErrs) > UpdateWhitelistCsvParseErrThreshold {
		a.p.API.LogWarn("Error parsing whitelist csv data", "lines", csvLineErrs)
		http.Error(w, "error parsing whitelist - please check data at line(s) "+strings.Join(csvLineErrs, ", ")+" and try again", http.StatusBadRequest)
		return
	}

	if len(failed) > UpdateWhitelistNotFoundEmailsErrThreshold {
		a.p.API.LogWarn("Error: too many users not found", "threshold", UpdateWhitelistNotFoundEmailsErrThreshold, "failed", len(failed))
		http.Error(w, "error - could not find user(s): "+strings.Join(failed, ", "), http.StatusInternalServerError)
		return
	}

	if err := a.p.store.SetWhitelist(ids, MaxPerPage); err != nil {
		a.p.API.LogWarn("Error processing whitelist", "error", err.Error())
		http.Error(w, "error processing whitelist - please check data and try again", http.StatusInternalServerError)
		return
	}

	a.p.API.LogInfo("Whitelist updated", "size", len(ids))

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(&UpdateWhitelistResult{
		Count:       len(ids),
		Failed:      failed,
		FailedLines: csvLineErrs,
	}); err != nil {
		a.p.API.LogWarn("Error writing update whitelist response")
	}
}

func (p *Plugin) getConnectedUsersList() ([]*storemodels.ConnectedUser, error) {
	page := DefaultPage
	perPage := MaxPerPage
	var connectedUserList []*storemodels.ConnectedUser
	for {
		connectedUsers, err := p.store.GetConnectedUsers(page, perPage)
		if err != nil {
			return nil, err
		}

		connectedUserList = append(connectedUserList, connectedUsers...)
		if len(connectedUsers) < perPage {
			break
		}

		page++
	}

	return connectedUserList, nil
}

func (p *Plugin) getWhitelistEmails() ([]string, error) {
	page := DefaultPage
	perPage := MaxPerPage
	var result []string
	for {
		emails, err := p.store.GetWhitelistEmails(page, perPage)
		if err != nil {
			return nil, err
		}

		result = append(result, emails...)
		if len(emails) < perPage {
			break
		}

		page++
	}

	return result, nil
}

// handleStaticFiles handles the static files under the assets directory.
func (p *Plugin) handleStaticFiles(r *mux.Router) {
	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		p.API.LogWarn("Failed to get bundle path.", "error", err.Error())
		return
	}

	// This will serve static files from the 'assets' directory under '/static/<filename>'
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(filepath.Join(bundlePath, "assets")))))
}

func GetPageAndPerPage(r *http.Request) (page, perPage int) {
	query := r.URL.Query()
	if val, err := strconv.Atoi(query.Get(QueryParamPage)); err != nil || val < 0 {
		page = DefaultPage
	} else {
		page = val
	}

	val, err := strconv.Atoi(query.Get(QueryParamPerPage))
	if err != nil || val < 0 || val > MaxPerPage {
		perPage = MaxPerPage
	} else {
		perPage = val
	}

	return page, perPage
}

func (a *API) siteStats(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogWarn("Insufficient permissions", "user_id", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	connectedUsersCount, err := a.p.store.GetConnectedUsersCount()
	if err != nil {
		a.p.API.LogWarn("Failed to get connected users count", "error", err.Error())
		http.Error(w, "unable to get connected users count", http.StatusInternalServerError)
		return
	}
	pendingInvites, err := a.p.store.GetInvitedCount()
	if err != nil {
		a.p.API.LogWarn("Failed to get invited users count", "error", err.Error())
		http.Error(w, "unable to get invited users count", http.StatusInternalServerError)
		return
	}
	whitelistedUsers, err := a.p.store.GetWhitelistCount()
	if err != nil {
		a.p.API.LogWarn("Failed to get whitelisted users count", "error", err.Error())
		http.Error(w, "unable to get whitelisted users count", http.StatusInternalServerError)
		return
	}
	receiving, err := a.p.store.GetActiveUsersReceivingCount(metricsActiveUsersRange)
	if err != nil {
		a.p.API.LogWarn("Failed to get users receiving count", "error", err.Error())
		http.Error(w, "unable to get users receiving count", http.StatusInternalServerError)
		return
	}
	sending, err := a.p.store.GetActiveUsersSendingCount(metricsActiveUsersRange)
	if err != nil {
		a.p.API.LogWarn("Failed to get sending users count", "error", err.Error())
		http.Error(w, "unable to get sending users count", http.StatusInternalServerError)
		return
	}

	siteStats := struct {
		TotalConnectedUsers   int64 `json:"total_connected_users"`
		PendingInvitedUsers   int64 `json:"pending_invited_users"`
		CurrentWhitelistUsers int64 `json:"current_whitelist_users"`
		UserReceivingMessages int64 `json:"total_users_receiving"`
		UserSendingMessages   int64 `json:"total_users_sending"`
	}{
		TotalConnectedUsers:   connectedUsersCount,
		PendingInvitedUsers:   int64(pendingInvites),
		CurrentWhitelistUsers: int64(whitelistedUsers),
		UserReceivingMessages: receiving,
		UserSendingMessages:   sending,
	}

	data, err := json.Marshal(siteStats)
	if err != nil {
		a.p.API.LogWarn("Failed to marshal site stats", "error", err.Error())
		http.Error(w, "unable to get site stats", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(data)
}

func (a *API) enableNotifications(w http.ResponseWriter, r *http.Request) {
	var actionHandler model.PostActionIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&actionHandler); err != nil {
		a.p.API.LogWarn("Unable to decode the action handler", "error", err.Error())
		http.Error(w, "unable to decode the action handler", http.StatusBadRequest)
		return
	}

	err := a.p.setNotificationPreference(actionHandler.UserId, true)
	if err != nil {
		a.p.API.LogWarn("Error when updating the preferences", "error", err.Error())
		http.Error(w, "error updating the preferences", http.StatusInternalServerError)
		return
	}

	post, err := a.p.apiClient.Post.GetPost(actionHandler.PostId)
	if err != nil {
		a.p.API.LogWarn("Unable to get the post", "error", err.Error())
		http.Error(w, "unable to get the post", http.StatusBadRequest)
		return
	}

	// Verify the post was authored by the bot itself.
	if post.UserId != a.p.botUserID {
		a.p.API.LogWarn("Attempt to update post not authored by the bot", "user_id", userID)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Verify the post is in the direct message channel between the bot and the user.
	botDMChannel, err := a.p.apiClient.Channel.GetDirect(a.p.botUserID, userID)
	if err != nil {
		a.p.API.LogWarn("Unable to get the bot direct channel", "user_id", userID, "error", err.Error())
		http.Error(w, "failed to authenticate the request", http.StatusInternalServerError)
		return
	} else if botDMChannel.Id != post.ChannelId {
		a.p.API.LogWarn("Unable to get the bot direct channel", "user_id", userID)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// At this point, it might be /another/ post by the bot in the DM with that user, but we
	// allow this for now.

	post.Message = "You will now start receiving notifications from chats or group chats in Teams. To change this setting, open your user settings or run `/msteams notifications`"
	post.DelProp("attachments")

	err = json.NewEncoder(w).Encode(model.PostActionIntegrationResponse{
		Update: post,
	})

	if err != nil {
		a.p.API.LogWarn("Unable to encode the response", "error", err.Error())
		http.Error(w, "unable to encode the response", http.StatusInternalServerError)
	}
}

func (a *API) dismissNotifications(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	var actionHandler model.PostActionIntegrationRequest
	if err := json.NewDecoder(r.Body).Decode(&actionHandler); err != nil {
		a.p.API.LogWarn("Unable to decode the action handler", "user_id", userID, "error", err.Error())
		http.Error(w, "unable to decode the action handler", http.StatusBadRequest)
		return
	}

	post, err := a.p.apiClient.Post.GetPost(actionHandler.PostId)
	if err != nil {
		a.p.API.LogWarn("Unable to get the post", "user_id", userID, "error", err.Error())
		http.Error(w, "unable to get the post", http.StatusBadRequest)
		return
	}

	// Verify the post was authored by the bot itself.
	if post.UserId != a.p.botUserID {
		a.p.API.LogWarn("Attempt to update post not authored by the bot", "user_id", userID)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Verify the post is in the direct message channel between the bot and the user.
	botDMChannel, err := a.p.apiClient.Channel.GetDirect(a.p.botUserID, userID)
	if err != nil {
		a.p.API.LogWarn("Unable to get the bot direct channel", "user_id", userID, "error", err.Error())
		http.Error(w, "failed to authenticate the request", http.StatusInternalServerError)
		return
	} else if botDMChannel.Id != post.ChannelId {
		a.p.API.LogWarn("Unable to get the bot direct channel", "user_id", userID)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// At this point, it might be /another/ post by the bot in the DM with that user, but we
	// allow this for now.

	post.Message = "To change your notification settings, open your user settings or run `/msteams notifications`"
	post.DelProp("attachments")

	err = json.NewEncoder(w).Encode(model.PostActionIntegrationResponse{
		Update: post,
	})
	if err != nil {
		a.p.API.LogWarn("Unable to encode the response", "error", err.Error())
		http.Error(w, "unable to encode the response", http.StatusInternalServerError)
	}
}
