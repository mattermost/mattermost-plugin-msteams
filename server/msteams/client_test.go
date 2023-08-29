package msteams

import (
	"testing"
	"time"

	"github.com/mattermost/mattermost-plugin-msteams-sync/server/testutils"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/stretchr/testify/assert"
)

func TestConvertToMessage(t *testing.T) {
	teamsUserID := testutils.GetTeamsUserID()
	teamsUserDisplayName := "mockTeamsUserDisplayName"
	teamsReplyID := "mockTeamsReplyID"
	content := "mockContent"
	subject := "mockSubject"
	reactionType := "mockReactionType"
	attachmentContent := "mockAttachmentContent"
	attachmentContentType := "mockAttachmentContentType"
	attachmentName := "mockAttachmentName"
	attachmentURL := "mockAttachmentURL"
	mentionID := 0
	for _, test := range []struct {
		Name           string
		ChatMessage    models.ChatMessageable
		ExpectedResult Message
	}{
		{
			Name: "ConvertToMessage: With data filled",
			ChatMessage: func() models.ChatMessageable {
				from := models.NewIdentitySet()
				user := models.NewIdentity()
				user.SetId(&teamsUserID)
				user.SetDisplayName(&teamsUserDisplayName)
				from.SetUser(user)

				body := models.NewItemBody()
				body.SetContent(&content)

				attachment := models.NewChatMessageAttachment()
				attachment.SetContentType(&attachmentContentType)
				attachment.SetContent(&attachmentContent)
				attachment.SetName(&attachmentName)
				attachment.SetContentUrl(&attachmentURL)

				reactionUserSet := models.NewIdentitySet()
				reactionUser := models.NewIdentity()
				reactionUser.SetId(&teamsUserID)
				reactionUserSet.SetUser(reactionUser)
				reaction := models.NewChatMessageReaction()
				reaction.SetUser(reactionUserSet)
				reaction.SetReactionType(&reactionType)

				mention := models.NewChatMessageMention()
				mentionedID := int32(mentionID)
				mention.SetId(&mentionedID)

				identity := models.NewIdentity()
				identity.SetId(&teamsUserID)
				identity.SetDisplayName(&teamsUserDisplayName)

				additionalData := map[string]interface{}{
					"userIdentityType": "aadUser",
				}

				identity.SetAdditionalData(additionalData)
				mentioned := models.NewChatMessageMentionedIdentitySet()
				mentioned.SetUser(identity)

				mention.SetMentionText(&teamsUserDisplayName)
				mention.SetMentioned(mentioned)

				message := models.NewChatMessage()
				message.SetFrom(from)
				message.SetReplyToId(&teamsReplyID)
				message.SetBody(body)
				message.SetSubject(&subject)
				message.SetLastModifiedDateTime(&time.Time{})
				message.SetAttachments([]models.ChatMessageAttachmentable{attachment})
				message.SetReactions([]models.ChatMessageReactionable{reaction})
				message.SetMentions([]models.ChatMessageMentionable{mention})
				return message
			}(),
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
				Mentions: []Mention{
					{
						ID:            int32(mentionID),
						UserID:        teamsUserID,
						MentionedText: teamsUserDisplayName,
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
			ChatMessage: func() models.ChatMessageable {
				message := models.NewChatMessage()
				message.SetLastModifiedDateTime(&time.Time{})
				return message
			}(),
			ExpectedResult: Message{
				Attachments:  []Attachment{},
				Reactions:    []Reaction{},
				Mentions:     []Mention{},
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
