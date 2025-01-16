// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"slices"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/mock"
)

// mockTeams is an abstraction over directly mocking the client calls made by the plugin to instead
// model the expected state, relieving many tests of the explicit chore of "reimplementing" calls.
type mockTeamsHelper struct {
	th *testHelper
}

func newMockTeamsHelper(th *testHelper) *mockTeamsHelper {
	return &mockTeamsHelper{
		th: th,
	}
}

func (mth *mockTeamsHelper) registerChat(chatID string, users []*model.User) {
	var members []clientmodels.ChatMember
	for _, user := range users {
		mth.registerUser(user)
		members = append(members, clientmodels.ChatMember{
			UserID: "t" + user.Id,
		})
	}

	mth.th.appClientMock.On("GetChat", chatID).Return(&clientmodels.Chat{
		ID:      chatID,
		Members: members,
		Type:    "D",
	}, nil).Maybe()

	var userIDs []string
	for _, user := range users {
		userIDs = append(userIDs, "t"+user.Id)
	}
	slices.Sort(userIDs)

	userIDsMatcher := mock.MatchedBy(func(actualUserIDs []string) bool {
		slices.Sort(actualUserIDs)
		return slices.Compare(userIDs, actualUserIDs) == 0
	})

	mth.th.clientMock.On("CreateOrGetChatForUsers", userIDsMatcher).Return(&clientmodels.Chat{
		ID:      chatID,
		Members: members,
		Type:    "D",
	}, nil).Maybe()
}

func (mth *mockTeamsHelper) registerGroupChat(chatID string, users []*model.User) {
	var members []clientmodels.ChatMember
	for _, user := range users {
		mth.registerUser(user)
		members = append(members, clientmodels.ChatMember{
			UserID: "t" + user.Id,
		})
	}

	mth.th.appClientMock.On("GetChat", chatID).Return(&clientmodels.Chat{
		ID:      chatID,
		Members: members,
		Type:    "G",
	}, nil).Maybe()

	var userIDs []string
	for _, user := range users {
		userIDs = append(userIDs, "t"+user.Id)
	}
	slices.Sort(userIDs)

	userIDsMatcher := mock.MatchedBy(func(actualUserIDs []string) bool {
		slices.Sort(actualUserIDs)
		return slices.Compare(userIDs, actualUserIDs) == 0
	})

	mth.th.clientMock.On("CreateOrGetChatForUsers", userIDsMatcher).Return(&clientmodels.Chat{
		ID:      chatID,
		Members: members,
		Type:    "G",
	}, nil).Maybe()
}

func (mth *mockTeamsHelper) registerChatMessage(chatID string, messageID string, senderUser *model.User, message string) {
	now := time.Now()

	mth.registerUser(senderUser)
	mth.th.clientMock.On("GetChatMessage", chatID, messageID).Return(
		&clientmodels.Message{
			ID:              messageID,
			UserID:          "t" + senderUser.Id,
			ChatID:          chatID,
			UserDisplayName: senderUser.GetDisplayName(model.ShowFullName),
			Text:            message,
			CreateAt:        now,
			LastUpdateAt:    now,
		}, nil).Maybe()
}

func (mth *mockTeamsHelper) registerUser(user *model.User) {
	mth.th.appClientMock.On("GetUser", "t"+user.Id).Return(&clientmodels.User{
		ID: "t" + user.Id,
	}, nil).Maybe()
}
