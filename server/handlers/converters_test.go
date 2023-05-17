package handlers

import (
	"errors"
	"net/http"
	"testing"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest/mock"
	"github.com/stretchr/testify/assert"
)

type FakeHTTPTransport struct{}

func (FakeHTTPTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{}, nil
}

func TestMsgToPost(t *testing.T) {
	defaultTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = &FakeHTTPTransport{}
	defer func() {
		http.DefaultClient.Transport = defaultTransport
	}()
	ah := ActivityHandler{}
	for _, testCase := range []struct {
		description string
		channelID   string
		userID      string
		senderID    string
		message     *msteams.Message
		post        *model.Post
		setupPlugin func(plugin *mocksPlugin.PluginIface)
	}{
		{
			description: "Successfully add message to post",
			channelID:   testutils.GetChannelID(),
			userID:      testutils.GetUserID(),
			senderID:    testutils.GetSenderID(),
			message: &msteams.Message{
				Subject:         "Subject of the messsage",
				UserDisplayName: "mock-UserDisplayName",
				UserID:          testutils.GetUserID(),
			},
			setupPlugin: func(p *mocksPlugin.PluginIface) {
				p.On("GetBotUserID").Return(testutils.GetSenderID())
				p.On("GetURL").Return("https://example.com/")
			},
			post: &model.Post{
				UserId:    testutils.GetSenderID(),
				ChannelId: testutils.GetChannelID(),
				Message:   "## Subject of the messsage\n",
				Props: model.StringInterface{
					"from_webhook":                         "true",
					"msteams_sync_pqoejrn65psweomewmosaqr": true,
					"override_icon_url":                    "https://example.com//public/msteams-sync-icon.svg",
					"override_username":                    "mock-UserDisplayName",
				},
				FileIds: model.StringArray{},
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			testCase.setupPlugin(p)
			ah.plugin = p
			post, _ := ah.msgToPost(testCase.userID, testCase.channelID, testCase.message, testCase.senderID)
			assert.Equal(t, testCase.post, post)
		})
	}
}

func TestHandleMentions(t *testing.T) {
	ah := ActivityHandler{}
	for _, testCase := range []struct {
		description     string
		setupPlugin     func(*mocksPlugin.PluginIface, *plugintest.API, *mocksStore.Store)
		setupAPI        func(*plugintest.API)
		setupStore      func(*mocksStore.Store)
		message         *msteams.Message
		expectedMessage string
	}{
		{
			description: "No mentions present",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {},
			setupAPI:    func(api *plugintest.API) {},
			setupStore:  func(store *mocksStore.Store) {},
			message: &msteams.Message{
				Text: "mockMessage",
			},
			expectedMessage: "mockMessage",
		},
		{
			description: "Channel mention present",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {},
			setupAPI:    func(api *plugintest.API) {},
			setupStore:  func(store *mocksStore.Store) {},
			message: &msteams.Message{
				Text: `mockMessage <at id="0">mockMentionedText</at>`,
				Mentions: []msteams.Mention{
					{
						ID:            0,
						MentionedText: "mockMentionedText",
					},
				},
			},
			expectedMessage: "mockMessage @channel",
		},
		{
			description: "Unable to get mm user ID for user mentions",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Once()
				p.On("GetStore").Return(store).Once()
			},
			setupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Unable to get mm UserID", "Error", mock.Anything).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamUserID()).Return("", errors.New("unable to get mm user ID"))
			},
			message: &msteams.Message{
				Text: `mockMessage <at id="0">mockMentionedText</at>`,
				Mentions: []msteams.Mention{
					{
						ID:            0,
						UserID:        testutils.GetTeamUserID(),
						MentionedText: "mockMentionedText",
					},
				},
			},
			expectedMessage: `mockMessage <at id="0">mockMentionedText</at>`,
		},
		{
			description: "Unable to get mm user details for user mentions",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Twice()
				p.On("GetStore").Return(store).Once()
			},
			setupAPI: func(api *plugintest.API) {
				api.On("LogDebug", "Unable to get mm user details", "Error", mock.Anything).Once()
				api.On("GetUser", testutils.GetMattermostID()).Return(nil, testutils.GetInternalServerAppError("unable to get mm user details")).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamUserID()).Return(testutils.GetMattermostID(), nil).Once()
			},
			message: &msteams.Message{
				Text: `mockMessage <at id="0">mockMentionedText</at>`,
				Mentions: []msteams.Mention{
					{
						ID:            0,
						UserID:        testutils.GetTeamUserID(),
						MentionedText: "mockMentionedText",
					},
				},
			},
			expectedMessage: `mockMessage <at id="0">mockMentionedText</at>`,
		},
		{
			description: "Successful user mentions",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Twice()
				p.On("GetStore").Return(store).Twice()
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetUser", "mockMMUserID-1").Return(&model.User{
					Id:       "mockMMUserID-1",
					Username: "mockMMUsername-1",
				}, nil).Once()
				api.On("GetUser", "mockMMUserID-2").Return(&model.User{
					Id:       "mockMMUserID-2",
					Username: "mockMMUsername-2",
				}, nil).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", "mockMSUserID-1").Return("mockMMUserID-1", nil).Once()
				store.On("TeamsToMattermostUserID", "mockMSUserID-2").Return("mockMMUserID-2", nil).Once()
			},
			message: &msteams.Message{
				Text: `hello <at id="0">mockMSUsername-1</at> from <at id="1">mockMSUsername-2</at>`,
				Mentions: []msteams.Mention{
					{
						ID:            0,
						UserID:        "mockMSUserID-1",
						MentionedText: "mockMSUsername-1",
					},
					{
						ID:            1,
						UserID:        "mockMSUserID-2",
						MentionedText: "mockMSUsername-2",
					},
				},
			},
			expectedMessage: "hello @mockMMUsername-1  from @mockMMUsername-2 ",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			assert := assert.New(t)
			p := mocksPlugin.NewPluginIface(t)
			store := mocksStore.NewStore(t)
			mockAPI := &plugintest.API{}
			testCase.setupPlugin(p, mockAPI, store)
			testCase.setupAPI(mockAPI)
			testCase.setupStore(store)

			ah.plugin = p
			message := ah.handleMentions(testCase.message)
			assert.Equal(testCase.expectedMessage, message)
		})
	}
}

func TestConvertToMD(t *testing.T) {
	for _, testCase := range []struct {
		description    string
		text           string
		expectedOutput string
	}{
		{
			description:    "Text does not contain tags",
			text:           "This is text area",
			expectedOutput: "This is text area",
		},
		{
			description:    "Text contains div and paragraph tags",
			text:           "This is text area with <div> and <p> tags",
			expectedOutput: "This is text area with \n and \n tags\n\n\n\n\n",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			text := convertToMD(testCase.text)
			assert.Equal(t, text, testCase.expectedOutput)
		})
	}
}
