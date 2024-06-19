package main

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
	storemocks "github.com/mattermost/mattermost-plugin-msteams/server/store/mocks"
	"github.com/mattermost/mattermost-plugin-msteams/server/testutils"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/assert"
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
			setupAPI: func(api *plugintest.API) {
			},
			post: &model.Post{
				UserId:    testutils.GetSenderID(),
				ChannelId: testutils.GetChannelID(),
				Message:   "## Subject of the messsage\n",
				Props: model.StringInterface{
					"msteams_sync_bot-user-id": true,
				},
				FileIds:  model.StringArray{},
				CreateAt: mmCreateAtTime,
			},
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			p := newTestPlugin(t)
			ah := ActivityHandler{}
			testCase.setupAPI(p.API.(*plugintest.API))

			ah.plugin = p

			post, _, _ := ah.msgToPost(testCase.channelID, testCase.senderID, testCase.message, nil, []string{})
			assert.Equal(t, testCase.post, post)
		})
	}
}

func TestHandleMentions(t *testing.T) {
	ah := ActivityHandler{}
	for _, testCase := range []struct {
		description     string
		setupAPI        func(*plugintest.API)
		setupStore      func(*storemocks.Store)
		message         *clientmodels.Message
		expectedMessage string
	}{
		{
			description: "No mentions present",
			setupAPI:    func(api *plugintest.API) {},
			setupStore:  func(store *storemocks.Store) {},
			message: &clientmodels.Message{
				Text: "mockMessage",
			},
			expectedMessage: "mockMessage",
		},
		{
			description: "All mention present",
			setupAPI:    func(api *plugintest.API) {},
			setupStore:  func(store *storemocks.Store) {},
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
			setupAPI: func(api *plugintest.API) {
			},
			setupStore: func(store *storemocks.Store) {
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
			setupAPI: func(api *plugintest.API) {
				api.On("GetUser", testutils.GetMattermostID()).Return(nil, testutils.GetInternalServerAppError("unable to get mm user details")).Once()
			},
			setupStore: func(store *storemocks.Store) {
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
			setupStore: func(store *storemocks.Store) {
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
			expectedMessage: "hello @mockMMUsername-1 from @mockMMUsername-2",
		},
		{
			description: "multi-word user mentions",
			setupAPI: func(api *plugintest.API) {
				api.On("GetUser", "mockMMUserID-1").Return(&model.User{
					Id:       "mockMMUserID-1",
					Username: "miguel",
				}, nil).Maybe()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", "mockMSUserID-1").Return("mockMMUserID-1", nil).Maybe()
			},
			message: &clientmodels.Message{
				Text: `hello <at id="0">Miguel</at>&nbsp;<at id="1">de</at>&nbsp;<at id="2">la</at>&nbsp;<at id="3">Cruz</at>`,
				Mentions: []clientmodels.Mention{
					{
						ID:            0,
						UserID:        "mockMSUserID-1",
						MentionedText: "Miguel",
					},
					{
						ID:            1,
						UserID:        "mockMSUserID-1",
						MentionedText: "de",
					},
					{
						ID:            2,
						UserID:        "mockMSUserID-1",
						MentionedText: "la",
					},
					{
						ID:            3,
						UserID:        "mockMSUserID-1",
						MentionedText: "Cruz",
					},
				},
			},
			expectedMessage: "hello @miguel",
		},
		{
			description: "multi-word user mentions, unknown user",
			setupAPI: func(api *plugintest.API) {
				api.On("GetUser", "mockMMUserID-1").Return(&model.User{
					Id:       "mockMMUserID-1",
					Username: "miguel",
				}, nil).Maybe()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", "mockMSUserID-1").Return("", errors.New("unable to get mm user ID"))
			},
			message: &clientmodels.Message{
				Text: `hello <at id="0">Miguel</at>&nbsp;<at id="1">de</at>&nbsp;<at id="2">la</at>&nbsp;<at id="3">Cruz</at>`,
				Mentions: []clientmodels.Mention{
					{
						ID:            0,
						UserID:        "mockMSUserID-1",
						MentionedText: "Miguel",
					},
					{
						ID:            1,
						UserID:        "mockMSUserID-1",
						MentionedText: "de",
					},
					{
						ID:            2,
						UserID:        "mockMSUserID-1",
						MentionedText: "la",
					},
					{
						ID:            3,
						UserID:        "mockMSUserID-1",
						MentionedText: "Cruz",
					},
				},
			},
			expectedMessage: `hello <at id="0">Miguel de la Cruz</at>`,
		},
		{
			description: "multi-word user mentions, repeated",
			setupAPI: func(api *plugintest.API) {
				api.On("GetUser", "mockMMUserID-1").Return(&model.User{
					Id:       "mockMMUserID-1",
					Username: "miguel",
				}, nil).Maybe()
			},
			setupStore: func(store *storemocks.Store) {
				store.On("TeamsToMattermostUserID", "mockMSUserID-1").Return("mockMMUserID-1", nil).Maybe()
			},
			message: &clientmodels.Message{
				Text: `hello <at id="0">Miguel</at>&nbsp;<at id="1">de</at>&nbsp;<at id="2">la</at>&nbsp;<at id="3">Cruz</at><at id="4">Miguel</at>&nbsp;<at id="5">de</at>&nbsp;<at id="6">la</at>&nbsp;<at id="7">Cruz</at>`,
				Mentions: []clientmodels.Mention{
					{
						ID:            0,
						UserID:        "mockMSUserID-1",
						MentionedText: "Miguel",
					},
					{
						ID:            1,
						UserID:        "mockMSUserID-1",
						MentionedText: "de",
					},
					{
						ID:            2,
						UserID:        "mockMSUserID-1",
						MentionedText: "la",
					},
					{
						ID:            3,
						UserID:        "mockMSUserID-1",
						MentionedText: "Cruz",
					},
					{
						ID:            4,
						UserID:        "mockMSUserID-1",
						MentionedText: "Miguel",
					},
					{
						ID:            5,
						UserID:        "mockMSUserID-1",
						MentionedText: "de",
					},
					{
						ID:            6,
						UserID:        "mockMSUserID-1",
						MentionedText: "la",
					},
					{
						ID:            7,
						UserID:        "mockMSUserID-1",
						MentionedText: "Cruz",
					},
				},
			},
			expectedMessage: "hello @miguel@miguel",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			assert := assert.New(t)
			p := newTestPlugin(t)
			testCase.setupAPI(p.API.(*plugintest.API))
			testutils.MockLogs(p.API.(*plugintest.API))
			testCase.setupStore(p.store.(*storemocks.Store))

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
			p := newTestPlugin(t)
			ah.plugin = p
			text := ah.handleEmojis(testCase.text)
			assert.Equal(t, text, testCase.expectedOutput)
		})
	}
}
