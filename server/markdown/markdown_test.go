// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToMD(t *testing.T) {
	for _, testCase := range []struct {
		description    string
		text           string
		expectedOutput string
	}{
		{
			description:    "Text does not contain tags",
			text:           "This is text area <></>",
			expectedOutput: "This is text area <></>",
		},
		{
			description:    "Text contains tags which shouldn't be converted to markdown",
			text:           "This is text area <textarea></textarea>",
			expectedOutput: "This is text area <textarea></textarea>",
		},
		{
			description:    "Text contains div and paragraph tags",
			text:           "This is text area with <div> and <p> tags",
			expectedOutput: "This is text area with\n\nand\n\ntags",
		},
		{
			description:    "Text contains paragraph and image tags",
			text:           "This is text area with <img src='test.com'> and <p class=''>Paragraph</p> tags",
			expectedOutput: "This is text area with ![](test.com) and\n\nParagraph\n\n tags",
		},
		{
			description:    "Text contains bold, italics and strike through tags",
			text:           "This is <p><b>bold</b></p>, <p><i>italics</i></p> and <p><s>strike through</s></p> text",
			expectedOutput: "This is\n\n**bold**\n\n,\n\n_italics_\n\n and\n\n~~strike through~~\n\n text",
		},
		{
			description:    "Text contains heading tags",
			text:           "This is text area with <h1>H1</h1>, <h2>H2</h2> and <h3>H3</h3> tags",
			expectedOutput: "This is text area with\n\n# H1\n\n,\n\n## H2\n\n and\n\n### H3\n\n tags",
		},
		{
			description:    "Simple Table, no header",
			text:           "<p>&nbsp;</p>\n<table itemprop=\"copy-paste-table\">\n<tbody>\n<tr>\n<td>\n<p>one</p>\n</td>\n<td>\n<p>two</p>\n</td>\n</tr>\n<tr>\n<td>\n<p>three</p>\n</td>\n<td>\n<p>four</p>\n</td>\n</tr>\n</tbody>\n</table>\n<p>&nbsp;</p>",
			expectedOutput: "|     |     |\n| --- | --- |\n| one | two |\n| three | four |",
		},
		{
			description:    "Simple Table with header",
			text:           "<p>&nbsp;</p>\n<table itemprop=\"copy-paste-table\">\n<thead>\n<tr>\n<th>one</th>\n<th>two</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>three</td>\n<td>four</td>\n</tr>\n</tbody>\n</table>\n<p>&nbsp;</p>",
			expectedOutput: "| one | two |\n| --- | --- |\n| three | four |",
		},
		{
			description:    "Bold Italics Strikethrough",
			text:           "<p><strong>bold </strong>normal <i>Italics <s>&nbsp;strike through&nbsp;</s></i></p>",
			expectedOutput: "**bold** normal _Italics ~~strike through~~_",
		},
		{
			description:    "Test Link",
			text:           "<p><a href=\"http://my.test.link/\" title=\"http://my.test.link/\">my message</a></p>",
			expectedOutput: "[my message](http://my.test.link/ \"http://my.test.link/\")",
		},
		{
			description:    "Test Link no title",
			text:           "<p><a href=\"http://my.test.link/\">my message</a></p>",
			expectedOutput: "[my message](http://my.test.link/)",
		},
		{
			description:    "Test Numbered list",
			text:           "<ol>\n<li><span style=\"font-size:inherit\">One</span></li><li><span style=\"font-size:inherit\">Two</span></li><li><span style=\"font-size:inherit\">Three</span></li><li><span style=\"font-size:inherit\">Four</span></li></ol>",
			expectedOutput: "1. One\n2. Two\n3. Three\n4. Four",
		},
		{
			description:    "Test Bulleted list",
			text:           "<ul>\n<li>bullet one</li><li>bullet two</li><li>bullet three</li></ul>",
			expectedOutput: "- bullet one\n- bullet two\n- bullet three",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			text := ConvertToMD(testCase.text)
			assert.Equal(t, testCase.expectedOutput, text)
		})
	}
}
