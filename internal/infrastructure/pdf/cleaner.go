package pdf

import (
	"regexp"
	"strings"
)

type TextCleaner struct{}

func NewCleaner() Cleaner {
	return &TextCleaner{}
}

func (c *TextCleaner) Clean(text string) string {
	text = c.RemoveHeaders(text)
	text = c.NormalizeSpaces(text)
	return strings.TrimSpace(text)
}

func (c *TextCleaner) RemoveHeaders(text string) string {
	lines := strings.Split(text, "\n")
	var result []string

	pagePattern := regexp.MustCompile(`(?i)^(página|page|pág\.?)\s*\d+`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if pagePattern.MatchString(trimmed) {
			continue
		}
		if len(trimmed) > 0 {
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

func (c *TextCleaner) NormalizeSpaces(text string) string {
	spacePattern := regexp.MustCompile(`[ \t]+`)
	text = spacePattern.ReplaceAllString(text, " ")

	newlinePattern := regexp.MustCompile(`\n{3,}`)
	text = newlinePattern.ReplaceAllString(text, "\n\n")

	return text
}

var _ Cleaner = (*TextCleaner)(nil)
