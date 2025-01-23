// Copyright (c) 2023-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

//go:build e2e
// +build e2e

package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/enescakir/emoji"
	"gitlab.com/golang-commonmark/markdown"

	mm_markdown "github.com/mattermost/mattermost-plugin-msteams/server/markdown"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendFormattedMessages(t *testing.T) {
	setup(t)

	md := markdown.New(markdown.XHTMLOutput(true), markdown.Typographer(false))

	for _, testCase := range []struct {
		description    string
		text           string
		expectedOutput string
	}{
		{
			description:    "Bold",
			text:           "**BOLD**",
			expectedOutput: "**BOLD**",
		},
		{
			description:    "Italics",
			text:           "~italics~",
			expectedOutput: "~italics~",
		},
		{
			description:    "Strike-through",
			text:           "~~strikethrough~~",
			expectedOutput: "~~strikethrough~~",
		},
		{
			description:    "Basic Table",
			text:           "|one  |two |\n|-----|----|\n|three|four|",
			expectedOutput: "|one  |two |\n|-----|----|\n|three|four|",
		},
		{
			description:    "Basic Table Spacing Truncated",
			text:           "|    one   |   two |\n|-----|-------|\n|  three  |  four  |",
			expectedOutput: "|one  |two |\n|-----|----|\n|three|four|",
		},
		{
			description:    "Numbered List",
			text:           "1. One\n2. Two\n3. Three\n4. Four",
			expectedOutput: "1. One\n2. Two\n3. Three\n4. Four",
		},
		{
			description:    "Bullet List 1",
			text:           "* bullet one\n* bullet two\n* bullet three",
			expectedOutput: "* bullet one\n* bullet two\n* bullet three",
		},
		{
			description:    "Bullet List 2",
			text:           "- bullet one\n- bullet two\n- bullet three",
			expectedOutput: "* bullet one\n* bullet two\n* bullet three",
		},
		{
			description:    "Quote List",
			text:           "> testing quote",
			expectedOutput: "> testing quote",
		},
		{
			description:    "Heading 1",
			text:           "# Heading One",
			expectedOutput: "# Heading One",
		},
		{
			description:    "Heading 2",
			text:           "## Heading Two",
			expectedOutput: "## Heading Two",
		},
		{
			description:    "Heading 3",
			text:           "### Heading Three",
			expectedOutput: "### Heading Three",
		},
		{
			description:    "Line",
			text:           "---",
			expectedOutput: "---",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			content := md.RenderToString([]byte(emoji.Parse(testCase.text)))

			_, err := msClient.SendChat(testCfg.MSTeams.ChatID, content, nil, nil, nil)
			require.NoError(t, err)
			// time.Sleep(2 * time.Second)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				postsList, _, err := mmClient.GetPostsForChannel(context.Background(), testCfg.Mattermost.DmID, 0, 1, "", false, false)
				require.NoError(c, err)
				require.Equal(c, 1, len(postsList.Posts))

				for _, post := range postsList.Posts {
					text := mm_markdown.ConvertToMD(post.Message)
					text = strings.Trim(text, "\n")
					assert.True(c, text == testCase.expectedOutput)
				}
			}, 10*time.Second, 500*time.Millisecond)
		})
	}

}
