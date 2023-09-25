package markdown

import (
	"strings"

	"github.com/mattn/godown"
)

var stringsToCheckHTML = map[string]bool{
	"<div":  true,
	"<p ":   true,
	"<p>":   true,
	"<img ": true,
}

func ConvertToMD(text string) string {
	for tag := range stringsToCheckHTML {
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
