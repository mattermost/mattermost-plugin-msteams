//go:generate mockery --name=Store
//go:generate go run layer_generators/main.go
package store

import (
	"database/sql"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"golang.org/x/oauth2"
)

type Store interface {
	Init() error
	GetAvatarCache(userID string) ([]byte, error)
	SetAvatarCache(userID string, photo []byte) error
	GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error)
	ListChannelLinks() ([]storemodels.ChannelLink, error)
	ListChannelLinksWithNames() ([]*storemodels.ChannelLink, error)
	GetLinkByMSTeamsChannelID(teamID, channelID string) (*storemodels.ChannelLink, error)
	DeleteLinkByChannelID(channelID string) error
	StoreChannelLink(link *storemodels.ChannelLink) error
	GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error)
	GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error)
	LinkPosts(tx *sql.Tx, postInfo storemodels.PostInfo) error
	SetPostLastUpdateAtByMattermostID(tx *sql.Tx, postID string, lastUpdateAt time.Time) error
	SetPostLastUpdateAtByMSTeamsID(tx *sql.Tx, postID string, lastUpdateAt time.Time) error
	GetTokenForMattermostUser(userID string) (*oauth2.Token, error)
	GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error)
	SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error
	DeleteUserInfo(mmUserID string) error
	TeamsToMattermostUserID(userID string) (string, error)
	MattermostToTeamsUserID(userID string) (string, error)
	CheckEnabledTeamByTeamID(teamID string) bool
	ListGlobalSubscriptions() ([]*storemodels.GlobalSubscription, error)
	ListGlobalSubscriptionsToRefresh() ([]*storemodels.GlobalSubscription, error)
	ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error)
	ListChannelSubscriptions() ([]*storemodels.ChannelSubscription, error)
	ListChannelSubscriptionsToRefresh() ([]*storemodels.ChannelSubscription, error)
	SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error
	SaveChatSubscription(subscription storemodels.ChatSubscription) error
	SaveChannelSubscription(tx *sql.Tx, subscription storemodels.ChannelSubscription) error
	UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error
	DeleteSubscription(subscriptionID string) error
	GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error)
	GetChannelSubscriptionByTeamsChannelID(teamsChannelID string) (*storemodels.ChannelSubscription, error)
	GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error)
	GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error)
	GetSubscriptionType(subscriptionID string) (string, error)
	StoreDMAndGMChannelPromptTime(channelID, userID string, timestamp time.Time) error
	GetDMAndGMChannelPromptTime(channelID, userID string) (time.Time, error)
	DeleteDMAndGMChannelPromptTime(userID string) error
	RecoverPost(postID string) error
	StoreOAuth2State(state string) error
	VerifyOAuth2State(state string) error
	SetJobStatus(jobName string, status bool) error
	CompareAndSetJobStatus(jobName string, oldStatus, newStatus bool) (bool, error)
	GetStats() (*storemodels.Stats, error)
	GetConnectedUsers(page, perPage int) ([]*storemodels.ConnectedUser, error)
	PrefillWhitelist() error
	GetSizeOfWhitelist() (int, error)
	StoreUserInWhitelist(userID string) error
	IsUserPresentInWhitelist(userID string) (bool, error)
	LockPostByMSTeamsPostID(tx *sql.Tx, messageID string) error
	LockPostByMMPostID(tx *sql.Tx, messageID string) error
	BeginTx() (*sql.Tx, error)
	RollbackTx(tx *sql.Tx) error
	CommitTx(tx *sql.Tx) error
}
