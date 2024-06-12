package main

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/metrics"
	mocksMetrics "github.com/mattermost/mattermost-plugin-msteams/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	clientmocks "github.com/mattermost/mattermost-plugin-msteams/server/msteams/mocks"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCodeSnippet(t *testing.T) {
	th := setupTestHelper(t)

	t.Run("failed to marshal codesnippet", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     "Invalid JSON",
		}
		message := "message"

		expectedOutput := "message"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("url is unexpected", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/go/snippet"}`,
		}
		message := "message"

		expectedOutput := "message"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("failure retrieving code snippet", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
		}
		message := "message"

		th.appClientMock.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("", errors.New("Error while retrieving code snippet"))

		expectedOutput := "message"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("code snippet for channel", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
		}
		message := "message"

		th.appClientMock.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)

		expectedOutput := "message\n```go\nsnippet content\n```\n"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})

	t.Run("code snippet for chat", func(t *testing.T) {
		th.Reset(t)

		attachment := clientmodels.Attachment{
			ContentType: "application/vnd.microsoft.card.codesnippet",
			Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
		}
		message := "message"

		th.appClientMock.On("GetCodeSnippet", "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)

		expectedOutput := "message\n```go\nsnippet content\n```\n"
		actualOutput := th.p.activityHandler.handleCodeSnippet(th.appClientMock, attachment, message)
		assert.Equal(t, actualOutput, expectedOutput)
	})
}

func TestHandleMessageReference(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("unable to marshal content", func(t *testing.T) {
		chatOrChannelID := model.NewId()

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     "Invalid JSON",
		}
		text := "message"

		expectedText := "message"
		expectedParentID := ""

		actualParentID, actualText := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID, text)
		assert.Equal(t, expectedParentID, actualParentID)
		assert.Equal(t, expectedText, actualText)
	})

	t.Run("unknown message", func(t *testing.T) {
		messageID := model.NewId()
		chatOrChannelID := model.NewId()

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     `{"messageId": "` + messageID + `"}`,
		}
		text := "message"

		expectedText := "message"
		expectedParentID := ""

		actualParentID, actualText := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID, text)
		assert.Equal(t, expectedParentID, actualParentID)
		assert.Equal(t, expectedText, actualText)
	})

	t.Run("successful lookup, no parent", func(t *testing.T) {
		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team)

		messageID := model.NewId()
		chatOrChannelID := model.NewId()

		post := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
		}
		err := th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		err = th.p.GetStore().LinkPosts(storemodels.PostInfo{
			MattermostID:        post.Id,
			MSTeamsID:           messageID,
			MSTeamsChannel:      chatOrChannelID,
			MSTeamsLastUpdateAt: time.Now(),
		})
		require.NoError(t, err)

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     `{"messageId": "` + messageID + `"}`,
		}
		text := "message"

		expectedText := "message"
		expectedParentID := post.Id

		actualParentID, actualText := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID, text)
		assert.Equal(t, expectedParentID, actualParentID)
		assert.Equal(t, expectedText, actualText)
	})

	t.Run("successful lookup, with parent", func(t *testing.T) {
		user := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team)

		messageID := model.NewId()
		chatOrChannelID := model.NewId()

		rootPost := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
		}
		err := th.p.apiClient.Post.CreatePost(rootPost)
		require.NoError(t, err)

		post := &model.Post{
			UserId:    user.Id,
			ChannelId: channel.Id,
			Message:   "post message",
			RootId:    rootPost.Id,
		}
		err = th.p.apiClient.Post.CreatePost(post)
		require.NoError(t, err)

		// Simulate the post having originated from Mattermost. Later, we'll let the code
		// do this itself once.
		err = th.p.GetStore().LinkPosts(storemodels.PostInfo{
			MattermostID:        post.Id,
			MSTeamsID:           messageID,
			MSTeamsChannel:      chatOrChannelID,
			MSTeamsLastUpdateAt: time.Now(),
		})
		require.NoError(t, err)

		attachment := clientmodels.Attachment{
			ContentType: "messageReference",
			Content:     `{"messageId": "` + messageID + `"}`,
		}
		text := "message"

		expectedText := "message"
		expectedParentID := rootPost.Id

		actualParentID, actualText := th.p.activityHandler.handleMessageReference(attachment, chatOrChannelID, text)
		assert.Equal(t, expectedParentID, actualParentID)
		assert.Equal(t, expectedText, actualText)
	})
}

