package main

import (
	"bytes"
	"database/sql"
	"math"
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
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSendChat(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("sender not mapped to teams user", func(t *testing.T) {
		th.Reset(t)
		sender := th.SetupUser(t, team)
		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		post := &model.Post{Id: model.NewId()}
		newMessageID, err := th.p.SendChat(sender.Id, []string{user1.Id, user2.Id}, post)
		require.ErrorIs(t, err, sql.ErrNoRows)
		assert.Empty(t, newMessageID)
	})

	t.Run("sender not connected", func(t *testing.T) {
		th.Reset(t)
		sender := th.SetupUser(t, team)
		th.ConnectUser(t, sender.Id)
		th.DisconnectUser(t, sender.Id)

		user1 := th.SetupUser(t, team)
		user2 := th.SetupUser(t, team)

		post := &model.Post{Id: model.NewId()}
		newMessageID, err := th.p.SendChat(sender.Id, []string{user1.Id, user2.Id}, post)
		require.EqualError(t, err, "not connected user")
		assert.Empty(t, newMessageID)
	})

	t.Run("one unmapped recipient", func(t *testing.T) {
		th.Reset(t)
		sender := th.SetupUser(t, team)
		th.ConnectUser(t, sender.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)

		post := &model.Post{Id: model.NewId()}
		newMessageID, err := th.p.SendChat(sender.Id, []string{user1.Id, user2.Id}, post)
		require.ErrorIs(t, err, sql.ErrNoRows)
		assert.Empty(t, newMessageID)
	})

	t.Run("failed to get chat", func(t *testing.T) {
		th.Reset(t)
		sender := th.SetupUser(t, team)
		th.ConnectUser(t, sender.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		th.clientMock.On(
			"CreateOrGetChatForUsers",
			[]string{
				"t" + user1.Id,
				"t" + user2.Id,
			},
		).Return(nil, errors.New("unable to create or get the chat")).Times(1)

		post := &model.Post{Id: model.NewId()}
		newMessageID, err := th.p.SendChat(sender.Id, []string{user1.Id, user2.Id}, post)
		require.EqualError(t, err, "unable to create or get the chat")
		assert.Empty(t, newMessageID)
	})

	t.Run("failed to send chat", func(t *testing.T) {
		th.Reset(t)
		sender := th.SetupUser(t, team)
		th.ConnectUser(t, sender.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		chatID := model.NewId()

		th.clientMock.On(
			"CreateOrGetChatForUsers",
			[]string{
				"t" + user1.Id,
				"t" + user2.Id,
			},
		).Return(&clientmodels.Chat{
			ID:   chatID,
			Type: "G",
		}, nil)

		th.clientMock.On(
			"SendChat",
			chatID,
			mock.AnythingOfType("string"),
			(*clientmodels.Message)(nil),
			([]*clientmodels.Attachment)(nil),
			[]models.ChatMessageMentionable{},
		).Return(nil, errors.New("unable to send the chat"))

		post := &model.Post{Id: model.NewId()}
		newMessageID, err := th.p.SendChat(sender.Id, []string{user1.Id, user2.Id}, post)
		require.EqualError(t, err, "unable to send the chat")
		assert.Empty(t, newMessageID)

		// TODO: assert post relationship stored
		// postInfo, err := th.p.store.GetPostInfoByMattermostID(post.Id)
		// require.NoError(t, err)
		// assert.Equal(t, message.ID, postInfo.MSTeamsID)
	})

	t.Run("sent chat", func(t *testing.T) {
		th.Reset(t)
		sender := th.SetupUser(t, team)
		th.ConnectUser(t, sender.Id)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		chatID := model.NewId()

		th.clientMock.On(
			"CreateOrGetChatForUsers",
			[]string{
				"t" + user1.Id,
				"t" + user2.Id,
			},
		).Return(&clientmodels.Chat{
			ID:   chatID,
			Type: "G",
		}, nil)

		messageID := model.NewId()

		th.clientMock.On(
			"SendChat",
			chatID,
			mock.AnythingOfType("string"),
			(*clientmodels.Message)(nil),
			([]*clientmodels.Attachment)(nil),
			[]models.ChatMessageMentionable{},
		).Return(&clientmodels.Message{
			ID: messageID,
		},
			nil,
		)

		post := &model.Post{Id: model.NewId()}
		newMessageID, err := th.p.SendChat(sender.Id, []string{user1.Id, user2.Id}, post)
		require.NoError(t, err)
		assert.Equal(t, messageID, newMessageID)

		postInfo, err := th.p.store.GetPostInfoByMattermostID(post.Id)
		require.NoError(t, err)
		assert.Equal(t, messageID, postInfo.MSTeamsID)
	})

	t.Run("sent chat, in reply to unknown parent", func(t *testing.T) {
		t.Skip("not yet implemented")
	})

	t.Run("sent chat, in reply", func(t *testing.T) {
		t.Skip("not yet implemented")
	})

	t.Run("sent chat, with failed attachment upload", func(t *testing.T) {
		t.Skip("not yet implemented")
	})

	t.Run("sent chat, with attachment", func(t *testing.T) {
		t.Skip("not yet implemented")
	})
}

func TestSend(t *testing.T) {
	for _, test := range []struct {
		Name            string
		SetupPlugin     func(*Plugin)
		SetupAPI        func(*plugintest.API)
		SetupStore      func(*storemocks.Store)
		SetupClient     func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics    func(mockmetrics *metricsmocks.Metrics)
		ExpectedMessage string
		ExpectedError   string
	}{
		{
			Name: "Send: Unable to get the client",
			SetupPlugin: func(p *Plugin) {
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", testutils.GetUserID(), &model.Post{
					UserId:    "bot-user-id",
					ChannelId: testutils.GetChannelID(),
					Message:   "Some users in this conversation rely on Microsoft Teams to receive your messages, but your account isn't connected. [Click here to connect your account](/plugins/com.mattermost.msteams-sync/connect).",
				}).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Send: Unable to get the file info",
			SetupPlugin: func(p *Plugin) {
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get file attachment")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n<abbr title=\"generated-from-mattermost\"></abbr>", ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to get file attachment from Mattermost",
			SetupPlugin: func(p *Plugin) {
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the file attachment from Mattermost")).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n<abbr title=\"generated-from-mattermost\"></abbr>", ([]*clientmodels.Attachment)(nil), []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, metrics.DiscardedReasonUnableToGetMMData, false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Unable to send message with attachments",
			SetupPlugin: func(p *Plugin) {
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n<abbr title=\"generated-from-mattermost\"></abbr>", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to send message with attachments")).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to send message with attachments",
		},
		{
			Name: "Send: Able to send message with attachments but unable to store posts",
			SetupPlugin: func(p *Plugin) {
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(errors.New("unable to store posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n<abbr title=\"generated-from-mattermost\"></abbr>", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
		{
			Name: "Send: Able to send message with attachments with no error",
			SetupPlugin: func(p *Plugin) {
			},
			SetupAPI: func(api *plugintest.API) {
				api.On("GetFileInfo", testutils.GetID()).Return(testutils.GetFileInfo(), nil).Times(1)
				api.On("GetFile", testutils.GetID()).Return([]byte("mockData"), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsID:      "mockMessageID",
					MSTeamsChannel: testutils.GetChannelID(),
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UploadFile", testutils.GetID(), testutils.GetChannelID(), "mockFile.Name"+"_"+testutils.GetID()+".txt", 1, "mockMimeType", bytes.NewReader([]byte("mockData")), (*clientmodels.Chat)(nil)).Return(&clientmodels.Attachment{
					ID: testutils.GetID(),
				}, nil).Times(1)
				uclient.On("SendMessageWithAttachments", testutils.GetID(), testutils.GetChannelID(), "", "<p>mockMessage??????????</p>\n<abbr title=\"generated-from-mattermost\"></abbr>", []*clientmodels.Attachment{
					{
						ID: testutils.GetID(),
					},
				}, []models.ChatMessageMentionable{}).Return(&clientmodels.Message{
					ID: "mockMessageID",
				}, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMattermost, "", false).Times(1)
				mockmetrics.On("ObserveMessage", metrics.ActionCreated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMessageDelay", metrics.ActionCreated, metrics.ActionSourceMattermost, false, mock.AnythingOfType("time.Duration")).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UploadFile", "true", "2XX", mock.AnythingOfType("float64")).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.SendMessageWithAttachments", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			ExpectedMessage: "mockMessageID",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupPlugin(p)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			resp, err := p.Send(testutils.GetID(), testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), mockPost)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}

			assert.Equal(resp, test.ExpectedMessage)
		})
	}
}

func TestDelete(t *testing.T) {
	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics  func(mockmetrics *metricsmocks.Metrics)
		ExpectedError string
	}{
		{
			Name:     "Delete: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Delete: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(metrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Delete: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(metrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Delete: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name:     "Delete: Valid",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("DeleteMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID").Return(nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
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
			err := p.Delete("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.SystemAdminRoleId, "test@test.com"), testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

var (
	anyString      = mock.AnythingOfType("string")
	anyStringSlice = mock.AnythingOfType("[]string")
	anyInt         = mock.AnythingOfType("int")
	anyPost        = mock.AnythingOfType("*model.Post")
	anyFloat64     = mock.AnythingOfType("float64")
)

func TestDeleteChat(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}

	for _, test := range []struct {
		Name          string
		SetupAPI      func(*plugintest.API)
		SetupStore    func(*storemocks.Store)
		SetupClient   func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics  func(mockmetrics *metricsmocks.Metrics)
		ExpectedError string
	}{
		{
			Name: "DeleteChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {
				api.On("SendEphemeralPost", anyString, anyPost).Return(&model.Post{}).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil).Times(3)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "DeleteChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", anyString).Return(testutils.GetUserID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
			},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "DeleteChat: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
			},
			ExpectedError: "post not found",
		},
		{
			Name: "DeleteChat: Unable to delete the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
				uclient.On("DeleteChatMessage", anyString, anyString, "mockMSTeamsID").Return(errors.New("unable to delete the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "false", "0", anyFloat64).Once()
			},
			ExpectedError: "unable to delete the message",
		},
		{
			Name: "DeleteChat: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", anyString).Return(testutils.GetChannel(model.ChannelTypeDirect), nil).Times(1)
				api.On("GetChannelMembers", anyString, anyInt, anyInt).Return(testutils.GetChannelMembers(10), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetID(), nil)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("CreateOrGetChatForUsers", anyStringSlice).Return(mockChat, nil).Times(1)
				uclient.On("DeleteChatMessage", anyString, anyString, "mockMSTeamsID").Return(nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.CreateOrGetChatForUsers", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", anyFloat64).Once()
				mockmetrics.On("ObserveMessage", metrics.ActionDeleted, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.DeleteChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
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
			err := p.DeleteChat(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()))
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdate(t *testing.T) {
	mockChannelMessage := &clientmodels.Message{
		ID:        "mockMSTeamsID",
		TeamID:    "mockTeamsTeamID",
		ChannelID: testutils.GetChannelID(),
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics   func(mockmetrics *metricsmocks.Metrics)
		ExpectedError  string
		UpdateRequired bool
	}{
		{
			Name:     "Update: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "Update: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "Update: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name: "Update: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError:  "unable to update the message",
			UpdateRequired: true,
		},
		{
			Name: "Update: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("GetMessage", "mockTeamsTeamID", testutils.GetChannelID(), "mockMSTeamsID").Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "Update: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(errors.New("unable to store the link posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "Update: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("KVSetWithOptions", "mutex_post_mutex_"+testutils.GetID(), mock.Anything, mock.Anything).Return(true, nil).Times(2)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: testutils.GetChannelID(),
					MSTeamsID:      "mockMSTeamsID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateMessage", "mockTeamsTeamID", testutils.GetChannelID(), "", "mockMSTeamsID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChannelMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, false).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			err := p.Update("mockTeamsTeamID", testutils.GetChannelID(), testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, test.UpdateRequired)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestUpdateChat(t *testing.T) {
	mockChatMessage := &clientmodels.Message{
		ID:     "mockChatID",
		ChatID: "mockTeamsTeamID",
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client, *clientmocks.Client)
		SetupMetrics   func(mockmetrics *metricsmocks.Metrics)
		ExpectedError  string
		UpdateRequired bool
	}{
		{
			Name: "UpdateChat: Unable to get the post info",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, testutils.GetInternalServerAppError("unable to get the post info")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "unable to get the post info",
		},
		{
			Name: "UpdateChat: Post info is nil",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(nil, nil).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "post not found",
		},
		{
			Name:     "UpdateChat: Unable to get the client",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockMSTeamsID",
				}, nil).Times(1)
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(nil, nil).Times(1)
				store.On("GetTokenForMattermostUser", "bot-user-id").Return(nil, nil).Times(1)
			},

			SetupClient:   func(client *clientmocks.Client, uclient *clientmocks.Client) {},
			SetupMetrics:  func(mockmetrics *metricsmocks.Metrics) {},
			ExpectedError: "not connected user",
		},
		{
			Name: "UpdateChat: Unable to update the message",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(nil, errors.New("unable to update the message")).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "false", "0", mock.AnythingOfType("float64")).Once()
			},
			ExpectedError:  "unable to update the message",
			UpdateRequired: true,
		},
		{
			Name:     "UpdateChat: Update not required on MS Teams",
			SetupAPI: func(api *plugintest.API) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("GetChatMessage", "mockChatID", "mockTeamsTeamID").Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(metrics *metricsmocks.Metrics) {
				metrics.On("ObserveMSGraphClientMethodDuration", "Client.GetChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
		},
		{
			Name: "UpdateChat: Unable to store the link posts",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(errors.New("unable to store the link posts")).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
		{
			Name: "UpdateChat: Valid",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("GetTokenForMattermostUser", testutils.GetID()).Return(&fakeToken, nil).Times(1)
				store.On("GetPostInfoByMattermostID", testutils.GetID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetID(),
					MSTeamsID:    "mockTeamsTeamID",
				}, nil).Times(1)
				store.On("LinkPosts", storemodels.PostInfo{
					MattermostID:   testutils.GetID(),
					MSTeamsChannel: "mockChatID",
					MSTeamsID:      "mockTeamsTeamID",
				}).Return(nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client, uclient *clientmocks.Client) {
				uclient.On("UpdateChatMessage", "mockChatID", "mockTeamsTeamID", "<p>mockMessage??????????</p>\n", []models.ChatMessageMentionable{}).Return(mockChatMessage, nil).Times(1)
			},
			SetupMetrics: func(mockmetrics *metricsmocks.Metrics) {
				mockmetrics.On("ObserveMessage", metrics.ActionUpdated, metrics.ActionSourceMattermost, true).Times(1)
				mockmetrics.On("ObserveMSGraphClientMethodDuration", "Client.UpdateChatMessage", "true", "2XX", mock.AnythingOfType("float64")).Once()
			},
			UpdateRequired: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			test.SetupClient(p.msteamsAppClient.(*clientmocks.Client), p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client))
			test.SetupMetrics(p.metricsService.(*metricsmocks.Metrics))
			mockPost := testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro())
			mockPost.Message = "mockMessage??????????"
			err := p.UpdateChat("mockChatID", testutils.GetUser(model.ChannelAdminRoleId, "test@test.com"), mockPost, test.UpdateRequired)
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
			} else {
				assert.Nil(err)
			}
		})
	}
}

func TestGetChatIDForChannel(t *testing.T) {
	mockChat := &clientmodels.Chat{
		ID: testutils.GetChatID(),
		Members: []clientmodels.ChatMember{
			{
				DisplayName: "mockDisplayName",
				UserID:      testutils.GetTeamsUserID(),
				Email:       testutils.GetTestEmail(),
			},
		},
	}
	for _, test := range []struct {
		Name           string
		SetupAPI       func(*plugintest.API)
		SetupStore     func(*storemocks.Store)
		SetupClient    func(*clientmocks.Client)
		ExpectedError  string
		ExpectedResult string
	}{
		{
			Name: "GetChatIDForChannel: Unable to get the channel",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(nil, testutils.GetInternalServerAppError("unable to get the channel")).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to get the channel",
		},
		{
			Name: "GetChatIDForChannel: Channel type is 'open'",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeOpen), nil).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "invalid channel type, chatID is only available for direct messages and group messages",
		},
		{
			Name: "GetChatIDForChannel: Unable to get the channel members",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(nil, testutils.GetInternalServerAppError("unable to get the channel members")).Times(1)
			},
			SetupStore:    func(store *storemocks.Store) {},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to get the channel members",
		},
		{
			Name: "GetChatIDForChannel: Unable to store users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("unable to store the user")).Times(1)
			},
			SetupClient:   func(client *clientmocks.Client) {},
			ExpectedError: "unable to store the user",
		},
		{
			Name: "GetChatIDForChannel: Unable to create or get chat for users",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return(nil, errors.New("unable to create or get chat for users")).Times(1)
			},
			ExpectedError: "unable to create or get chat for users",
		},
		{
			Name: "GetChatIDForChannel: Valid",
			SetupAPI: func(api *plugintest.API) {
				api.On("GetChannel", testutils.GetChannelID()).Return(testutils.GetChannel(model.ChannelTypeGroup), nil).Times(1)
				api.On("GetChannelMembers", testutils.GetChannelID(), 0, math.MaxInt32).Return(testutils.GetChannelMembers(2), nil).Times(1)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("mockTeamsUserID", nil).Times(2)
				store.On("GetTokenForMattermostUser", "mockClientUserID").Return(&fakeToken, nil).Times(1)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("CreateOrGetChatForUsers", []string{"mockTeamsUserID", "mockTeamsUserID"}).Return(mockChat, nil).Times(1)
			},
			ExpectedResult: testutils.GetChatID(),
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))
			client := p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client)
			test.SetupClient(client)
			resp, err := p.GetChatIDForChannel(client, testutils.GetChannelID())
			if test.ExpectedError != "" {
				assert.Contains(err.Error(), test.ExpectedError)
				assert.Equal(resp, "")
			} else {
				assert.Nil(err)
				assert.Equal(resp, test.ExpectedResult)
			}
		})
	}
}

