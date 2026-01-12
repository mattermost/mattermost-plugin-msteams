// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"net/http"
	"testing"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/stretchr/testify/assert"

	"github.com/mattermost/mattermost-plugin-msteams/server/msteams/clientmodels"
)

type FakeHTTPTransport struct{}

func (FakeHTTPTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{}, nil
}

func TestMsgToPost(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("successfully add message to post", func(t *testing.T) {
		th.Reset(t)

		sender := th.SetupUser(t, team)
		channel := th.SetupPublicChannel(t, team)

		message := &clientmodels.Message{
			Subject:         "Subject of the messsage",
			UserDisplayName: "mock-UserDisplayName",
			UserID:          "t" + sender.Id,
			CreateAt:        time.Now(),
		}

		expectedPost := &model.Post{
			UserId:    sender.Id,
			ChannelId: channel.Id,
			Message:   "## Subject of the messsage\n",
			Props: model.StringInterface{
				"msteams_sync_" + th.p.botUserID: true,
			},
			FileIds:  model.StringArray{},
			CreateAt: message.CreateAt.UnixNano() / int64(time.Millisecond),
		}

		actualPost, _, _ := th.p.activityHandler.msgToPost(channel.Id, sender.Id, message, nil, []string{})
		assert.Equal(t, expectedPost, actualPost)
	})
}

func TestHandleMentions(t *testing.T) {
	th := setupTestHelper(t)
	team := th.SetupTeam(t)

	t.Run("no mentions", func(t *testing.T) {
		th.Reset(t)

		message := &clientmodels.Message{
			Text: "mockMessage",
		}
		expectedMessage := "mockMessage"

		actualMessage := th.p.activityHandler.handleMentions(message)
		assert.Equal(t, expectedMessage, actualMessage)
	})

	t.Run("all mentions present", func(t *testing.T) {
		th.Reset(t)

		message := &clientmodels.Message{
			Text: `mockMessage <at id="0">Everyone</at>`,
			Mentions: []clientmodels.Mention{
				{
					ID:            0,
					MentionedText: "Everyone",
				},
			},
		}
		expectedMessage := "mockMessage @all"

		actualMessage := th.p.activityHandler.handleMentions(message)
		assert.Equal(t, expectedMessage, actualMessage)
	})

	t.Run("unknown user mentioned", func(t *testing.T) {
		th.Reset(t)

		message := &clientmodels.Message{
			Text: `mockMessage <at id="0">mockMentionedText</at>`,
			Mentions: []clientmodels.Mention{
				{
					ID:            0,
					UserID:        model.NewId(),
					MentionedText: "mockMentionedText",
				},
			},
		}
		expectedMessage := `mockMessage <at id="0">mockMentionedText</at>`

		actualMessage := th.p.activityHandler.handleMentions(message)
		assert.Equal(t, expectedMessage, actualMessage)
	})

	t.Run("multiple user mentions", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		user2 := th.SetupUser(t, team)
		th.ConnectUser(t, user2.Id)

		message := &clientmodels.Message{
			Text: `hello <at id="0">mockMSUsername-1</at> from <at id="1">mockMSUsername-2</at>`,
			Mentions: []clientmodels.Mention{
				{
					ID:            0,
					UserID:        "t" + user1.Id,
					MentionedText: "mockMSUsername-1",
				},
				{
					ID:            1,
					UserID:        "t" + user2.Id,
					MentionedText: "mockMSUsername-2",
				},
			},
		}
		expectedMessage := "hello @" + user1.Username + " from @" + user2.Username

		actualMessage := th.p.activityHandler.handleMentions(message)
		assert.Equal(t, expectedMessage, actualMessage)
	})

	t.Run("multi-word user mentions", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		message := &clientmodels.Message{
			Text: `hello <at id="0">Miguel</at>&nbsp;<at id="1">de</at>&nbsp;<at id="2">la</at>&nbsp;<at id="3">Cruz</at>`,
			Mentions: []clientmodels.Mention{
				{
					ID:            0,
					UserID:        "t" + user1.Id,
					MentionedText: "Miguel",
				},
				{
					ID:            1,
					UserID:        "t" + user1.Id,
					MentionedText: "de",
				},
				{
					ID:            2,
					UserID:        "t" + user1.Id,
					MentionedText: "la",
				},
				{
					ID:            3,
					UserID:        "t" + user1.Id,
					MentionedText: "Cruz",
				},
			},
		}

		expectedMessage := "hello @" + user1.Username

		actualMessage := th.p.activityHandler.handleMentions(message)
		assert.Equal(t, expectedMessage, actualMessage)
	})

	t.Run("multi-word user mentions, unknown user", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)

		message := &clientmodels.Message{
			Text: `hello <at id="0">Miguel</at>&nbsp;<at id="1">de</at>&nbsp;<at id="2">la</at>&nbsp;<at id="3">Cruz</at>`,
			Mentions: []clientmodels.Mention{
				{
					ID:            0,
					UserID:        "t" + user1.Id,
					MentionedText: "Miguel",
				},
				{
					ID:            1,
					UserID:        "t" + user1.Id,
					MentionedText: "de",
				},
				{
					ID:            2,
					UserID:        "t" + user1.Id,
					MentionedText: "la",
				},
				{
					ID:            3,
					UserID:        "t" + user1.Id,
					MentionedText: "Cruz",
				},
			},
		}

		expectedMessage := `hello <at id="0">Miguel de la Cruz</at>`

		actualMessage := th.p.activityHandler.handleMentions(message)
		assert.Equal(t, expectedMessage, actualMessage)
	})

	t.Run("multi-word user mentions, repeated", func(t *testing.T) {
		th.Reset(t)

		user1 := th.SetupUser(t, team)
		th.ConnectUser(t, user1.Id)

		message := &clientmodels.Message{
			Text: `hello <at id="0">Miguel</at>&nbsp;<at id="1">de</at>&nbsp;<at id="2">la</at>&nbsp;<at id="3">Cruz</at><at id="4">Miguel</at>&nbsp;<at id="5">de</at>&nbsp;<at id="6">la</at>&nbsp;<at id="7">Cruz</at>`,
			Mentions: []clientmodels.Mention{
				{
					ID:            0,
					UserID:        "t" + user1.Id,
					MentionedText: "Miguel",
				},
				{
					ID:            1,
					UserID:        "t" + user1.Id,
					MentionedText: "de",
				},
				{
					ID:            2,
					UserID:        "t" + user1.Id,
					MentionedText: "la",
				},
				{
					ID:            3,
					UserID:        "t" + user1.Id,
					MentionedText: "Cruz",
				},
				{
					ID:            4,
					UserID:        "t" + user1.Id,
					MentionedText: "Miguel",
				},
				{
					ID:            5,
					UserID:        "t" + user1.Id,
					MentionedText: "de",
				},
				{
					ID:            6,
					UserID:        "t" + user1.Id,
					MentionedText: "la",
				},
				{
					ID:            7,
					UserID:        "t" + user1.Id,
					MentionedText: "Cruz",
				},
			},
		}

		expectedMessage := "hello @" + user1.Username + "@" + user1.Username

		actualMessage := th.p.activityHandler.handleMentions(message)
		assert.Equal(t, expectedMessage, actualMessage)
	})
}

