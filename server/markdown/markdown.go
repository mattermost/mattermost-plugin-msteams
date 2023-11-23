package markdown

import (
	"strings"

	"github.com/mattn/godown"
)

var stringsToCheckForHTML = []string{
	"<div",
	"<p ",
	"<p>",
	"<img ",
	"<h",
}

func ConvertToMD(text string) string {
	for _, tag := range stringsToCheckForHTML {
		if strings.Contains(text, tag) {
			var sb strings.Builder
			if err := godown.Convert(&sb, strings.NewReader(text), nil); err != nil {
				return text
			}

			return sb.String()
		}
	}

	return text
}
