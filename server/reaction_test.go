package main

import (
	"database/sql"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	metricsmocks "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSetChatReaction(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setupChat := func(t *testing.T, emojiName string) (*model.User, *model.Channel, string, string) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user2.Id)
		require.NoError(t, err)

		chatID := model.NewId()
		messageID := model.NewId()
		mockTeamsHelper := newMockTeamsHelper(th)
		mockTeamsHelper.registerChat(chatID, []*model.User{senderUser, user2})
		mockTeamsHelper.registerChatMessage(chatID, messageID, senderUser, "message")

		return senderUser, channel, chatID, messageID
	}

	t.Run("sender not connected", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, _, messageID := setupChat(t, emojiName)

		updateRequired := true
		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("failed to set the chat reaction", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)
		th.ConnectUser(t, senderUser.Id)

		updateRequired := true

		th.clientMock.On("SetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(nil, errors.New("unable to set the chat reaction"))

		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.Error(t, err)
	})

	t.Run("chat reaction added", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)

		th.ConnectUser(t, senderUser.Id)

		updateRequired := true

		th.clientMock.On("SetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(&clientmodels.Message{
			LastUpdateAt: time.Now(),
		}, nil).Once()

		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.NoError(t, err)
	})

	t.Run("chat reaction added, update not required", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, _, messageID := setupChat(t, emojiName)

		th.ConnectUser(t, senderUser.Id)

		updateRequired := false

		err := th.p.SetChatReaction(messageID, senderUser.Id, channel.Id, emojiName, updateRequired)
		require.NoError(t, err)
	})
}

func TestSetReaction(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setup := func(t *testing.T, emojiName string, linkPost bool) (*model.User, *storemodels.ChannelLink, *model.Post, storemodels.PostInfo) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)

		channel := th.SetupPublicChannel(t, team, WithMembers(senderUser))
		channelLink := th.LinkChannel(t, team, channel, senderUser)

		post := &model.Post{
			UserId:    senderUser.Id,
			ChannelId: channel.Id,
			Message:   "test set reaction",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		var postInfo storemodels.PostInfo
		if linkPost {
			postInfo = storemodels.PostInfo{
				MattermostID:        post.Id,
				MSTeamsChannel:      model.NewId(),
				MSTeamsID:           model.NewId(),
				MSTeamsLastUpdateAt: time.Now(),
			}
			err = th.p.GetStore().LinkPosts(postInfo)
			require.NoError(t, err)

			mockTeamsHelper := newMockTeamsHelper(th)
			mockTeamsHelper.registerMessage(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, postInfo.MSTeamsID, senderUser, post.Message)
		}

		return senderUser, channelLink, post, postInfo
	}

	t.Run("sender not connected", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, true)

		updateRequired := true
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.ErrorContains(t, err, "not connected user")
	})

	t.Run("post not linked", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, false)

		err := th.p.UnsetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("failed to set the reaction", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, postInfo := setup(t, emojiName, true)
		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("SetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", postInfo.MSTeamsID, "t"+senderUser.Id, "🎉").Return(nil, errors.New("unable to set the reaction"))

		updateRequired := true
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.Error(t, err)
	})

	t.Run("reaction added", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, postInfo := setup(t, emojiName, true)

		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("SetReaction", channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, "", postInfo.MSTeamsID, "t"+senderUser.Id, "🎉").Return(&clientmodels.Message{
			LastUpdateAt: time.Now(),
		}, nil).Once()

		updateRequired := true
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.NoError(t, err)
	})

	t.Run("reaction added, update not required", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channelLink, post, _ := setup(t, emojiName, true)

		th.ConnectUser(t, senderUser.Id)

		updateRequired := false
		err := th.p.SetReaction(channelLink.MSTeamsTeam, channelLink.MSTeamsChannel, senderUser.Id, post, emojiName, updateRequired)
		require.NoError(t, err)
	})
}

func TestUnsetChatReaction(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	setupChat := func(t *testing.T, emojiName string) (*model.User, *model.Channel, string, string) {
		th.Reset(t)

		senderUser := th.SetupUser(t, team)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		channel, err := th.p.apiClient.Channel.GetDirect(senderUser.Id, user2.Id)
		require.NoError(t, err)

		chatID := model.NewId()
		messageID := model.NewId()
		mockTeamsHelper := newMockTeamsHelper(th)
		mockTeamsHelper.registerChat(chatID, []*model.User{senderUser, user2})
		mockTeamsHelper.registerChatMessage(chatID, messageID, senderUser, "message")

		return senderUser, channel, chatID, messageID
	}

	t.Run("sender not connected", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, messageID, _ := setupChat(t, emojiName)

		err := th.p.UnsetChatReaction(messageID, senderUser.Id, channel.Id, emojiName)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("failed to set the chat reaction", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)
		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("UnsetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(nil, errors.New("unable to unset the chat reaction"))

		err := th.p.UnsetChatReaction(messageID, senderUser.Id, channel.Id, emojiName)
		require.Error(t, err)
	})

	t.Run("chat reaction removed", func(t *testing.T) {
		emojiName := "tada"
		senderUser, channel, chatID, messageID := setupChat(t, emojiName)
		th.ConnectUser(t, senderUser.Id)

		th.clientMock.On("UnsetChatReaction", chatID, messageID, "t"+senderUser.Id, "🎉").Return(&clientmodels.Message{
			LastUpdateAt: time.Now(),
		}, nil).Once()

		err := th.p.UnsetChatReaction(messageID, senderUser.Id, channel.Id, emojiName)
		require.NoError(t, err)
	})
}

func TestUnsetReaction(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:           testutils.GetID(),
		TeamID:       "mockTeamsTeamID",
		ChannelID:    "mockTeamsChannelID",
		LastUpdateAt: testutils.GetMockTime(),
	}
	for _, test := range []struct {
		Name            string
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
	}{
		{
			Name:     "UnsetReaction: Unable to get the post info",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "unable to get the post info",
		},
		{
			Name:     "UnsetReaction: Post info is nil",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "teams message not found",
		},
		{
			Name:     "UnsetReaction: Unable to get the client",
			SetupAPI: func(a *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", mock.Anything).Return(nil, nil).Times(2)
			},
			SetupClient:     func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:    func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedMessage: "not connected user",
		},
		{
			Name: "UnsetReaction: Unable to unset the reaction",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(nil, errors.New("unable to unset the reaction")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "unable to unset the reaction",
		},
		{
			Name: "UnsetReaction: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Once()
				store.On("SetPostLastUpdateAtByMattermostID", "", testutils.GetMockTime()).Return(nil).Once()
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UnsetReaction", "mockTeamsTeamID", "mockTeamsChannelID", "", "", testutils.GetID(), ":mockName:").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveReaction", metrics.ReactionUnsetAction, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UnsetReaction", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))

			resp := p.UnsetReaction("mockTeamsTeamID", "mockTeamsChannelID", testutils.GetUserID(), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), "mockName")
			if test.ExpectedMessage != "" {
				assert.Contains(resp.Error(), test.ExpectedMessage)
			} else {
				assert.Nil(resp)
			}
		})
	}
}