func TestGetMentionsData(t *testing.T) {
	for _, test := range []struct {
		Name                  string
		Message               string
		ChatID                string
		SetupAPI              func(*plugintest.API)
		SetupStore            func(*storemocks.Store)
		SetupClient           func(*clientmocks.Client)
		ExpectedMessage       string
		ExpectedMentionsCount int
	}{
		{
			Name:            "GetMentionsData: mentioned in direct chat message",
			Message:         "Hi @all",
			ExpectedMessage: "Hi @all",
			ChatID:          testutils.GetChatID(),
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{}, nil)
			},
			SetupStore: func(store *storemocks.Store) {},
		},
		{
			Name:            "GetMentionsData: mentioned all in a group chat message",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">Everyone</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Type: "G",
				}, nil)
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: error occurred while getting chat",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">@all</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(nil, errors.New("error occurred while getting chat"))
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned all in Teams channel",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">mock-name</at>",
			SetupAPI:        func(api *plugintest.API) {},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChannelInTeam", testutils.GetTeamID(), testutils.GetChannelID()).Return(&clientmodels.Channel{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: error occurred while getting the MS Teams channel",
			Message:         "Hi @all",
			ExpectedMessage: "Hi <at id=\"0\">@all</at>",
			SetupAPI: func(api *plugintest.API) {
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChannelInTeam", testutils.GetTeamID(), testutils.GetChannelID()).Return(nil, errors.New("error occurred while getting the MS Teams channel"))
			},
			SetupStore:            func(store *storemocks.Store) {},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned a user",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi <at id=\"0\">mock-name</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(&clientmodels.User{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
			ExpectedMentionsCount: 1,
		},
		{
			Name:            "GetMentionsData: mentioned all and a specific user in a group chat",
			Message:         "Hi @all @test-username",
			ExpectedMessage: "Hi <at id=\"0\">Everyone</at> <at id=\"1\">mock-name</at>",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{
					Type: "G",
				}, nil)
				client.On("GetUser", testutils.GetUserID()).Return(&clientmodels.User{
					DisplayName: "mock-name",
				}, nil)
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
			ExpectedMentionsCount: 2,
		},
		{
			Name:            "GetMentionsData: error getting MM user with username",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(nil, testutils.GetInternalServerAppError("error getting MM user with username"))
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
		},
		{
			Name:            "GetMentionsData: error getting msteams user ID from MM user ID",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return("", errors.New("error getting msteams user ID from MM user ID"))
			},
		},
		{
			Name:            "GetMentionsData: error getting msteams user",
			Message:         "Hi @test-username",
			ExpectedMessage: "Hi @test-username",
			ChatID:          testutils.GetChatID(),
			SetupAPI: func(api *plugintest.API) {
				api.On("GetUserByUsername", "test-username").Return(testutils.GetUser("mock-role", "mock-email"), nil)
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetUser", testutils.GetUserID()).Return(nil, errors.New("error getting msteams user"))
			},
			SetupStore: func(store *storemocks.Store) {
				store.On("MattermostToTeamsUserID", testutils.GetID()).Return(testutils.GetUserID(), nil)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))

			client := p.msteamsAppClient.(*clientmocks.Client)
			test.SetupClient(client)

			msg, mentions := p.getMentionsData(test.Message, testutils.GetTeamID(), testutils.GetChannelID(), test.ChatID, client)
			assert.Equal(test.ExpectedMessage, msg)
			assert.Equal(test.ExpectedMentionsCount, len(mentions))
		})
	}
}

