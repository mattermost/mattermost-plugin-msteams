package markdown

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	plugin "github.com/JohannesKaufmann/html-to-markdown/plugin"
)

var stringsToCheckForHTML = []string{
	"<div",
	"<p ",
	"<p>",
	"<img ",
	"<h1>",
	"<h2>",
	"<h3>",
	"<ol>",
	"<ul>",
}

func ConvertToMD(text string) string {
	for _, tag := range stringsToCheckForHTML {
		if strings.Contains(text, tag) {
			converter := md.NewConverter("", true, nil)
			converter.Use(plugin.GitHubFlavored())
			markdown, err := converter.ConvertString(text)
			if err != nil {
				return text
			}
			return markdown
		}
	}

	return text
}
