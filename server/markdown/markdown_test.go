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
			description:    "Text contains div and paragraph tags",
			text:           "This is text area with <div> and <p> tags",
			expectedOutput: "This is text area with \n and \n tags\n\n\n\n\n",
		},
		{
			description:    "Text contains paragraph and image tags",
			text:           "This is text area with <img src=''> and <p class=''>Paragraph</p> tags",
			expectedOutput: "This is text area with  and \nParagraph\n\n\n tags\n",
		},
		{
			description:    "Text contains bold, italics and strike through tags",
			text:           "This is <p><b>bold</b></p>, <p><i>italics</i></p> and <p><s>strike through</s></p> text",
			expectedOutput: "This is \n**bold**\n\n\n, \n_italics_\n\n\n and \n~~strike through~~\n\n\n text\n",
		},
	} {
		t.Run(testCase.description, func(t *testing.T) {
			text := ConvertToMD(testCase.text)
			assert.Equal(t, text, testCase.expectedOutput)
		})
	}
}