func TestUserWillLogin(t *testing.T) {
	for _, test := range []struct {
		Name                               string
		User                               *model.User
		UserIsRemote                       bool // the remote id is set by the plugin during the test
		AutomaticallyPromoteSyntheticUsers bool
		SetupAPI                           func(*plugintest.API)
		SetupStore                         func(*storemocks.Store)
		SetupClient                        func(*clientmocks.Client)
		Result                             string
	}{
		{
			Name: "Autopromotion works",
			User: &model.User{
				Id: testutils.GetID(),
			},
			UserIsRemote:                       true,
			AutomaticallyPromoteSyntheticUsers: true,
			SetupAPI: func(api *plugintest.API) {
				api.On("UpdateUser", mock.MatchedBy(func(user *model.User) bool {
					return !user.IsRemote()
				})).Once().Return(nil, nil)
				api.On("LogInfo", "Promoted synthetic user", "user_id", testutils.GetID()).Once()
			},
			SetupClient: func(client *clientmocks.Client) {
				client.On("GetChat", testutils.GetChatID()).Return(&clientmodels.Chat{}, nil)
			},
			SetupStore: func(store *storemocks.Store) {},
			Result:     "",
		},
		{
			Name: "UpdateUser failed during autopromotion",
			User: &model.User{
				Id: testutils.GetID(),
			},
			UserIsRemote:                       true,
			AutomaticallyPromoteSyntheticUsers: true,
			SetupAPI: func(api *plugintest.API) {
				api.On("UpdateUser", mock.MatchedBy(func(user *model.User) bool {
					return !user.IsRemote()
				})).Once().Return(nil, model.NewAppError("UpdateUser", "err from test", nil, "", 500))
				api.On("LogWarn", "Failed to promote synthetic user", "user_id", testutils.GetID(), "err", "err from test").Once()
			},
			SetupClient: func(client *clientmocks.Client) {},
			SetupStore:  func(store *storemocks.Store) {},
			Result:      "Unable to promote synthetic user",
		},
		{
			Name: "No autopromotion",
			User: &model.User{
				Id: testutils.GetID(),
			},
			UserIsRemote:                       true,
			AutomaticallyPromoteSyntheticUsers: false,
			SetupAPI:                           func(api *plugintest.API) {},
			SetupClient:                        func(client *clientmocks.Client) {},
			SetupStore:                         func(store *storemocks.Store) {},
			Result:                             "",
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			cfg := p.configuration
			cfg.AutomaticallyPromoteSyntheticUsers = test.AutomaticallyPromoteSyntheticUsers

			test.SetupAPI(p.API.(*plugintest.API))
			test.SetupStore(p.store.(*storemocks.Store))

			client := p.msteamsAppClient.(*clientmocks.Client)
			test.SetupClient(client)

			user := test.User
			if test.UserIsRemote {
				user.RemoteId = &p.remoteID
			}

			res := p.UserWillLogIn(nil, user)

			assert.Equal(test.Result, res)
		})
	}
}
