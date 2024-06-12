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
)

func TestHandleCodeSnippet(t *testing.T) {
	for _, testCase := range []struct {
		description    string
		userID         string
		attach         clientmodels.Attachment
		text           string
		expectedOutput string
		setupClient    func(client *clientmocks.Client)
		setupAPI       func(api *plugintest.API)
	}{
		{
			description: "Successfully handled code snippet for channel",
			userID:      testutils.GetUserID(),
			attach: clientmodels.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data\n```go\nsnippet content\n```\n",
			setupClient: func(client *clientmocks.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)
			},
			setupAPI: func(api *plugintest.API) {},
		},
		{
			description: "Successfully handled code snippet for chat",
			userID:      testutils.GetUserID(),
			attach: clientmodels.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data\n```go\nsnippet content\n```\n",
			setupClient: func(client *clientmocks.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)
			},
			setupAPI: func(api *plugintest.API) {},
		},
		{
			description: "Unable to unmarshal codesnippet",
			userID:      testutils.GetUserID(),
			attach: clientmodels.Attachment{
				Content: "Invalid JSON",
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupClient:    func(client *clientmocks.Client) {},
			setupAPI: func(api *plugintest.API) {
			},
		},
		{
			description: "CodesnippetUrl has unexpected size",
			userID:      testutils.GetUserID(),
			attach: clientmodels.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/go/snippet"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupClient:    func(client *clientmocks.Client) {},
			setupAPI: func(api *plugintest.API) {
			},
		},
		{
			description: "Unable to retrieve code snippet",
			userID:      testutils.GetUserID(),
			attach: clientmodels.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupClient: func(client *clientmocks.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("", errors.New("Error while retrieving code snippet"))
			},
			setupAPI: func(api *plugintest.API) {
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			client := p.clientBuilderWithToken("", "", "", "", nil, nil).(*clientmocks.Client)
			testCase.setupClient(client)
			testCase.setupAPI(p.API.(*plugintest.API))
			testutils.MockLogs(p.API.(*plugintest.API))

			ah := ActivityHandler{}
			ah.plugin = p

			resp := ah.handleCodeSnippet(client, testCase.attach, testCase.text)
			assert.Equal(t, resp, testCase.expectedOutput)
		})
	}
}

func TestHandleMessageReference(t *testing.T) {
	for _, testCase := range []struct {
		description      string
		attach           clientmodels.Attachment
		chatOrChannelID  string
		text             string
		expectedText     string
		expectedParentID string
		setupStore       func(store *storemocks.Store)
		setupAPI         func(api *plugintest.API)
	}{
		{
			description: "Successfully got postID and text",
			attach: clientmodels.Attachment{
				Content: `{"messageId": "dsdfonreoapwer4onebfdr"}`,
			},
			chatOrChannelID:  testutils.GetChannelID(),
			text:             "mock-data",
			expectedText:     "mock-data",
			expectedParentID: testutils.GetID(),
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetMattermostID(),
				}, nil)
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetPost", testutils.GetMattermostID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID(), time.Now().UnixMicro()), nil)
			},
		},
		{
			description: "Unable to unmarshal content",
			attach: clientmodels.Attachment{
				Content: "Invalid JSON",
			},
			text:         "mock-data",
			expectedText: "mock-data",
			setupStore:   func(store *storemocks.Store) {},
			setupAPI:     func(api *plugintest.API) {},
		},
		{
			description: "Unable to get post info by msteam ID",
			attach: clientmodels.Attachment{
				Content: `{"messageId": "dsdfonreoapwer4onebfdr"}`,
			},
			chatOrChannelID: "mock-chatOrChannelID",
			text:            "mock-data",
			expectedText:    "mock-data",
			setupStore: func(store *storemocks.Store) {
				store.On("GetPostInfoByMSTeamsID", "mock-chatOrChannelID", testutils.GetMessageID()).Return(nil, errors.New("Error while getting post info by msteam ID"))
			},
			setupAPI: func(api *plugintest.API) {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			testCase.setupAPI(p.API.(*plugintest.API))
			testutils.MockLogs(p.API.(*plugintest.API))
			testCase.setupStore(p.store.(*storemocks.Store))
			ah := ActivityHandler{}
			ah.plugin = p

			parentID, text := ah.handleMessageReference(testCase.attach, testCase.chatOrChannelID, testCase.text)
			assert.Equal(t, text, testCase.expectedText)
			assert.Equal(t, parentID, testCase.expectedParentID)
		})
	}
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
