package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/constants"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"

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
	DefaultPage               = 0
	MaxPerPage                = 100
	QueryParamPage            = "page"
	QueryParamPerPage         = "per_page"
	QueryParamPrimaryPlatform = "primary_platform"

	APIChoosePrimaryPlatform = "/choose-primary-platform"
)

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
	router.HandleFunc("/needsConnect", api.needsConnect).Methods("GET", "OPTIONS")
	router.HandleFunc("/connect", api.connect).Methods("GET", "OPTIONS")
	router.HandleFunc("/oauth-redirect", api.oauthRedirectHandler).Methods("GET", "OPTIONS")
	router.HandleFunc("/connected-users", api.getConnectedUsers).Methods(http.MethodGet)
	router.HandleFunc("/connected-users/download", api.getConnectedUsersFile).Methods(http.MethodGet)
	router.HandleFunc(APIChoosePrimaryPlatform, api.choosePrimaryPlatform).Methods(http.MethodGet)

	// iFrame support
	router.HandleFunc("/iframe/mattermostTab", api.iFrame).Methods("GET")
	router.HandleFunc("/iframe-manifest", api.iFrameManifest).Methods("GET")

	api.registerClientMock()

	return api
}

func (a *API) decryptEncryptedContentData(key []byte, encryptedContent msteams.EncryptedContent) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedContent.Data)
	if err != nil {
		a.p.API.LogDebug("Unable to decode encrypted data", "error", err)
		return nil, err
	}
	msDataSignature, err := base64.StdEncoding.DecodeString(encryptedContent.DataSignature)
	if err != nil {
		a.p.API.LogDebug("Unable to decode data signature", "error", err)
		return nil, err
	}

	mac := hmac.New(sha256.New, key)
	mac.Write(ciphertext)
	expectedMac := mac.Sum(nil)
	if !hmac.Equal(expectedMac, msDataSignature) {
		a.p.API.LogDebug("Invalid data signature", "error", errors.New("The key signature doesn't match"))
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
		a.p.API.LogDebug("Unable to decode key", "error", err)
		return nil, err
	}

	key, err := a.p.getPrivateKey()
	if err != nil {
		a.p.API.LogDebug("Unable to get private key", "error", err)
		return nil, err
	}
	hash := sha1.New() //nolint:gosec
	plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, key, ciphertext, nil)
	if err != nil {
		a.p.API.LogDebug("Unable to decrypt data", "error", err, "cipheredText", string(ciphertext))
		return nil, err
	}
	return plaintext, nil
}

