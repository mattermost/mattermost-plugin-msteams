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
		setupPlugin   func(plugin *mocksPlugin.PluginIface)
		setupClient   func()
	}{
		{
			description: "Successfully file downloaded",
			userID:      testutils.GetUserID(),
			weburl:      "https://example.com/file1.txt",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
			},
			setupClient: func() {
				client.On("GetFileContent", "https://example.com/file1.txt").Return([]byte("data"), nil)
			},
		},
		{
			description:   "Unable to get client for a user",
			userID:        "mock-userID",
			weburl:        "https://example.com/file1.txt",
			expectedError: "Error while getting the client for a user",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", "mock-userID").Return(nil, errors.New("Error while getting the client for a user"))
			},
			setupClient: func() {},
		},
		{
			description:   "Unable to get file content",
			userID:        testutils.GetUserID(),
			expectedError: "Error while getting file content",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
			},
			setupClient: func() {
				client.On("GetFileContent", "").Return(nil, errors.New("Error while getting file content"))
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
			ah.plugin = p

			data, err := ah.handleDownloadFile(testCase.userID, testCase.weburl)
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
	client := mocksClient.NewClient(t)
	mockAPI := &plugintest.API{}

	for _, testCase := range []struct {
		description    string
		userID         string
		attach         msteams.Attachment
		text           string
		expectedOutput string
		setupPlugin    func(plugin *mocksPlugin.PluginIface)
		setupClient    func()
	}{
		{
			description: "Successfully handled code snippet",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/go/snippet/1/2/3/4/5/6/7/8"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data\n```go\nsnippet content\n```\n",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
			},
			setupClient: func() {
				client.On("GetCodeSnippet", "https://example.com/go/snippet/1/2/3/4/5/6/7/8").Return("snippet content", nil)
			},
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
			setupClient: func() {},
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
			setupClient: func() {},
		},
		{
			description: "Unable to get bot client",
			userID:      "mock-userID",
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/go/snippet/1/2/3/4/5/6/7/8"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", "mock-userID").Return(nil, errors.New("Error while getting bot client"))
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {},
		},
		{
			description: "Unable while retrieving code snippet",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://mock-example.com/go/snippet/1/2/3/4/5/6/7/8"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data",
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetCodeSnippet", "https://mock-example.com/go/snippet/1/2/3/4/5/6/7/8").Return("", errors.New("Error while retrieving code snippet"))
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			mockAPI.On("LogError", mock.Anything, mock.Anything, mock.Anything).Return()
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
			ah.plugin = p

			resp := ah.handleCodeSnippet(testCase.userID, testCase.attach, testCase.text)
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
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
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
			description: "Error getting client for the user",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(nil, errors.New("error getting client for the user"))
				p.On("GetAPI").Return(mockAPI)
			},
			setupAPI: func(mockAPI *plugintest.API) {
				mockAPI.On("LogError", "file download failed", "filename", "mock-name", "error", "error getting client for the user").Return()
			},
			setupClient: func(client *mocksClient.Client) {},
			attachments: []msteams.Attachment{
				{
					Name: "mock-name",
				},
			},
			expectedText: "mock-text",
		},
		{
			description: "File size is greater than the max allowed size",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
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
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
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
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
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
				p.On("GetClientForUser", testutils.GetUserID()).Return(client, nil)
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
				client.On("GetCodeSnippet", "mock-url////////////").Return("", nil)
			},
			attachments: []msteams.Attachment{
				{
					Name:        "mock-name",
					ContentType: "application/vnd.microsoft.card.codesnippet",
					Content:     `{"language":"", "codeSnippetUrl": "mock-url////////////"}`,
				},
			},
			expectedText: "mock-text\n```\n\n```\n",
		},
		{
			description: "Attachment type message reference",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, store *mocksStore.Store) {
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

			newText, attachmentIDs, parentID := ah.handleAttachments(testutils.GetUserID(), testutils.GetChannelID(), "mock-text", attachments)
			assert.Equal(t, testCase.expectedParentID, parentID)
			assert.Equal(t, testCase.expectedAttachmentIDsCount, len(attachmentIDs))
			assert.Equal(t, testCase.expectedText, newText)
		})
	}
}