func TestHandleAttachments(t *testing.T) {
	for _, testCase := range []struct {
		description                    string
		setupAPI                       func(mockAPI *plugintest.API)
		setupClient                    func(*clientmocks.Client)
		setupMetrics                   func(*mocksMetrics.Metrics)
		setupStore                     func(store *storemocks.Store)
		attachments                    []clientmodels.Attachment
		expectedText                   string
		expectedAttachmentIDsCount     int
		expectedParentID               string
		expectedSkippedFileAttachments int
		expectedError                  bool
		fileIDs                        []string
	}{
		{
			description: "Successfully handled attachments",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(&model.FileInfo{
					Id: testutils.GetID(),
				}, nil)
			},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Once()
				client.On("GetFileContent", "mockDownloadURL").Return([]byte{}, nil).Once()
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMSTeams, "", false).Times(1)
			},
			setupStore: func(store *storemocks.Store) {},
			attachments: []clientmodels.Attachment{
				{
					Name: "mock-name",
				},
			},
			expectedText:               "mock-text",
			expectedAttachmentIDsCount: 1,
		},
		{
			description: "Error uploading the file",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(nil, &model.AppError{Message: "error uploading the file"})
			},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Once()
				client.On("GetFileContent", "mockDownloadURL").Return([]byte{}, nil).Once()
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonEmptyFileID, false).Times(1)
			},
			setupStore: func(store *storemocks.Store) {},
			attachments: []clientmodels.Attachment{
				{
					Name: "mock-name",
				},
			},
			expectedText:                   "mock-text",
			expectedSkippedFileAttachments: 1,
		},
		{
			description: "Number of attachments are greater than 10",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), mock.AnythingOfType("string")).Return(&model.FileInfo{Id: testutils.GetID()}, nil).Times(10)
			},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Times(10)
				client.On("GetFileContent", "mockDownloadURL").Return([]byte{}, nil).Times(10)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMSTeams, "", false).Times(10)
				mockmetrics.On("ObserveFiles", metrics.ActionCreated, metrics.ActionSourceMSTeams, metrics.DiscardedReasonFileLimitReached, false, int64(2)).Times(1)
			},
			setupStore: func(store *storemocks.Store) {},
			attachments: []clientmodels.Attachment{
				{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
			},
			expectedText:                   "mock-text",
			expectedAttachmentIDsCount:     10,
			expectedSkippedFileAttachments: 2,
		},
		{
			description: "Attachment with existing fileID",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetFileInfo", mock.Anything).Return(testutils.GetFileInfo(), nil).Once()
			},
			setupClient:  func(client *clientmocks.Client) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
			setupStore:   func(store *storemocks.Store) {},
			attachments: []clientmodels.Attachment{
				{
					Name: "mockFile.Name.txt",
				},
			},
			expectedAttachmentIDsCount: 1,
			expectedText:               "mock-text",
			fileIDs:                    []string{"testFileId"},
		},
		{
			description: "No attachment with existing fileID",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetFileInfo", mock.Anything).Return(testutils.GetFileInfo(), nil).Once()
			},
			setupClient:                func(client *clientmocks.Client) {},
			setupMetrics:               func(mockmetrics *mocksMetrics.Metrics) {},
			setupStore:                 func(store *storemocks.Store) {},
			attachments:                []clientmodels.Attachment{},
			expectedAttachmentIDsCount: 0,
			expectedText:               "mock-text",
			fileIDs:                    []string{"testFileId"},
		},
		{
			description: "Attachment with new and existing fileID",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetFileInfo", mock.Anything).Return(testutils.GetFileInfo(), nil).Once()
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(&model.FileInfo{
					Id: testutils.GetID(),
				}, nil).Once()
			},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetFileSizeAndDownloadURL", "").Return(int64(5), "mockDownloadURL", nil).Once()
				client.On("GetFileContent", "mockDownloadURL").Return([]byte{}, nil).Once()
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {
				mockmetrics.On("ObserveFile", metrics.ActionCreated, metrics.ActionSourceMSTeams, "", false).Times(1)
			},
			setupStore: func(store *storemocks.Store) {},
			attachments: []clientmodels.Attachment{
				{
					Name: "mock-name",
				},
				{
					Name: "mockFile.Name.txt",
				},
			},
			expectedText:               "mock-text",
			expectedAttachmentIDsCount: 2,
			fileIDs:                    []string{"testFileId"},
		},
		{
			description: "Attachment type code snippet",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(&model.FileInfo{}, nil)
			},
			setupClient: func(client *clientmocks.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)
			},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
			setupStore:   func(store *storemocks.Store) {},
			attachments: []clientmodels.Attachment{
				{
					Name:        "mock-name",
					ContentType: "application/vnd.microsoft.card.codesnippet",
					Content:     `{"language": "go", "codeSnippetUrl": "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
				},
			},
			expectedText: "mock-text\n```go\nsnippet content\n```\n",
		},
		{
			description: "Attachment type message reference",
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(&model.FileInfo{}, nil)
				mockAPI.On("GetPost", testutils.GetUserID()).Return(&model.Post{
					Id: testutils.GetID(),
				}, nil)
			},
			setupClient:  func(client *clientmocks.Client) {},
			setupMetrics: func(mockmetrics *mocksMetrics.Metrics) {},
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", fmt.Sprintf("%s%s", testutils.GetChatID(), testutils.GetChannelID()), "mock-ID").Return(&storemodels.PostInfo{
					MattermostID: testutils.GetUserID(),
				}, nil)
			},
			attachments: []clientmodels.Attachment{{
				Name:        "mock-name",
				ContentType: "messageReference",
				Content:     `{"messageId":"mock-ID"}`,
			}},
			expectedText:     "mock-text",
			expectedParentID: testutils.GetID(),
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)

			testCase.setupAPI(p.API.(*plugintest.API))
			testutils.MockLogs(p.API.(*plugintest.API))
			testCase.setupStore(p.store.(*storemocks.Store))
			testCase.setupClient(p.msteamsAppClient.(*clientmocks.Client))
			testCase.setupMetrics(p.metricsService.(*mocksMetrics.Metrics))

			ah := ActivityHandler{}
			ah.plugin = p

			attachments := &clientmodels.Message{
				Attachments: testCase.attachments,
				ChatID:      testutils.GetChatID(),
				ChannelID:   testutils.GetChannelID(),
			}

			newText, attachmentIDs, parentID, skippedFileAttachments, errorsFound := ah.handleAttachments(testutils.GetChannelID(), testutils.GetUserID(), "mock-text", attachments, nil, testCase.fileIDs)
			assert.Equal(t, testCase.expectedParentID, parentID)
			assert.Equal(t, testCase.expectedAttachmentIDsCount, len(attachmentIDs))
			assert.Equal(t, testCase.expectedText, newText)
			assert.Equal(t, testCase.expectedSkippedFileAttachments, skippedFileAttachments)
			assert.Equal(t, testCase.expectedError, errorsFound)
		})
	}
}
