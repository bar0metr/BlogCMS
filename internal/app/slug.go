package app

import (
	"regexp"
	"strings"
)

var (
	// One or more non-letter/non-digit (Unicode-aware) -> "-"
	reNonAlnum = regexp.MustCompile(`[^\p{L}\p{N}]+`)
	// Multiple "-" -> single "-"
	reDashes = regexp.MustCompile(`-+`)
)

func Slugify(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return "post"
	}

	s = reNonAlnum.ReplaceAllString(s, "-")
	s = reDashes.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")

	if s == "" {
		return "post"
	}
	return s
}
