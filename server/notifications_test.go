// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatNotificationMessage(t *testing.T) {
	testCases := []struct {
		Description string

		ActorDisplayName       string
		ChatTopic              string
		ChatSize               int
		ChatLink               string
		Message                string
		AttachmentCount        int
		SkippedFileAttachments int
		ExpectedMessage        string
	}{
		{
			Description: "empty message, no attachments",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "",

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

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):`,
		},
		{
			Description: "empty message, more than one attachment",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "",
			AttachmentCount:  2,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):`,
		},
		{
			Description: "chat message",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "chat message with attachments",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "chat message with topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         2,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "chat message with skipped attachments",

			ActorDisplayName:       "Sender",
			ChatTopic:              "",
			ChatSize:               2,
			ChatLink:               "http://teams.microsoft.com/chat/1",
			Message:                "Hello!",
			SkippedFileAttachments: 1,

			ExpectedMessage: `**Sender** messaged you in an [MS Teams chat](http://teams.microsoft.com/chat/1):
> Hello!

*Some file attachments from this message could not be delivered.*`,
		},
		{
			Description: "group chat message",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message with attachments",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",
			AttachmentCount:  1,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message with topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         3,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message with skipped attachments",

			ActorDisplayName:       "Sender",
			ChatTopic:              "Topic",
			ChatSize:               3,
			ChatLink:               "http://teams.microsoft.com/chat/1",
			Message:                "Hello!",
			SkippedFileAttachments: 1,

			ExpectedMessage: `**Sender** messaged you and 1 other user in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!

*Some file attachments from this message could not be delivered.*`,
		},
		{
			Description: "group chat message with 5 users",

			ActorDisplayName: "Sender",
			ChatTopic:        "",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat](http://teams.microsoft.com/chat/1):
> Hello!`,
		},
		{
			Description: "group chat message with 5 users and topic",

			ActorDisplayName: "Sender",
			ChatTopic:        "Topic",
			ChatSize:         5,
			ChatLink:         "http://teams.microsoft.com/chat/1",
			Message:          "Hello!",

			ExpectedMessage: `**Sender** messaged you and 3 other users in an [MS Teams group chat: Topic](http://teams.microsoft.com/chat/1):
> Hello!`,
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
> ` + "```",
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
				tc.SkippedFileAttachments,
			)
			assert.Equal(t, tc.ExpectedMessage, actualMessage)
		})
	}
}
