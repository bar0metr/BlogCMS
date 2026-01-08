package web

import (
	"html"
	"regexp"
	"strings"
)

var reHTMLTags = regexp.MustCompile(`<[^>]+>`)

func excerptFromHTML(s string) string {
	// Very lightweight excerpt generator: strip tags, unescape entities, collapse
	// whitespace, then return the first ~2 sentences or ~220 chars.
	plain := reHTMLTags.ReplaceAllString(s, " ")
	plain = html.UnescapeString(plain)
	plain = strings.Join(strings.Fields(plain), " ")
	if plain == "" {
		return ""
	}

	// Prefer sentence-ish cut.
	cut := -1
	for i := 0; i < len(plain) && i < 260; i++ {
		if plain[i] == '.' || plain[i] == '!' || plain[i] == '?' {
			// Take up to 2 sentences.
			if cut == -1 {
				cut = i + 1
				continue
			}
			cut = i + 1
			break
		}
	}
	if cut == -1 {
		if len(plain) > 220 {
			cut = 220
		} else {
			cut = len(plain)
		}
	}
	plain = strings.TrimSpace(plain[:cut])
	return plain
}
