// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Code generated by "make generate"
// DO NOT EDIT

package timerlayer

import (
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"

	"golang.org/x/oauth2"
)

type TimerLayer struct {
	store.Store
	metrics metrics.Metrics
}

func (s *TimerLayer) CheckEnabledTeamByTeamID(teamID string) bool {
	start := time.Now()

	result := s.Store.CheckEnabledTeamByTeamID(teamID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if true {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.CheckEnabledTeamByTeamID", success, elapsed)
	return result
}

func (s *TimerLayer) CompareAndSetJobStatus(jobName string, oldStatus bool, newStatus bool) (bool, error) {
	start := time.Now()

	result, err := s.Store.CompareAndSetJobStatus(jobName, oldStatus, newStatus)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.CompareAndSetJobStatus", success, elapsed)
	return result, err
}

func (s *TimerLayer) DeleteDMAndGMChannelPromptTime(userID string) error {
	start := time.Now()

	err := s.Store.DeleteDMAndGMChannelPromptTime(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.DeleteDMAndGMChannelPromptTime", success, elapsed)
	return err
}

func (s *TimerLayer) DeleteLinkByChannelID(channelID string) error {
	start := time.Now()

	err := s.Store.DeleteLinkByChannelID(channelID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.DeleteLinkByChannelID", success, elapsed)
	return err
}

func (s *TimerLayer) DeleteSubscription(subscriptionID string) error {
	start := time.Now()

	err := s.Store.DeleteSubscription(subscriptionID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.DeleteSubscription", success, elapsed)
	return err
}

func (s *TimerLayer) DeleteUserInfo(mmUserID string) error {
	start := time.Now()

	err := s.Store.DeleteUserInfo(mmUserID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.DeleteUserInfo", success, elapsed)
	return err
}

func (s *TimerLayer) GetAvatarCache(userID string) ([]byte, error) {
	start := time.Now()

	result, err := s.Store.GetAvatarCache(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetAvatarCache", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error) {
	start := time.Now()

	result, err := s.Store.GetChannelSubscription(subscriptionID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetChannelSubscription", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetChannelSubscriptionByTeamsChannelID(teamsChannelID string) (*storemodels.ChannelSubscription, error) {
	start := time.Now()

	result, err := s.Store.GetChannelSubscriptionByTeamsChannelID(teamsChannelID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetChannelSubscriptionByTeamsChannelID", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error) {
	start := time.Now()

	result, err := s.Store.GetChatSubscription(subscriptionID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetChatSubscription", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetConnectedUsers(page int, perPage int) ([]*storemodels.ConnectedUser, error) {
	start := time.Now()

	result, err := s.Store.GetConnectedUsers(page, perPage)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetConnectedUsers", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetDMAndGMChannelPromptTime(channelID string, userID string) (time.Time, error) {
	start := time.Now()

	result, err := s.Store.GetDMAndGMChannelPromptTime(channelID, userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetDMAndGMChannelPromptTime", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error) {
	start := time.Now()

	result, err := s.Store.GetGlobalSubscription(subscriptionID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetGlobalSubscription", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error) {
	start := time.Now()

	result, err := s.Store.GetLinkByChannelID(channelID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetLinkByChannelID", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetLinkByMSTeamsChannelID(teamID string, channelID string) (*storemodels.ChannelLink, error) {
	start := time.Now()

	result, err := s.Store.GetLinkByMSTeamsChannelID(teamID, channelID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetLinkByMSTeamsChannelID", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetMattermostAdminsIds() ([]string, error) {
	start := time.Now()

	result, err := s.Store.GetMattermostAdminsIds()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetMattermostAdminsIds", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error) {
	start := time.Now()

	result, err := s.Store.GetPostInfoByMSTeamsID(chatID, postID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetPostInfoByMSTeamsID", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error) {
	start := time.Now()

	result, err := s.Store.GetPostInfoByMattermostID(postID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetPostInfoByMattermostID", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetSizeOfWhitelist() (int, error) {
	start := time.Now()

	result, err := s.Store.GetSizeOfWhitelist()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetSizeOfWhitelist", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetStats() (*storemodels.Stats, error) {
	start := time.Now()

	result, err := s.Store.GetStats()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetStats", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetSubscriptionType(subscriptionID string) (string, error) {
	start := time.Now()

	result, err := s.Store.GetSubscriptionType(subscriptionID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetSubscriptionType", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetSubscriptionsLastActivityAt() (map[string]time.Time, error) {
	start := time.Now()

	result, err := s.Store.GetSubscriptionsLastActivityAt()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetSubscriptionsLastActivityAt", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error) {
	start := time.Now()

	result, err := s.Store.GetTokenForMSTeamsUser(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetTokenForMSTeamsUser", success, elapsed)
	return result, err
}

func (s *TimerLayer) GetTokenForMattermostUser(userID string) (*oauth2.Token, error) {
	start := time.Now()

	result, err := s.Store.GetTokenForMattermostUser(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.GetTokenForMattermostUser", success, elapsed)
	return result, err
}

func (s *TimerLayer) Init() error {
	start := time.Now()

	err := s.Store.Init()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.Init", success, elapsed)
	return err
}

func (s *TimerLayer) IsUserPresentInWhitelist(userID string) (bool, error) {
	start := time.Now()

	result, err := s.Store.IsUserPresentInWhitelist(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.IsUserPresentInWhitelist", success, elapsed)
	return result, err
}

func (s *TimerLayer) LinkPosts(postInfo storemodels.PostInfo) error {
	start := time.Now()

	err := s.Store.LinkPosts(postInfo)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.LinkPosts", success, elapsed)
	return err
}

func (s *TimerLayer) ListChannelLinks() ([]storemodels.ChannelLink, error) {
	start := time.Now()

	result, err := s.Store.ListChannelLinks()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.ListChannelLinks", success, elapsed)
	return result, err
}

func (s *TimerLayer) ListChannelLinksWithNames() ([]*storemodels.ChannelLink, error) {
	start := time.Now()

	result, err := s.Store.ListChannelLinksWithNames()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.ListChannelLinksWithNames", success, elapsed)
	return result, err
}

func (s *TimerLayer) ListChannelSubscriptions() ([]*storemodels.ChannelSubscription, error) {
	start := time.Now()

	result, err := s.Store.ListChannelSubscriptions()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.ListChannelSubscriptions", success, elapsed)
	return result, err
}

func (s *TimerLayer) ListChannelSubscriptionsToRefresh(certificate string) ([]*storemodels.ChannelSubscription, error) {
	start := time.Now()

	result, err := s.Store.ListChannelSubscriptionsToRefresh(certificate)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.ListChannelSubscriptionsToRefresh", success, elapsed)
	return result, err
}

func (s *TimerLayer) ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error) {
	start := time.Now()

	result, err := s.Store.ListChatSubscriptionsToCheck()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.ListChatSubscriptionsToCheck", success, elapsed)
	return result, err
}

func (s *TimerLayer) ListGlobalSubscriptions() ([]*storemodels.GlobalSubscription, error) {
	start := time.Now()

	result, err := s.Store.ListGlobalSubscriptions()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.ListGlobalSubscriptions", success, elapsed)
	return result, err
}

func (s *TimerLayer) ListGlobalSubscriptionsToRefresh(certificate string) ([]*storemodels.GlobalSubscription, error) {
	start := time.Now()

	result, err := s.Store.ListGlobalSubscriptionsToRefresh(certificate)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.ListGlobalSubscriptionsToRefresh", success, elapsed)
	return result, err
}

func (s *TimerLayer) MattermostToTeamsUserID(userID string) (string, error) {
	start := time.Now()

	result, err := s.Store.MattermostToTeamsUserID(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.MattermostToTeamsUserID", success, elapsed)
	return result, err
}

func (s *TimerLayer) PrefillWhitelist() error {
	start := time.Now()

	err := s.Store.PrefillWhitelist()

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.PrefillWhitelist", success, elapsed)
	return err
}

func (s *TimerLayer) RecoverPost(postID string) error {
	start := time.Now()

	err := s.Store.RecoverPost(postID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.RecoverPost", success, elapsed)
	return err
}

func (s *TimerLayer) SaveChannelSubscription(subscription storemodels.ChannelSubscription) error {
	start := time.Now()

	err := s.Store.SaveChannelSubscription(subscription)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SaveChannelSubscription", success, elapsed)
	return err
}

func (s *TimerLayer) SaveChatSubscription(subscription storemodels.ChatSubscription) error {
	start := time.Now()

	err := s.Store.SaveChatSubscription(subscription)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SaveChatSubscription", success, elapsed)
	return err
}

func (s *TimerLayer) SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error {
	start := time.Now()

	err := s.Store.SaveGlobalSubscription(subscription)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SaveGlobalSubscription", success, elapsed)
	return err
}

func (s *TimerLayer) SetAvatarCache(userID string, photo []byte) error {
	start := time.Now()

	err := s.Store.SetAvatarCache(userID, photo)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SetAvatarCache", success, elapsed)
	return err
}

func (s *TimerLayer) SetJobStatus(jobName string, status bool) error {
	start := time.Now()

	err := s.Store.SetJobStatus(jobName, status)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SetJobStatus", success, elapsed)
	return err
}

func (s *TimerLayer) SetPostLastUpdateAtByMSTeamsID(postID string, lastUpdateAt time.Time) error {
	start := time.Now()

	err := s.Store.SetPostLastUpdateAtByMSTeamsID(postID, lastUpdateAt)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SetPostLastUpdateAtByMSTeamsID", success, elapsed)
	return err
}

func (s *TimerLayer) SetPostLastUpdateAtByMattermostID(postID string, lastUpdateAt time.Time) error {
	start := time.Now()

	err := s.Store.SetPostLastUpdateAtByMattermostID(postID, lastUpdateAt)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SetPostLastUpdateAtByMattermostID", success, elapsed)
	return err
}

func (s *TimerLayer) SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error {
	start := time.Now()

	err := s.Store.SetUserInfo(userID, msTeamsUserID, token)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.SetUserInfo", success, elapsed)
	return err
}

func (s *TimerLayer) StoreChannelLink(link *storemodels.ChannelLink) error {
	start := time.Now()

	err := s.Store.StoreChannelLink(link)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.StoreChannelLink", success, elapsed)
	return err
}

func (s *TimerLayer) StoreDMAndGMChannelPromptTime(channelID string, userID string, timestamp time.Time) error {
	start := time.Now()

	err := s.Store.StoreDMAndGMChannelPromptTime(channelID, userID, timestamp)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.StoreDMAndGMChannelPromptTime", success, elapsed)
	return err
}

func (s *TimerLayer) StoreOAuth2State(state string) error {
	start := time.Now()

	err := s.Store.StoreOAuth2State(state)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.StoreOAuth2State", success, elapsed)
	return err
}

func (s *TimerLayer) StoreUserInWhitelist(userID string) error {
	start := time.Now()

	err := s.Store.StoreUserInWhitelist(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.StoreUserInWhitelist", success, elapsed)
	return err
}

func (s *TimerLayer) TeamsToMattermostUserID(userID string) (string, error) {
	start := time.Now()

	result, err := s.Store.TeamsToMattermostUserID(userID)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.TeamsToMattermostUserID", success, elapsed)
	return result, err
}

func (s *TimerLayer) UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error {
	start := time.Now()

	err := s.Store.UpdateSubscriptionExpiresOn(subscriptionID, expiresOn)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.UpdateSubscriptionExpiresOn", success, elapsed)
	return err
}

func (s *TimerLayer) UpdateSubscriptionLastActivityAt(subscriptionID string, lastActivityAt time.Time) error {
	start := time.Now()

	err := s.Store.UpdateSubscriptionLastActivityAt(subscriptionID, lastActivityAt)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.UpdateSubscriptionLastActivityAt", success, elapsed)
	return err
}

func (s *TimerLayer) VerifyOAuth2State(state string) error {
	start := time.Now()

	err := s.Store.VerifyOAuth2State(state)

	elapsed := float64(time.Since(start)) / float64(time.Second)
	success := "false"
	if err == nil {
		success = "true"
	}
	s.metrics.ObserveStoreMethodDuration("Store.VerifyOAuth2State", success, elapsed)
	return err
}

func New(childStore store.Store, metrics metrics.Metrics) *TimerLayer {
	return &TimerLayer{
		Store:   childStore,
		metrics: metrics,
	}
}
