package markdown

import (
	"strings"

	"github.com/mattn/godown"
)

var stringsToCheckHTML = []string{
	"<div",
	"<p ",
	"<p>",
	"<img ",
}

func ConvertToMD(text string) string {
	for _, tag := range stringsToCheckHTML {
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
