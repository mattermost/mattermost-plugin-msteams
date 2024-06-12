package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatNotificationMessage(t *testing.T) {
	testCases := []struct {
		Description string

		ActorDisplayName string
		ChatTopic        string
		ChatSize         int
		ChatLink         string
		Message          string
		AttachmentCount  int
		ExpectedMessage  string
	}{
		{
			Description: "empty message, no attachments",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "",
			AttachmentCount:  0,

			ExpectedMessage: ``,
		},
		{
			Description: "empty message, one attachment",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):

*This message was originally sent with one attachment.*`,
		},
		{
			Description: "empty message, more than one attachment",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):

*This message was originally sent with 2 attachments.*`,
		},
		{
			Description: "chat message, no attachments, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  0,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "chat message, one attachment, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with one attachment.*`,
		},
		{
			Description: "chat message, more than one attachment, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with 2 attachments.*`,
		},
		{
			Description: "chat message, no attachments, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  0,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "chat message, one attachment, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with one attachment.*`,
		},
		{
			Description: "chat message, more than one attachment, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with 2 attachments.*`,
		},
		{
			Description: "group chat message, no attachments, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  0,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message, one attachment, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with one attachment.*`,
		},
		{
			Description: "group chat message, more than one attachment, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with 2 attachments.*`,
		},
		{
			Description: "group chat message, no attachments, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  0,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message, one attachment, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with one attachment.*`,
		},
		{
			Description: "group chat message, more than one attachment, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with 2 attachments.*`,
		},
		{
			Description: "group chat message with 5 users, no attachments, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  0,

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message with 5 users, one attachment, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with one attachment.*`,
		},
		{
			Description: "group chat message with 5 users, more than one attachment, no topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with 2 attachments.*`,
		},
		{
			Description: "group chat message with 5 users, no attachments, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  0,

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message with 5 users, one attachment, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with one attachment.*`,
		},
		{
			Description: "group chat message with 5 users, more than one attachment, has topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!

*This message was originally sent with 2 attachments.*`,
		},
		{
			Description: "multiline, complex chat message",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message: `Hello!
I heard you say:
> Welcome!

| Column A | Column B |
| --- | --- |
| Value 1 | Value 2 |

` + "```" + `sql
SELECT * FROM Users
` + "```" + `
`,
			AttachmentCount: 2,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!
> I heard you say:
> > Welcome!
>` + " " + `
> | Column A | Column B |
> | --- | --- |
> | Value 1 | Value 2 |
>` + " " + `
> ` + "```" + `sql
> SELECT * FROM Users
> ` + "```" + `

*This message was originally sent with 2 attachments.*`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Description, func(t *testing.T) {
			actualMessage := formatNotificationMessage(
				tc.ActorDisplayName,
				tc.ChatTopic,
				tc.ChatSize,
				tc.ChatLink,
				tc.Message,
				tc.AttachmentCount,
			)
			assert.Equal(t, tc.ExpectedMessage, actualMessage)
		})
	}
}
