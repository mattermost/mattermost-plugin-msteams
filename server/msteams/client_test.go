package msteams

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/stretchr/testify/assert"
	msgraph "github.com/yaegashi/msgraph.go/beta"
)

func TestConvertToMessage(t *testing.T) {
	teamsUserID := testutils.GetTeamUserID()
	teamsUserDisplayName := "mockTeamsUserDisplayName"
	teamsReplyID := "mockTeamsReplyID"
	content := "mockContent"
	subject := "mockSubject"
	reactionType := "mockReactionType"
	attachmentContent := "mockAttachmentContent"
	attachmentContentType := "mockAttachmentContentType"
	attachmentName := "mockAttachmentName"
	attachmentURL := "mockAttachmentURL"
	for _, test := range []struct {
		Name           string
		ChatMessage    *msgraph.ChatMessage
		ExpectedResult Message
	}{
		{
			Name: "ConvertToMessage: With data filled",
			ChatMessage: &msgraph.ChatMessage{
				From: &msgraph.IdentitySet{
					User: &msgraph.Identity{
						ID:          &teamsUserID,
						DisplayName: &teamsUserDisplayName,
					},
				},
				ReplyToID: &teamsReplyID,
				Body: &msgraph.ItemBody{
					Content: &content,
				},
				Subject:              &subject,
				LastModifiedDateTime: &time.Time{},
				Attachments: []msgraph.ChatMessageAttachment{
					{
						ContentType: &attachmentContentType,
						Content:     &attachmentContent,
						Name:        &attachmentName,
						ContentURL:  &attachmentURL,
					},
				},
				Reactions: []msgraph.ChatMessageReaction{
					{
						ReactionType: &reactionType,
						User: &msgraph.IdentitySet{
							User: &msgraph.Identity{
								ID: &teamsUserID,
							},
						},
					},
				},
			},
			ExpectedResult: Message{
				UserID:          teamsUserID,
				UserDisplayName: teamsUserDisplayName,
				ReplyToID:       teamsReplyID,
				Text:            content,
				Subject:         subject,
				LastUpdateAt:    time.Time{},
				Attachments: []Attachment{
					{
						ContentType: attachmentContentType,
						Content:     attachmentContent,
						Name:        attachmentName,
						ContentURL:  attachmentURL,
					},
				},
				Reactions: []Reaction{
					{
						Reaction: reactionType,
						UserID:   teamsUserID,
					},
				},
				ChannelID: testutils.GetChannelID(),
				TeamID:    "mockTeamsTeamID",
				ChatID:    "mockChatID",
			},
		},
		{
			Name: "ConvertToMessage: With no data filled",
			ChatMessage: &msgraph.ChatMessage{
				LastModifiedDateTime: &time.Time{},
			},
			ExpectedResult: Message{
				Attachments:  []Attachment{},
				Reactions:    []Reaction{},
				LastUpdateAt: time.Time{},
				ChannelID:    testutils.GetChannelID(),
				TeamID:       "mockTeamsTeamID",
				ChatID:       "mockChatID",
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			resp := convertToMessage(test.ChatMessage, "mockTeamsTeamID", testutils.GetChannelID(), "mockChatID")

			assert.Equal(test.ExpectedResult, *resp)
		})
	}
}

func TestGetResourceIds(t *testing.T) {
	for _, test := range []struct {
		Name           string
		Resource       string
		ExpectedResult ActivityIds
	}{
		{
			Name:     "GetResourceIds: With prefix chats(",
			Resource: "chats('19:8ea0e38b-efb3-4757-924a-5f94061cf8c2_97f62344-57dc-409c-88ad-c4af14158ff5@unq.gbl.spaces')/messages('1612289765949')",
			ExpectedResult: ActivityIds{
				ChatID:    "19:8ea0e38b-efb3-4757-924a-5f94061cf8c2_97f62344-57dc-409c-88ad-c4af14158ff5@unq.gbl.spaces",
				MessageID: "1612289765949",
			},
		},
		{
			Name:     "GetResourceIds: Without prefix chats(",
			Resource: "teams('fbe2bf47-16c8-47cf-b4a5-4b9b187c508b')/channels('19:4a95f7d8db4c4e7fae857bcebe0623e6@thread.tacv2')/messages('1612293113399')/replies('19:zOtXfpDMWANo7-9CHuzHdM7WPSamQejH0Vydj0U-Yho1')",
			ExpectedResult: ActivityIds{
				TeamID:    "fbe2bf47-16c8-47cf-b4a5-4b9b187c508b",
				ChannelID: "19:4a95f7d8db4c4e7fae857bcebe0623e6@thread.tacv2",
				MessageID: "1612293113399",
				ReplyID:   "19:zOtXfpDMWANo7-9CHuzHdM7WPSamQejH0Vydj0U-Yho1",
			},
		},
		{
			Name:           "GetResourceIds: Resource with multiple '/'",
			Resource:       "/////19:4a95f7d8db4c4e7fae857bcebe0623e6@thread.tacv2///",
			ExpectedResult: ActivityIds{},
		},
		{
			Name:           "GetResourceIds: Empty resource",
			ExpectedResult: ActivityIds{},
		},
		{
			Name:           "GetResourceIds: Resource with small length",
			Resource:       "ID",
			ExpectedResult: ActivityIds{},
		},
		{
			Name:           "GetResourceIds: Resource with large length",
			Resource:       "very-long-teams-ID-with-very-long-chat-ID",
			ExpectedResult: ActivityIds{},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			resp := GetResourceIds(test.Resource)
			assert.Equal(test.ExpectedResult, resp)
		})
	}
}
