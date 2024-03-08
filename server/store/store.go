//go:generate mockery --name=Store
//go:generate go run layer_generators/main.go
package store

import (
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"golang.org/x/oauth2"
)

type Store interface {
	Init() error
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
	GetTokenForMattermostUser(userID string) (*oauth2.Token, error)
	GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error)
	SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error
	DeleteUserInfo(mmUserID string) error
	TeamsToMattermostUserID(userID string) (string, error)
	MattermostToTeamsUserID(userID string) (string, error)
	CheckEnabledTeamByTeamID(teamID string) bool
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
	StoreDMAndGMChannelPromptTime(channelID, userID string, timestamp time.Time) error
	GetDMAndGMChannelPromptTime(channelID, userID string) (time.Time, error)
	DeleteDMAndGMChannelPromptTime(userID string) error
	RecoverPost(postID string) error
	StoreOAuth2State(state string) error
	VerifyOAuth2State(state string) error
	GetStats() (*storemodels.Stats, error)
	GetConnectedUsers(page, perPage int) ([]*storemodels.ConnectedUser, error)
	PrefillWhitelist() error
	GetSizeOfWhitelist() (int, error)
	StoreUserInWhitelist(userID string) error
	IsUserPresentInWhitelist(userID string) (bool, error)
	StoreInvitedUser(invitedUser *storemodels.InvitedUser) error
	GetInvitedUser(mmUserID string) (*storemodels.InvitedUser, error)
	DeleteUserInvite(mmUserID string) error
	GetSizeOfInvitedUsers() (int, error)
	GetSizeOfUnresponsiveInvitedUsers(unresponsiveCutoff time.Time) (int, error)
	UpdateSubscriptionLastActivityAt(subscriptionID string, lastActivityAt time.Time) error
	GetSubscriptionsLastActivityAt() (map[string]time.Time, error)
}
