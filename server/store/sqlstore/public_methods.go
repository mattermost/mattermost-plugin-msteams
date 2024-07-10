// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Code generated by "make generate"
// DO NOT EDIT

// To add a public method, create its private implementation inside
// the sqlstore package with sq.BaseRunner as its first
// parameter. Annotate it with a @withTransaction comment if you need
// it to be transactional, or @withReplica if the method queries can
// go to a database replica instead of the master node. Then run `make
// generate` for the public method of the store to be created.

package sqlstore

import (
	"context"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"

	"golang.org/x/oauth2"
)

func (s *SQLStore) DeleteLinkByChannelID(channelID string) error {
	return s.deleteLinkByChannelID(s.db, channelID)
}

func (s *SQLStore) DeleteSubscription(subscriptionID string) error {
	return s.deleteSubscription(s.db, subscriptionID)
}

func (s *SQLStore) DeleteUserFromWhitelist(userID string) error {
	return s.deleteUserFromWhitelist(s.db, userID)
}

func (s *SQLStore) DeleteUserInfo(mmUserID string) error {
	return s.deleteUserInfo(s.db, mmUserID)
}

func (s *SQLStore) DeleteUserInvite(mmUserID string) error {
	return s.deleteUserInvite(s.db, mmUserID)
}

func (s *SQLStore) GetActiveUsersReceivingCount(dur time.Duration) (int64, error) {
	return s.getActiveUsersReceivingCount(s.replica, dur)
}

func (s *SQLStore) GetActiveUsersSendingCount(dur time.Duration) (int64, error) {
	return s.getActiveUsersSendingCount(s.replica, dur)
}

func (s *SQLStore) GetChannelSubscription(subscriptionID string) (*storemodels.ChannelSubscription, error) {
	return s.getChannelSubscription(s.replica, subscriptionID)
}

func (s *SQLStore) GetChannelSubscriptionByTeamsChannelID(teamsChannelID string) (*storemodels.ChannelSubscription, error) {
	return s.getChannelSubscriptionByTeamsChannelID(s.replica, teamsChannelID)
}

func (s *SQLStore) GetChatSubscription(subscriptionID string) (*storemodels.ChatSubscription, error) {
	return s.getChatSubscription(s.replica, subscriptionID)
}

func (s *SQLStore) GetConnectedUsers(page int, perPage int) ([]*storemodels.ConnectedUser, error) {
	return s.getConnectedUsers(s.replica, page, perPage)
}

func (s *SQLStore) GetConnectedUsersCount() (int64, error) {
	return s.getConnectedUsersCount(s.replica)
}

func (s *SQLStore) GetGlobalSubscription(subscriptionID string) (*storemodels.GlobalSubscription, error) {
	return s.getGlobalSubscription(s.replica, subscriptionID)
}

func (s *SQLStore) GetHasConnectedCount() (int, error) {
	return s.getHasConnectedCount(s.replica)
}

func (s *SQLStore) GetInvitedCount() (int, error) {
	return s.getInvitedCount(s.replica)
}

func (s *SQLStore) GetInvitedUser(mmUserID string) (*storemodels.InvitedUser, error) {
	return s.getInvitedUser(s.replica, mmUserID)
}

func (s *SQLStore) GetLinkByChannelID(channelID string) (*storemodels.ChannelLink, error) {
	return s.getLinkByChannelID(s.replica, channelID)
}

func (s *SQLStore) GetLinkByMSTeamsChannelID(teamID string, channelID string) (*storemodels.ChannelLink, error) {
	return s.getLinkByMSTeamsChannelID(s.replica, teamID, channelID)
}

func (s *SQLStore) GetLinkedChannelsCount() (int64, error) {
	return s.getLinkedChannelsCount(s.replica)
}

func (s *SQLStore) GetPostInfoByMSTeamsID(chatID string, postID string) (*storemodels.PostInfo, error) {
	return s.getPostInfoByMSTeamsID(s.replica, chatID, postID)
}

func (s *SQLStore) GetPostInfoByMattermostID(postID string) (*storemodels.PostInfo, error) {
	return s.getPostInfoByMattermostID(s.replica, postID)
}

func (s *SQLStore) GetSubscriptionType(subscriptionID string) (string, error) {
	return s.getSubscriptionType(s.replica, subscriptionID)
}

func (s *SQLStore) GetSubscriptionsLastActivityAt() (map[string]time.Time, error) {
	return s.getSubscriptionsLastActivityAt(s.replica)
}

func (s *SQLStore) GetTokenForMSTeamsUser(userID string) (*oauth2.Token, error) {
	return s.getTokenForMSTeamsUser(s.replica, userID)
}

func (s *SQLStore) GetTokenForMattermostUser(userID string) (*oauth2.Token, error) {
	return s.getTokenForMattermostUser(s.replica, userID)
}

func (s *SQLStore) GetUserConnectStatus(mmUserID string) (*storemodels.UserConnectStatus, error) {
	return s.getUserConnectStatus(s.replica, mmUserID)
}

func (s *SQLStore) GetWhitelistCount() (int, error) {
	return s.getWhitelistCount(s.replica)
}

func (s *SQLStore) GetWhitelistEmails(page int, perPage int) ([]string, error) {
	return s.getWhitelistEmails(s.replica, page, perPage)
}

func (s *SQLStore) IsUserWhitelisted(userID string) (bool, error) {
	return s.isUserWhitelisted(s.replica, userID)
}

func (s *SQLStore) LinkPosts(postInfo storemodels.PostInfo) error {
	return s.linkPosts(s.db, postInfo)
}

