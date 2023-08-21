package handlers

import (
	"errors"
	"fmt"
	"testing"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest/mock"
	"github.com/stretchr/testify/assert"
)

func TestHandleDownloadFile(t *testing.T) {
	ah := ActivityHandler{}
	client := mocksClient.NewClient(t)

	for _, testCase := range []struct {
		description   string
		userID        string
		weburl        string
		expectedError string
		mockChat      *msteams.Chat
		setupClient   func()
	}{
		{
			description: "Successfully file downloaded for channel",
			userID:      testutils.GetUserID(),
			weburl:      "https://example.com/file1.txt",
			setupClient: func() {
				client.On("GetFileContent", "https://example.com/file1.txt").Return([]byte("data"), nil)
			},
		},
		{
			description: "Successfully file downloaded for chat",
			userID:      testutils.GetUserID(),
			weburl:      "https://example.com/file1.txt",
			mockChat: &msteams.Chat{
				Members: []msteams.ChatMember{
					{UserID: testutils.GetTeamsUserID()},
				},
			},
			setupClient: func() {
				client.On("GetFileContent", "https://example.com/file1.txt").Return([]byte("data"), nil)
			},
		},
		{
			description:   "Unable to get file content",
			userID:        testutils.GetUserID(),
			expectedError: "Error while getting file content",
			setupClient: func() {
				client.On("GetFileContent", "").Return(nil, errors.New("Error while getting file content"))
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupClient()
			ah.plugin = p

			data, err := ah.handleDownloadFile(testCase.weburl, client)
			if testCase.expectedError != "" {
				assert.Nil(t, data)
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestHandleCodeSnippet(t *testing.T) {
	ah := ActivityHandler{}
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description    string
		userID         string
		attach         msteams.Attachment
		text           string
		expectedOutput string
		setupPlugin    func(plugin *mocksPlugin.PluginIface)
		setupClient    func(client *mocksClient.Client)
		setupAPI       func(api *plugintest.API)
	}{
		{
			description: "Successfully handled code snippet for channel",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data\n```go\nsnippet content\n```\n",
			setupPlugin:    func(p *mocksPlugin.PluginIface) {},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)
			},
			setupAPI: func(api *plugintest.API) {},
		},
		{
			description: "Successfully handled code snippet for chat",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data\n```go\nsnippet content\n```\n",
			setupPlugin:    func(p *mocksPlugin.PluginIface) {},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)
			},
			setupAPI: func(api *plugintest.API) {},
		},
		{
			description: "Unable to unmarshal codesnippet",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: "Invalid JSON",
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "failed to unmarshal codesnippet", "error", "invalid character 'I' looking for beginning of value").Return().Once()
			},
		},
		{
			description: "CodesnippetUrl has unexpected size",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/go/snippet"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func(client *mocksClient.Client) {},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "invalid codesnippetURL", "URL", "https://example.com/go/snippet").Return().Once()
			},
		},
		{
			description: "Unable to retrieve code snippet",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/teams/mock-team-id/channels/mock-channel-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("", errors.New("Error while retrieving code snippet"))
			},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "retrieving snippet content failed", "error", errors.New("Error while retrieving code snippet"))
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			client := mocksClient.NewClient(t)
			testCase.setupClient(client)
			testCase.setupAPI(mockAPI)

			ah.plugin = p

			resp := ah.handleCodeSnippet(client, testCase.attach, testCase.text)
			assert.Equal(t, resp, testCase.expectedOutput)
		})
	}
}

func TestHandleMessageReference(t *testing.T) {
	ah := ActivityHandler{}
	mockAPI := &plugintest.API{}
	store := mocksStore.NewStore(t)

	for _, testCase := range []struct {
		description      string
		attach           msteams.Attachment
		chatOrChannelID  string
		text             string
		expectedText     string
		expectedParentID string
		setupPlugin      func(plugin *mocksPlugin.PluginIface)
		setupStore       func()
		setupAPI         func()
	}{
		{
			description: "Successfully got postID and text",
			attach: msteams.Attachment{
				Content: `{"messageId": "dsdfonreoapwer4onebfdr"}`,
			},
			chatOrChannelID:  testutils.GetChannelID(),
			text:             "mock-data",
			expectedText:     "mock-data",
			expectedParentID: testutils.GetID(),
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				p.On("GetAPI").Return(mockAPI)
			},
			setupStore: func() {
				store.On("GetPostInfoByMSTeamsID", testutils.GetChannelID(), testutils.GetMessageID()).Return(&storemodels.PostInfo{
					MattermostID: testutils.GetMattermostID(),
				}, nil)
			},
			setupAPI: func() {
				mockAPI.On("GetPost", testutils.GetMattermostID()).Return(testutils.GetPost(testutils.GetChannelID(), testutils.GetUserID()), nil)
			},
		},
		{
			description: "Unable to unmarshal content",
			attach: msteams.Attachment{
				Content: "Invalid JSON",
			},
			text:         "mock-data",
			expectedText: "mock-data",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetAPI").Return(mockAPI)
			},
			setupStore: func() {},
			setupAPI:   func() {},
		},
		{
			description: "Unable to get post info by msteam ID",
			attach: msteams.Attachment{
				Content: `{"messageId": "dsdfonreoapwer4onebfdr"}`,
			},
			chatOrChannelID: "mock-chatOrChannelID",
			text:            "mock-data",
			expectedText:    "mock-data",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetStore").Return(store)
				store.On("GetPostInfoByMSTeamsID", "mock-chatOrChannelID", testutils.GetMessageID()).Return(nil, errors.New("Error while getting post info by msteam ID"))
			},
			setupStore: func() {},
			setupAPI:   func() {},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything).Return()
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupAPI()
			testCase.setupStore()
			ah.plugin = p

			parentID, text := ah.handleMessageReference(testCase.attach, testCase.chatOrChannelID, testCase.text)
			assert.Equal(t, text, testCase.expectedText)
			assert.Equal(t, parentID, testCase.expectedParentID)
		})
	}
}

