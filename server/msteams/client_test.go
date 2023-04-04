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
	assert := assert.New(t)
	resp := convertToMessage(&msgraph.ChatMessage{
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
	}, "mockTeamsTeamID", testutils.GetChannelID(), "mockChatID")

	assert.Equal(Message{
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
	}, *resp)
}

func TestGetResourceIds(t *testing.T) {
	for _, test := range []struct {
		Name           string
		Resource       string
		ExpectedResult ActivityIds
	}{
		{
			Name:     "GetResourceIds: With prefix chats(",
			Resource: "chats(chat123-ID/mockChannel456-ID",
			ExpectedResult: ActivityIds{
				ChatID:    "hat123-",
				MessageID: "l456-",
			},
		},
		{
			Name:     "GetResourceIds: Without prefix chats(",
			Resource: "mockTeam123-ID/mockChannel456-ID/mockMessage789-ID/mockReply910-ID",
			ExpectedResult: ActivityIds{
				TeamID:    "m123-",
				ChannelID: "l456-",
				MessageID: "e789-",
				ReplyID:   "910-",
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			resp := GetResourceIds(test.Resource)
			assert.Equal(test.ExpectedResult, resp)
		})
	}
}
