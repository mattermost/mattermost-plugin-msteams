//go:generate mockery --name=Store
//go:generate go run generators/main.go
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
	//@withReplica
	TeamsToMattermostUserID(userID string) (string, error)
	//@withReplica
	MattermostToTeamsUserID(userID string) (string, error)
	//@withReplica
	GetTokenForMattermostUser(userID string) (*oauth2.Token, error)
	//@withReplica
	GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error)
	//@withReplica
	GetConnectedUsers(page, perPage int) ([]*storemodels.ConnectedUser, error)
	UserHasConnected(mmUserID string) (bool, error)
	//@withReplica
	GetUserConnectStatus(mmUserID string) (*storemodels.UserConnectStatus, error)
	//@withReplica
	GetHasConnectedCount() (int, error)
	//@withTransaction
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
	//@withReplica
	GetInvitedUser(mmUserID string) (*storemodels.InvitedUser, error)
	DeleteUserInvite(mmUserID string) error
	//@withReplica
	GetInvitedCount() (int, error)
	StoreUserInWhitelist(userID string) error
	//@withReplica
	IsUserWhitelisted(userID string) (bool, error)
	DeleteUserFromWhitelist(userID string) error
	//@withReplica
	GetWhitelistCount() (int, error)
	//@withReplica
	GetWhitelistEmails(page int, perPage int) ([]string, error)
	//@withTransaction
	SetWhitelist(userIDs []string, batchSize int) error

	// stats
	//@withReplica
	GetLinkedChannelsCount() (linkedChannels int64, err error)
	//@withReplica
	GetConnectedUsersCount() (connectedUsers int64, err error)
	//@withReplica
	GetSyntheticUsersCount(remoteID string) (syntheticUsers int64, err error)
	//@withReplica
	GetUsersByPrimaryPlatformsCount(preferenceCategory string) (msTeamsPrimary int64, mmPrimary int64, err error)
	//@withReplica
	GetActiveUsersSendingCount(dur time.Duration) (activeUsersSending int64, err error)
	//@withReplica
	GetActiveUsersReceivingCount(dur time.Duration) (activeUsersReceiving int64, err error)

	// links, channels, posts
	//@withReplica
	GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error)
	//@withReplica
	ListChannelLinks() ([]storemodels.ChannelLink, error)
	//@withReplica
	ListChannelLinksWithNames() ([]*storemodels.ChannelLink, error)
	//@withReplica
	GetLinkByMSTeamsChannelID(teamID, channelID string) (*storemodels.ChannelLink, error)
	DeleteLinkByChannelID(channelID string) error
	StoreChannelLink(link *storemodels.ChannelLink) error
	//@withReplica
	GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error)
	//@withReplica
	GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error)
	LinkPosts(postInfo storemodels.PostInfo) error
	SetPostLastUpdateAtByMattermostID(postID string, lastUpdateAt time.Time) error
	SetPostLastUpdateAtByMSTeamsID(postID string, lastUpdateAt time.Time) error
	RecoverPost(postID string) error

	// subscriptions
	//@withReplica
	ListGlobalSubscriptions() ([]*storemodels.GlobalSubscription, error)
	//@withReplica
	ListGlobalSubscriptionsToRefresh(certificate string) ([]*storemodels.GlobalSubscription, error)
	//@withReplica
	ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error)
	//@withReplica
	ListChannelSubscriptions() ([]*storemodels.ChannelSubscription, error)
	//@withReplica
	ListChannelSubscriptionsToRefresh(certificate string) ([]*storemodels.ChannelSubscription, error)
	//@withTransaction
	SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error
	//@withTransaction
	SaveChatSubscription(subscription storemodels.ChatSubscription) error
	//@withTransaction
	SaveChannelSubscription(subscription storemodels.ChannelSubscription) error
	UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error
	DeleteSubscription(subscriptionID string) error
	//@withReplica
	GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error)
	//@withReplica
	GetChannelSubscriptionByTeamsChannelID(teamsChannelID string) (*storemodels.ChannelSubscription, error)
	//@withReplica
	GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error)
	//@withReplica
	GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error)
	//@withReplica
	GetSubscriptionType(subscriptionID string) (string, error)
	UpdateSubscriptionLastActivityAt(subscriptionID string, lastActivityAt time.Time) error
	//@withReplica
	GetSubscriptionsLastActivityAt() (map[string]time.Time, error)
}
