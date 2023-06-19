package markdown

import (
	"strings"

	"github.com/mattn/godown"
)

func ConvertToMD(text string) string {
	if !strings.Contains(text, "<div>") && !strings.Contains(text, "<p>") {
		return text
	}
	var sb strings.Builder
	if err := godown.Convert(&sb, strings.NewReader(text), nil); err != nil {
		return text
	}
	return sb.String()
}