func (a *API) processEncryptedContent(encryptedContent msteams.EncryptedContent) ([]byte, error) {
	msKey, err := a.decryptEncryptedContentDataKey(encryptedContent)
	if err != nil {
		a.p.API.LogDebug("Unable to decrypt key", "error", err)
		return nil, err
	}

	data, err := a.decryptEncryptedContentData(msKey, encryptedContent)
	if err != nil {
		a.p.API.LogDebug("Unable to decrypt data", "error", err)
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

	a.p.API.LogDebug("Lifecycle activity request", "activities", lifecycleEvents)

	errors := ""
	for _, event := range lifecycleEvents.Value {
		if event.ClientState != a.p.getConfiguration().WebhookSecret {
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
		a.p.API.LogError("Unable to get the client for user", "MMUserID", userID, "Error", err.Error())
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	teams, err := client.ListTeams()
	if err != nil {
		a.p.API.LogError("Unable to get the MS Teams teams", "Error", err.Error())
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
	userID := r.Header.Get("Mattermost-User-ID")
	args := strings.Fields(r.URL.Query().Get("parsed"))
	if len(args) < 3 {
		data, _ := json.Marshal(out)
		_, _ = w.Write(data)
		return
	}

	client, err := a.p.GetClientForUser(userID)
	if err != nil {
		a.p.API.LogError("Unable to get the client for user", "MMUserID", userID, "Error", err.Error())
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
	if r.Method == http.MethodOptions {
		return
	}

	response := map[string]bool{
		"canSkip":      a.p.getConfiguration().AllowSkipConnectUsers,
		"needsConnect": false,
	}

	if a.p.getConfiguration().EnforceConnectedUsers {
		userID := r.Header.Get("Mattermost-User-ID")
		client, _ := a.p.GetClientForUser(userID)
		if client == nil {
			if a.p.getConfiguration().EnabledTeams == "" {
				response["needsConnect"] = true
			} else {
				enabledTeams := strings.Split(a.p.getConfiguration().EnabledTeams, ",")

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

	state := fmt.Sprintf("%s_%s", model.NewId(), userID)
	if err := a.store.StoreOAuth2State(state); err != nil {
		a.p.API.LogError("Error in storing the OAuth state", "error", err.Error())
		http.Error(w, "Error in trying to connect the account, please try again.", http.StatusInternalServerError)
		return
	}

	codeVerifier := model.NewId()
	if appErr := a.p.API.KVSet("_code_verifier_"+userID, []byte(codeVerifier)); appErr != nil {
		a.p.API.LogError("Error in storing the code verifier", "error", appErr.Message)
		http.Error(w, "Error in trying to connect the account, please try again.", http.StatusInternalServerError)
		return
	}

	connectURL := msteams.GetAuthURL(a.p.GetURL()+"/oauth-redirect", a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.configuration.ClientSecret, state, codeVerifier)

	data, _ := json.Marshal(map[string]string{"connectUrl": connectURL})
	_, _ = w.Write(data)
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
	if len(stateArr) != 2 {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	mmUserID := stateArr[1]
	if err := a.store.VerifyOAuth2State(state); err != nil {
		a.p.API.LogError("Unable to verify OAuth state", "MMUserID", mmUserID, "Error", err.Error())
		http.Error(w, "Unable to complete authentication.", http.StatusInternalServerError)
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
		a.p.API.LogError("Unable to get OAuth2 token", "error", err.Error())
		http.Error(w, "Unable to complete authentication", http.StatusInternalServerError)
		return
	}

	client := msteams.NewTokenClient(a.p.GetURL()+"/oauth-redirect", a.p.configuration.TenantID, a.p.configuration.ClientID, a.p.configuration.ClientSecret, token, &a.p.apiClient.Log)
	if err = client.Connect(); err != nil {
		a.p.API.LogError("Unable to connect to the client", "error", err.Error())
		http.Error(w, "failed to connect to the client", http.StatusInternalServerError)
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
		a.p.API.LogDebug("Unable to get the token for MS Teams user", "Error", err.Error())
	}

	if storedToken != nil {
		a.p.API.LogError("This Teams user is already connected to another user on Mattermost.", "MSTeamsUserID", msteamsUser.ID)
		http.Error(w, "This Teams user is already connected to another user on Mattermost.", http.StatusBadRequest)
		return
	}

	if err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, token); err != nil {
		a.p.API.LogError("Unable to store the token", "error", err.Error())
		http.Error(w, "failed to store the token", http.StatusInternalServerError)
		return
	}

	a.p.whitelistClusterMutex.Lock()
	defer a.p.whitelistClusterMutex.Unlock()
	whitelistSize, err := a.p.store.GetSizeOfWhitelist()
	if err != nil {
		a.p.API.LogError("Unable to get whitelist size", "error", err.Error())
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	if whitelistSize >= a.p.getConfiguration().ConnectedUsersAllowed {
		if err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, nil); err != nil {
			a.p.API.LogError("Unable to delete the OAuth token for user", "UserID", mmUserID, "Error", err.Error())
		}
		http.Error(w, "You cannot connect your account because the maximum limit of users allowed to connect has been reached. Please contact your system administrator.", http.StatusBadRequest)
		return
	}

	if err := a.p.store.StoreUserInWhitelist(mmUserID); err != nil {
		a.p.API.LogError("Unable to store the user in whitelist", "UserID", mmUserID, "Error", err.Error())
		if err = a.p.store.SetUserInfo(mmUserID, msteamsUser.ID, nil); err != nil {
			a.p.API.LogError("Unable to delete the OAuth token for user", "UserID", mmUserID, "Error", err.Error())
		}

		http.Error(w, "Something went wrong.", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "text/html")
	connectionMessage := "Your account has been connected"
	if mmUser.Id == a.p.GetBotUserID() {
		connectionMessage = "The bot account has been connected"
	}

	_, _ = w.Write([]byte(fmt.Sprintf("<html><body><h1>%s</h1><p>You can close this window.</p></body></html>", connectionMessage)))

	// TODO: Remove the comment after the completion of other related tasks.
	// bundlePath, err := a.p.API.GetBundlePath()
	// if err != nil {
	// 	a.p.API.LogWarn("Failed to get bundle path.", "Error", err.Error())
	// 	return
	// }

	// t, err := template.ParseFiles(filepath.Join(bundlePath, "assets/info-page/index.html"))
	// if err != nil {
	// 	a.p.API.LogError("unable to parse the template", "Error", err.Error())
	// 	http.Error(w, "unable to view the primary platform selection page", http.StatusInternalServerError)
	// }

	// err = t.Execute(w, struct {
	// 	ServerURL string
	// 	APIEndPoint string
	// 	QueryParamPrimaryPlatform string
	// } {
	// 	ServerURL: a.p.GetURL(),
	// 	APIEndPoint: APIChoosePrimaryPlatform,
	// 	QueryParamPrimaryPlatform: QueryParamPrimaryPlatform,
	// })
	// if err != nil {
	// 	a.p.API.LogError("unable to execute the template", "Error", err.Error())
	// 	http.Error(w, "unable to view the primary platform selection page", http.StatusInternalServerError)
	// }
}

func (a *API) getConnectedUsers(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		a.p.API.LogError("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogError("Insufficient permissions", "UserID", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	page, perPage := GetPageAndPerPage(r)
	connectedUsersList, err := a.p.store.GetConnectedUsers(page, perPage)
	if err != nil {
		a.p.API.LogError("Unable to get connected users list", "Error", err.Error())
		http.Error(w, "unable to get connected users list", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	b, err := json.Marshal(connectedUsersList)
	if err != nil {
		a.p.API.LogError("Failed to marshal JSON response", "Error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("[]"))
		return
	}

	if _, err = w.Write(b); err != nil {
		a.p.API.LogError("Error while writing response", "Error", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (a *API) getConnectedUsersFile(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		a.p.API.LogError("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	if !a.p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		a.p.API.LogError("Insufficient permissions", "UserID", userID)
		http.Error(w, "not able to authorize the user", http.StatusForbidden)
		return
	}

	connectedUsersList, err := a.p.getConnectedUsersList()
	if err != nil {
		a.p.API.LogError("Unable to get connected users list", "Error", err.Error())
		http.Error(w, "unable to get connected users list", http.StatusInternalServerError)
		return
	}

	b := &bytes.Buffer{}
	csvWriter := csv.NewWriter(b)
	if err := csvWriter.Write([]string{"First Name", "Last Name", "Email", "Mattermost User Id", "Teams User Id"}); err != nil {
		a.p.API.LogError("Unable to write headers in CSV file", "Error", err.Error())
		http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
		return
	}

	for _, connectedUser := range connectedUsersList {
		if err := csvWriter.Write([]string{connectedUser.FirstName, connectedUser.LastName, connectedUser.Email, connectedUser.MattermostUserID, connectedUser.TeamsUserID}); err != nil {
			a.p.API.LogError("Unable to write data in CSV file", "Error", err.Error())
			http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
			return
		}
	}

	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		a.p.API.LogError("Unable to flush the data in writer", "Error", err.Error())
		http.Error(w, "unable to write data in CSV file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename=connected-users.csv")
	if _, err := w.Write(b.Bytes()); err != nil {
		a.p.API.LogError("Unable to write the data", "Error", err.Error())
		http.Error(w, "unable to write the data", http.StatusInternalServerError)
	}
}

func (a *API) choosePrimaryPlatform(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		a.p.API.LogError("Not authorized")
		http.Error(w, "not authorized", http.StatusUnauthorized)
		return
	}

	primaryPlatform := r.URL.Query().Get(QueryParamPrimaryPlatform)
	if primaryPlatform != constants.PreferenceValuePlatformMM && primaryPlatform != constants.PreferenceValuePlatformMSTeams {
		a.p.API.LogError("Invalid primary platform", "PrimaryPlatform", primaryPlatform)
		http.Error(w, "invalid primary platform", http.StatusBadRequest)
		return
	}

	err := a.p.API.UpdatePreferencesForUser(userID, []model.Preference{{
		UserId:   userID,
		Category: getPreferenceCategoryName(),
		Name:     constants.PreferenceNamePlatform,
		Value:    primaryPlatform,
	}})

	if err != nil {
		a.p.API.LogError("Error when updating the preferences", "error", err)
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

// handleStaticFiles handles the static files under the assets directory.
func (p *Plugin) handleStaticFiles(r *mux.Router) {
	bundlePath, err := p.API.GetBundlePath()
	if err != nil {
		p.API.LogWarn("Failed to get bundle path.", "Error", err.Error())
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
