package handlers

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	mocksMetrics "github.com/mattermost/mattermost-plugin-msteams-sync/server/metrics/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/clientmodels"
	mocksClient "github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams/mocks"
	mocksStore "github.com/mattermost/mattermost-plugin-msteams-sync/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
)

type FakeHTTPTransport struct{}

func (FakeHTTPTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{}, nil
}

func TestMsgToPost(t *testing.T) {
	msteamsCreateAtTime := time.Now()
	mmCreateAtTime := msteamsCreateAtTime.UnixNano() / int64(time.Millisecond)
	for _, testCase := range []struct {
		description string
		channelID   string
		userID      string
		senderID    string
		message     *clientmodels.Message
		post        *model.Post
		setupPlugin func(plugin *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, mockmetrics *mocksMetrics.Metrics)
		setupAPI    func(*plugintest.API)
	}{
		{
			description: "Successfully add message to post",
			channelID:   testutils.GetChannelID(),
			userID:      testutils.GetUserID(),
			senderID:    testutils.GetSenderID(),
			message: &clientmodels.Message{
				Subject:         "Subject of the messsage",
				UserDisplayName: "mock-UserDisplayName",
				UserID:          testutils.GetUserID(),
				CreateAt:        msteamsCreateAtTime,
			},
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, client *mocksClient.Client, mockmetrics *mocksMetrics.Metrics) {
				p.On("GetBotUserID").Return(testutils.GetSenderID())
				p.On("GetClientForApp").Return(client).Maybe()
				p.On("GetMetrics").Return(mockmetrics).Maybe()
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetRemoteID").Return(testutils.GetRemoteID())
			},
			setupAPI: func(api *plugintest.API) {
			},
			post: &model.Post{
				UserId:    testutils.GetSenderID(),
				ChannelId: testutils.GetChannelID(),
				Message:   "## Subject of the messsage\n",
				Props: model.StringInterface{
					"from_webhook":                         "true",
					"msteams_sync_pqoejrn65psweomewmosaqr": true,
				},
				FileIds:  model.StringArray{},
				CreateAt: mmCreateAtTime,
				RemoteId: model.NewString(testutils.GetRemoteID()),
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah := ActivityHandler{}
			client := mocksClient.NewClient(t)
			mockAPI := &plugintest.API{}
			mockMetrics := mocksMetrics.NewMetrics(t)
			testCase.setupAPI(mockAPI)
			testCase.setupPlugin(p, mockAPI, client, mockMetrics)

			ah.plugin = p

			post, _ := ah.msgToPost(testCase.channelID, testCase.senderID, testCase.message, nil, false)
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
		message         *clientmodels.Message
		expectedMessage string
	}{
		{
			description: "No mentions present",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {},
			setupAPI:    func(api *plugintest.API) {},
			setupStore:  func(store *mocksStore.Store) {},
			message: &clientmodels.Message{
				Text: "mockMessage",
			},
			expectedMessage: "mockMessage",
		},
		{
			description: "All mention present",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {},
			setupAPI:    func(api *plugintest.API) {},
			setupStore:  func(store *mocksStore.Store) {},
			message: &clientmodels.Message{
				Text: `mockMessage <at id="0">Everyone</at>`,
				Mentions: []clientmodels.Mention{
					{
						ID:            0,
						MentionedText: "Everyone",
					},
				},
			},
			expectedMessage: "mockMessage @all",
		},
		{
			description: "Unable to get mm user ID for user mentions",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
			},
			setupAPI: func(api *plugintest.API) {
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return("", errors.New("unable to get mm user ID"))
			},
			message: &clientmodels.Message{
				Text: `mockMessage <at id="0">mockMentionedText</at>`,
				Mentions: []clientmodels.Mention{
					{
						ID:            0,
						UserID:        testutils.GetTeamsUserID(),
						MentionedText: "mockMentionedText",
					},
				},
			},
			expectedMessage: `mockMessage <at id="0">mockMentionedText</at>`,
		},
		{
			description: "Unable to get mm user details for user mentions",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
			},
			setupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetMattermostID()).Return(nil, testutils.GetInternalServerAppError("unable to get mm user details")).Once()
			},
			setupStore: func(store *mocksStore.Store) {
				store.On("TeamsToMattermostUserID", testutils.GetTeamsUserID()).Return(testutils.GetMattermostID(), nil).Once()
			},
			message: &clientmodels.Message{
				Text: `mockMessage <at id="0">mockMentionedText</at>`,
				Mentions: []clientmodels.Mention{
					{
						ID:            0,
						UserID:        testutils.GetTeamsUserID(),
						MentionedText: "mockMentionedText",
					},
				},
			},
			expectedMessage: `mockMessage <at id="0">mockMentionedText</at>`,
		},
		{
			description: "Successful user mentions",
			setupPlugin: func(p *mocksPlugin.PluginIface, mockAPI *plugintest.API, store *mocksStore.Store) {
				p.On("GetAPI").Return(mockAPI).Maybe()
				p.On("GetStore").Return(store).Maybe()
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
			message: &clientmodels.Message{
				Text: `hello <at id="0">mockMSUsername-1</at> from <at id="1">mockMSUsername-2</at>`,
				Mentions: []clientmodels.Mention{
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
			testutils.MockLogs(mockAPI)
			testCase.setupStore(store)

			ah.plugin = p
			message := ah.handleMentions(testCase.message)
			assert.Equal(testCase.expectedMessage, message)
		})
	}
}

func TestHandleEmojis(t *testing.T) {
	ah := ActivityHandler{}
	for _, testCase := range []struct {
		description    string
		text           string
		expectedOutput string
	}{
		{
			description:    "Text with emoji in end",
			text:           `<div><div>hi <emoji id="lipssealed" alt="🤫" title=""></emoji><emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji></div></div>`,
			expectedOutput: "<div><div>hi 🤫😛</div></div>",
		},
		{
			description:    "Text between emoji",
			text:           `<div><div>hiii <emoji id="lipssealed" alt="🤫" title=""></emoji> hi <emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji></div></div>`,
			expectedOutput: "<div><div>hiii 🤫 hi 😛</div></div>",
		},
		{
			description:    "Text with emoji in start",
			text:           `<div><div><emoji id="lipssealed" alt="🤫" title=""></emoji><emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji> hi</div></div>`,
			expectedOutput: "<div><div>🤫😛 hi</div></div>",
		},
		{
			description:    "Text with only emoji",
			text:           `<div><div><emoji id="lipssealed" alt="🤫" title=""></emoji><emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji></div></div>`,
			expectedOutput: "<div><div>🤫😛</div></div>",
		},
		{
			description:    "Text with random formatting",
			text:           `<div><div> hi   <emoji id="lipssealed" alt="🤫" title=""></emoji> hello  <emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji> hey    </div></div>`,
			expectedOutput: "<div><div> hi   🤫 hello  😛 hey    </div></div>",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := mocksPlugin.NewPluginIface(t)
			ah.plugin = p
			text := ah.handleEmojis(testCase.text)
			assert.Equal(t, text, testCase.expectedOutput)
		})
	}
}
