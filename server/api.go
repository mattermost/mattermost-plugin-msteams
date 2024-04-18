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
	QueryParamPrimaryPlatform                 = "primary_platform"
	QueryParamChannelID                       = "channel_id"
	QueryParamPostID                          = "post_id"

	APIChoosePrimaryPlatform = "/choose-primary-platform"
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
	router.HandleFunc("/connect", api.connect).Methods("GET", "OPTIONS")
	router.HandleFunc("/oauth-redirect", api.oauthRedirectHandler).Methods("GET", "OPTIONS")
	router.HandleFunc("/connected-users", api.getConnectedUsers).Methods(http.MethodGet)
	router.HandleFunc("/connected-users/download", api.getConnectedUsersFile).Methods(http.MethodGet)
	router.HandleFunc("/whitelist", api.updateWhitelist).Methods(http.MethodPut)
	router.HandleFunc("/whitelist/download", api.getWhitelistEmailsFile).Methods(http.MethodGet)
	router.HandleFunc("/notify-connect", api.notifyConnect).Methods("GET")
	router.HandleFunc(APIChoosePrimaryPlatform, api.choosePrimaryPlatform).Methods(http.MethodGet)
	router.HandleFunc("/stats/site", api.siteStats).Methods("GET")
	router.HandleFunc("/primary-platform", api.primaryPlatform).Methods("GET")

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

	channelID := query.Get(QueryParamChannelID)
	postID := query.Get(QueryParamPostID)
	if channelID == "" || postID == "" {
		a.p.API.LogWarn("Missing channelID or postID from query paramaeters", "channelID", channelID, "postID", postID)
		http.Error(w, "Missing required query parameters.", http.StatusBadRequest)
	}

	if storedToken, _ := a.p.store.GetTokenForMattermostUser(userID); storedToken != nil {
		a.p.API.LogWarn("The account is already connected to MS Teams", "user_id", userID)
		http.Error(w, "Error in trying to connect the account, please try again.", http.StatusInternalServerError)
		return
	}

	state := fmt.Sprintf("%s_%s_%s_%s", model.NewId(), userID, postID, channelID)
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

func (a *API) primaryPlatform(w http.ResponseWriter, r *http.Request) {
	bundlePath, err := a.p.API.GetBundlePath()
	if err != nil {
		a.p.API.LogWarn("Failed to get bundle path.", "error", err.Error())
		return
	}

	t, err := template.ParseFiles(filepath.Join(bundlePath, "assets/info-page/index.html"))
	if err != nil {
		a.p.API.LogError("unable to parse the template", "error", err.Error())
		http.Error(w, "unable to view the primary platform selection page", http.StatusInternalServerError)
	}

	err = t.Execute(w, struct {
		ServerURL                 string
		APIEndPoint               string
		QueryParamPrimaryPlatform string
	}{
		ServerURL:                 a.p.GetURL(),
		APIEndPoint:               APIChoosePrimaryPlatform,
		QueryParamPrimaryPlatform: QueryParamPrimaryPlatform,
	})
	if err != nil {
		a.p.API.LogError("unable to execute the template", "error", err.Error())
		http.Error(w, "unable to view the primary platform selection page", http.StatusInternalServerError)
	}
}

func (a *API) notifyConnect(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")

	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if inviteWasSent, err := a.p.MaybeSendInviteMessage(userID); err != nil {
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
	if len(stateArr) != 4 {
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
		canOpenlyConnect, openConnectErr := a.p.UserCanOpenlyConnect(mmUserID)
		if openConnectErr != nil {
			a.p.API.LogWarn("Unable to check if user can openly connect", "error", openConnectErr.Error())
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}

		if !canOpenlyConnect {
			if err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, nil); err != nil {
				a.p.API.LogWarn("Unable to delete the OAuth token for user", "user_id", mmUserID, "error", err.Error())
			}
			http.Error(w, "You cannot connect your account because the maximum limit of users allowed to connect has been reached. Please contact your system administrator.", http.StatusBadRequest)
			return
		}
	}

	if err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, token); err != nil {
		a.p.API.LogWarn("Unable to store the token", "error", err.Error())
		http.Error(w, "failed to store the token", http.StatusInternalServerError)
		return
	}

	if err = a.p.store.DeleteUserInvite(mmUserID); err != nil {
		a.p.API.LogWarn("Unable to clear user invite", "user_id", mmUserID, "error", err.Error())
	}

	if err = a.p.store.DeleteUserFromWhitelist(mmUserID); err != nil {
		a.p.API.LogWarn("Unable to remove user from whitelist", "user_id", mmUserID, "error", err.Error())
	}

	w.Header().Add("Content-Type", "text/html")
	if mmUser.Id == a.p.GetBotUserID() {
		connectionMessage := "The bot account has been connected"
		_, _ = w.Write([]byte(fmt.Sprintf("<html><body><h1>%s</h1><p>You can close this window.</p></body></html>", connectionMessage)))
		return
	}

	_, _ = a.p.updateAutomutingOnUserConnect(mmUserID)

	const userConnectedMessage = "Welcome to Mattermost for Microsoft Teams! Your conversations with MS Teams users are now synchronized."
	post := &model.Post{
		Id:        stateArr[2],
		Message:   userConnectedMessage,
		ChannelId: stateArr[3],
		UserId:    a.p.GetBotUserID(),
		CreateAt:  model.GetMillis(),
	}

	_, appErr = a.p.GetAPI().GetPost(stateArr[2])
	if appErr == nil {
		_, appErr = a.p.GetAPI().UpdatePost(post)
		if appErr != nil {
			a.p.API.LogWarn("Unable to update post", "post", post.Id, "error", err.Error())
		}
	} else {
		_ = a.p.GetAPI().UpdateEphemeralPost(mmUser.Id, post)
	}

	connectURL := a.p.GetURL() + "/primary-platform"
	http.Redirect(w, r, connectURL, http.StatusSeeOther)
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

func (a *API) choosePrimaryPlatform(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		a.p.API.LogWarn("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	primaryPlatform := r.URL.Query().Get(QueryParamPrimaryPlatform)

	if primaryPlatform != PreferenceValuePlatformMM && primaryPlatform != PreferenceValuePlatformMSTeams {
		a.p.API.LogWarn("Invalid primary platform", "primary_platform", primaryPlatform)
		http.Error(w, "invalid primary platform", http.StatusBadRequest)
		return
	}

	err := a.p.setPrimaryPlatform(userID, primaryPlatform)
	if err != nil {
		a.p.API.LogWarn("Error when updating the preferences", "error", err.Error())
		http.Error(w, "error updating the preferences", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
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

	stats, err := a.p.store.GetStats()
	if err != nil {
		a.p.API.LogWarn("Failed to get site stats", "error", err.Error())
		http.Error(w, "unable to get site stats", http.StatusInternalServerError)
		return
	}

	siteStats := struct {
		TotalConnectedUsers int64 `json:"total_connected_users"`
	}{
		TotalConnectedUsers: stats.ConnectedUsers,
	}

	data, err := json.Marshal(siteStats)
	if err != nil {
		a.p.API.LogWarn("Failed to marshal site stats", "error", err.Error())
		http.Error(w, "unable to get site stats", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write(data)
}
