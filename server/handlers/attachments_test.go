package handlers

import (
	"errors"
	"testing"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/store/storemodels"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
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
		setupAPI       func(api *plugintest.API)
	}{
		{
			description: "Successfully handled code snippet",
			userID:      testutils.GetUserID(),
			attach: msteams.Attachment{
				Content: `{"language": "go", "codeSnippetUrl": "https://example.com/go/snippet/1/2/3/4/5/6/7/8"}`,
			},
			text:           "mock-data",
			expectedOutput: "mock-data\n```go\nsnippet content\n```\n",
			setupPlugin:    func(p *mocksPlugin.PluginIface) {},
			setupClient: func() {
				client.On("GetCodeSnippet", "https://example.com/go/snippet/1/2/3/4/5/6/7/8").Return("snippet content", nil)
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
			setupClient: func() {},
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
			setupClient: func() {},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "codesnippetURL has unexpected size", "URL", "https://example.com/go/snippet").Return().Once()
			},
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
				p.On("GetAPI").Return(mockAPI)
			},
			setupClient: func() {
				client.On("GetCodeSnippet", "https://mock-example.com/go/snippet/1/2/3/4/5/6/7/8").Return("", errors.New("Error while retrieving code snippet"))
			},
			setupAPI: func(api *plugintest.API) {
				api.On("LogError", "retrieving snippet content failed", "error", errors.New("Error while retrieving code snippet"))
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			testCase.setupClient()
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
