package utils

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
			text := ConvertToMD(testCase.text)
			assert.Equal(t, text, testCase.expectedOutput)
		})
	}
}