func (s *SQLStore) ListChannelLinks() ([]storemodels.ChannelLink, error) {
	return s.listChannelLinks(s.replica)
}

func (s *SQLStore) ListChannelLinksWithNames() ([]*storemodels.ChannelLink, error) {
	return s.listChannelLinksWithNames(s.replica)
}

func (s *SQLStore) ListChannelSubscriptions() ([]*storemodels.ChannelSubscription, error) {
	return s.listChannelSubscriptions(s.replica)
}

func (s *SQLStore) ListChannelSubscriptionsToRefresh() ([]*storemodels.ChannelSubscription, error) {
	return s.listChannelSubscriptionsToRefresh(s.replica)
}

func (s *SQLStore) ListChatSubscriptionsToCheck() ([]storemodels.ChatSubscription, error) {
	return s.listChatSubscriptionsToCheck(s.replica)
}

func (s *SQLStore) ListGlobalSubscriptions() ([]*storemodels.GlobalSubscription, error) {
	return s.listGlobalSubscriptions(s.replica)
}

func (s *SQLStore) ListGlobalSubscriptionsToRefresh() ([]*storemodels.GlobalSubscription, error) {
	return s.listGlobalSubscriptionsToRefresh(s.replica)
}

func (s *SQLStore) MattermostToTeamsUserID(userID string) (string, error) {
	return s.mattermostToTeamsUserID(s.replica, userID)
}

func (s *SQLStore) RecoverPost(postID string) error {
	return s.recoverPost(s.db, postID)
}

func (s *SQLStore) SaveChannelSubscription(subscription storemodels.ChannelSubscription) error {
	tx, txErr := s.db.BeginTx(context.Background(), nil)
	if txErr != nil {
		return txErr
	}
	err := s.saveChannelSubscription(tx, subscription)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.api.LogError("transaction rollback error", "Error", rollbackErr, "methodName", "SaveChannelSubscription")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) SaveChatSubscription(subscription storemodels.ChatSubscription) error {
	tx, txErr := s.db.BeginTx(context.Background(), nil)
	if txErr != nil {
		return txErr
	}
	err := s.saveChatSubscription(tx, subscription)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.api.LogError("transaction rollback error", "Error", rollbackErr, "methodName", "SaveChatSubscription")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) SaveGlobalSubscription(subscription storemodels.GlobalSubscription) error {
	tx, txErr := s.db.BeginTx(context.Background(), nil)
	if txErr != nil {
		return txErr
	}
	err := s.saveGlobalSubscription(tx, subscription)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.api.LogError("transaction rollback error", "Error", rollbackErr, "methodName", "SaveGlobalSubscription")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) SetPostLastUpdateAtByMSTeamsID(postID string, lastUpdateAt time.Time) error {
	return s.setPostLastUpdateAtByMSTeamsID(s.db, postID, lastUpdateAt)
}

func (s *SQLStore) SetPostLastUpdateAtByMattermostID(postID string, lastUpdateAt time.Time) error {
	return s.setPostLastUpdateAtByMattermostID(s.db, postID, lastUpdateAt)
}

func (s *SQLStore) SetUserInfo(userID string, msTeamsUserID string, token *oauth2.Token) error {
	tx, txErr := s.db.BeginTx(context.Background(), nil)
	if txErr != nil {
		return txErr
	}
	err := s.setUserInfo(tx, userID, msTeamsUserID, token)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.api.LogError("transaction rollback error", "Error", rollbackErr, "methodName", "SetUserInfo")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) SetUserLastChatReceivedAt(mmUserID string, receivedAt int64) error {
	return s.setUserLastChatReceivedAt(s.db, mmUserID, receivedAt)
}

func (s *SQLStore) SetUserLastChatSentAt(mmUserID string, sentAt int64) error {
	return s.setUserLastChatSentAt(s.db, mmUserID, sentAt)
}

func (s *SQLStore) SetUsersLastChatReceivedAt(mmUserIDs []string, receivedAt int64) error {
	return s.setUsersLastChatReceivedAt(s.db, mmUserIDs, receivedAt)
}

func (s *SQLStore) SetWhitelist(userIDs []string, batchSize int) error {
	tx, txErr := s.db.BeginTx(context.Background(), nil)
	if txErr != nil {
		return txErr
	}
	err := s.setWhitelist(tx, userIDs, batchSize)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			s.api.LogError("transaction rollback error", "Error", rollbackErr, "methodName", "SetWhitelist")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (s *SQLStore) StoreChannelLink(link *storemodels.ChannelLink) error {
	return s.storeChannelLink(s.db, link)
}

func (s *SQLStore) StoreInvitedUser(invitedUser *storemodels.InvitedUser) error {
	return s.storeInvitedUser(s.db, invitedUser)
}

func (s *SQLStore) StoreUserInWhitelist(userID string) error {
	return s.storeUserInWhitelist(s.db, userID)
}

func (s *SQLStore) TeamsToMattermostUserID(userID string) (string, error) {
	return s.teamsToMattermostUserID(s.replica, userID)
}

func (s *SQLStore) UpdateSubscriptionExpiresOn(subscriptionID string, expiresOn time.Time) error {
	return s.updateSubscriptionExpiresOn(s.db, subscriptionID, expiresOn)
}

func (s *SQLStore) UpdateSubscriptionLastActivityAt(subscriptionID string, lastActivityAt time.Time) error {
	return s.updateSubscriptionLastActivityAt(s.db, subscriptionID, lastActivityAt)
}