func TestHandleEmojis(t *testing.T) {
	th := setupTestHelper(t)

	for _, testCase := range []struct {
		description  string
		text         string
		expectedText string
	}{
		{
			description:  "Text with emoji in end",
			text:         `<div><div>hi <emoji id="lipssealed" alt="🤫" title=""></emoji><emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji></div></div>`,
			expectedText: "<div><div>hi 🤫😛</div></div>",
		},
		{
			description:  "Text between emoji",
			text:         `<div><div>hiii <emoji id="lipssealed" alt="🤫" title=""></emoji> hi <emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji></div></div>`,
			expectedText: "<div><div>hiii 🤫 hi 😛</div></div>",
		},
		{
			description:  "Text with emoji in start",
			text:         `<div><div><emoji id="lipssealed" alt="🤫" title=""></emoji><emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji> hi</div></div>`,
			expectedText: "<div><div>🤫😛 hi</div></div>",
		},
		{
			description:  "Text with only emoji",
			text:         `<div><div><emoji id="lipssealed" alt="🤫" title=""></emoji><emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji></div></div>`,
			expectedText: "<div><div>🤫😛</div></div>",
		},
		{
			description:  "Text with random formatting",
			text:         `<div><div> hi   <emoji id="lipssealed" alt="🤫" title=""></emoji> hello  <emoji id="1f61b_facewithtongue" alt="😛" title=""></emoji> hey    </div></div>`,
			expectedText: "<div><div> hi   🤫 hello  😛 hey    </div></div>",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			th.Reset(t)

			actualText := th.p.activityHandler.handleEmojis(testCase.text)
			assert.Equal(t, actualText, testCase.expectedText)
		})
	}
}
