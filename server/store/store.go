//go:generate mockery --name=Store
//go:generate go run layer_generators/main.go
package store

import (
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"golang.org/x/oauth2"
)

type Store interface {
	Init(remoteID string) error

	// teams
	CheckEnabledTeamByTeamID(teamID string) bool

	// users
	TeamsToMattermostUserID(userID string) (string, error)
	MattermostToTeamsUserID(userID string) (string, error)
	GetTokenForMattermostUser(userID string) (*oauth2.Token, error)
	GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error)
	GetConnectedUsers(page, perPage int) ([]*storemodels.ConnectedUser, error)
	UserHasConnected(mmUserID string) (bool, error)
	GetUserConnectStatus(mmUserID string) (*storemodels.UserConnectStatus, error)
	GetHasConnectedCount() (int, error)
	SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error
	DeleteUserInfo(mmUserID string) error
	SetUserLastChatSentAt(mmUserID string, sentAt int64) error
	SetUserLastChatReceivedAt(mmUserID string, receivedAt int64) error
	SetUsersLastChatReceivedAt(mmUserIDs []string, receivedAt int64) error

	// auth
	StoreOAuth2State(state string) error
	VerifyOAuth2State(state string) error

	// invites & whitelist
	StoreInvitedUser(invitedUser *storemodels.InvitedUser) error
	GetInvitedUser(mmUserID string) (*storemodels.InvitedUser, error)
	DeleteUserInvite(mmUserID string) error
	GetInvitedCount() (int, error)
	StoreUserInWhitelist(userID string) error
	IsUserWhitelisted(userID string) (bool, error)
	DeleteUserFromWhitelist(userID string) error
	GetWhitelistCount() (int, error)
	GetWhitelistEmails(page int, perPage int) ([]string, error)
	SetWhitelist(userIDs []string, batchSize int) error

	// stats
	GetLinkedChannelsCount() (linkedChannels int64, err error)
	GetConnectedUsersCount() (connectedUsers int64, err error)
	GetSyntheticUsersCount(remoteID string) (syntheticUsers int64, err error)
	GetUsersByPrimaryPlatformsCount(preferenceCategory string) (msTeamsPrimary int64, mmPrimary int64, err error)
	GetActiveUsersSendingCount(dur time.Duration) (activeUsersSending int64, err error)
	GetActiveUsersReceivingCount(dur time.Duration) (activeUsersReceiving int64, err error)

	// links, channels, posts
	GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error)
	ListChannelLinks() ([]storemodels.ChannelLink, error)
	ListChannelLinksWithNames() ([]*storemodels.ChannelLink, error)
	GetLinkByMSTeamsChannelID(teamID, channelID string) (*storemodels.ChannelLink, error)
	DeleteLinkByChannelID(channelID string) error
	StoreChannelLink(link *storemodels.ChannelLink) error
	GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error)
	GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error)
	LinkPosts(postInfo storemodels.PostInfo) error
	SetPostLastUpdateAtByMattermostID(postID string, lastUpdateAt time.Time) error
	SetPostLastUpdateAtByMSTeamsID(postID string, lastUpdateAt time.Time) error
	RecoverPost(postID string) error

	// subscriptions
	ListGlobalSubscriptions() ([]*storemodels.GlobalSubscription, error)
	ListGlobalSubscriptionsToRefresh(certificate string) ([]*storemodels.GlobalSubscription, error)
	ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error)
	ListChannelSubscriptions() ([]*storemodels.ChannelSubscription, error)
	ListChannelSubscriptionsToRefresh(certificate string) ([]*storemodels.ChannelSubscription, error)
	SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error
	SaveChatSubscription(subscription storemodels.ChatSubscription) error
	SaveChannelSubscription(subscription storemodels.ChannelSubscription) error
	UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error
	DeleteSubscription(subscriptionID string) error
	GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error)
	GetChannelSubscriptionByTeamsChannelID(teamsChannelID string) (*storemodels.ChannelSubscription, error)
	GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error)
	GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error)
	GetSubscriptionType(subscriptionID string) (string, error)
	UpdateSubscriptionLastActivityAt(subscriptionID string, lastActivityAt time.Time) error
	GetSubscriptionsLastActivityAt() (map[string]time.Time, error)
}
