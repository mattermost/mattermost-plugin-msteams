package handlers

import (
	"net/http"
	"testing"

	mocksPlugin "github.com/mattermost/mattermost-plugin-msteams-sync/server/handlers/mocks"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/msteams"
	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/stretchr/testify/assert"
)

type FakeHttpTransport struct{}

func (_ FakeHttpTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{}, nil
}

func TestMsgToPost(t *testing.T) {
	defaultTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = &FakeHttpTransport{}
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
