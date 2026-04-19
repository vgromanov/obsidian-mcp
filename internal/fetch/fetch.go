// Package fetch converts remote HTML to Markdown for the fetch tool.
package fetch

import (
	"strings"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// HTMLToMarkdown converts HTML (or full document) to Markdown.
func HTMLToMarkdown(html string) (string, error) {
	return htmltomd.ConvertString(html)
}

// IsLikelyHTML mirrors upstream heuristics.
func IsLikelyHTML(body, contentType string) bool {
	ct := strings.ToLower(contentType)
	if strings.Contains(strings.ToLower(body), "<html") {
		return true
	}
	if strings.Contains(ct, "text/html") {
		return true
	}
	if contentType == "" {
		return true
	}
	return false
}