func TestHandleAttachments(t *testing.T) {
	for _, testCase := range []struct {
		description                string
		setupPlugin                func(plugin *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store)
		setupAPI                   func(mockAPI *plugintest.API)
		setupClient                func(*mocksClient.Client)
		attachments                []msteams.Attachment
		expectedText               string
		expectedAttachmentIDsCount int
		expectedParentID           string
	}{
		{
			description: "Successfully handled attachments",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetConfig").Return(&model.Config{
					FileSettings: model.FileSettings{
						MaxFileSize: model.NewInt64(5),
					},
				})
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(&model.FileInfo{
					Id: testutils.GetID(),
				}, nil)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetFileContent", "").Return([]byte{}, nil)
			},
			attachments: []msteams.Attachment{
				{
					Name: "mock-name",
				},
			},
			expectedText:               "mock-text",
			expectedAttachmentIDsCount: 1,
		},
		{
			description: "Client is nil",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(nil)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogError", "Unable to get the client").Return()
			},
			setupClient: func(client *mocksClient.Client) {},
			attachments: []msteams.Attachment{
				{
					Name: "mock-name",
				},
			},
		},
		{
			description: "File size is greater than the max allowed size",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetConfig").Return(&model.Config{
					FileSettings: model.FileSettings{
						MaxFileSize: model.NewInt64(-1),
					},
				})
				mockAPI.On("LogError", "cannot upload file to Mattermost as its size is greater than allowed size", "filename", "mock-name").Return()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetFileContent", "").Return([]byte{}, nil)
			},
			attachments: []msteams.Attachment{
				{
					Name: "mock-name",
				},
			},
			expectedText: "mock-text",
		},
		{
			description: "Error uploading the file",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetConfig").Return(&model.Config{
					FileSettings: model.FileSettings{
						MaxFileSize: model.NewInt64(5),
					},
				})
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(nil, &model.AppError{Message: "error uploading the file"})
				mockAPI.On("LogError", "upload file to Mattermost failed", "filename", "mock-name", "error", "error uploading the file").Return()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetFileContent", "").Return([]byte{}, nil)
			},
			attachments: []msteams.Attachment{
				{
					Name: "mock-name",
				},
			},
			expectedText: "mock-text",
		},
		{
			description: "Number of attachments are greater than 10",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetConfig").Return(&model.Config{
					FileSettings: model.FileSettings{
						MaxFileSize: model.NewInt64(5),
					},
				})
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "").Return(&model.FileInfo{}, nil)
				mockAPI.On("LogDebug", "discarding the rest of the attachments as Mattermost supports only 10 attachments per post").Return()
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetFileContent", "").Return([]byte{}, nil)
			},
			attachments: []msteams.Attachment{
				{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {},
			},
			expectedText:               "mock-text",
			expectedAttachmentIDsCount: 10,
		},
		{
			description: "Attachment type code snippet",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetConfig").Return(&model.Config{
					FileSettings: model.FileSettings{
						MaxFileSize: model.NewInt64(5),
					},
				})
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(&model.FileInfo{}, nil)
			},
			setupClient: func(client *mocksClient.Client) {
				client.On("GetCodeSnippet", "https://example.com/version/chats/mock-chat-id/messages/mock-message-id/hostedContents/mock-content-id/$value").Return("snippet content", nil)
			},
			attachments: []msteams.Attachment{
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
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForApp").Return(client)
				p.On("GetStore").Return(store, nil)
				p.On("GetAPI").Return(mockAPI)
				store.On("GetPostInfoByMSTeamsID", fmt.Sprintf("%s%s", testutils.GetChatID(), testutils.GetChannelID()), "mock-ID").Return(&storemodels.PostInfo{
					MattermostID: testutils.GetUserID(),
				}, nil)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("GetConfig").Return(&model.Config{
					FileSettings: model.FileSettings{
						MaxFileSize: model.NewInt64(5),
					},
				})
				mockAPI.On("UploadFile", []byte{}, testutils.GetChannelID(), "mock-name").Return(&model.FileInfo{}, nil)
				mockAPI.On("GetPost", testutils.GetUserID()).Return(&model.Post{
					Id: testutils.GetID(),
				}, nil)
			},
			setupClient: func(client *mocksClient.Client) {},
			attachments: []msteams.Attachment{{
				Name:        "mock-name",
				ContentType: "messageReference",
				Content:     `{"messageId":"mock-ID"}`,
			}},
			expectedText:     "mock-text",
			expectedParentID: testutils.GetID(),
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			mockAPI := &plugintest.API{}
			client := mocksClient.NewClient(t)
			store := mocksStore.NewStore(t)

			mockAPI.AssertExpectations(t)

			testCase.setupPlugin(p, mockAPI, client, store)
			testCase.setupAPI(mockAPI)
			testCase.setupClient(client)

			ah.plugin = p

			attachments := &msteams.Message{
				Attachments: testCase.attachments,
				ChatID:      testutils.GetChatID(),
				ChannelID:   testutils.GetChannelID(),
			}

			newText, attachmentIDs, parentID := ah.handleAttachments(testutils.GetChannelID(), "mock-text", attachments, nil)
			assert.Equal(t, testCase.expectedParentID, parentID)
			assert.Equal(t, testCase.expectedAttachmentIDsCount, len(attachmentIDs))
			assert.Equal(t, testCase.expectedText, newText)
		})
	}
}
