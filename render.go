package main

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

func maskPreview(entry Entry) string {
	if normalizeKind(entry.Meta.Kind) != kindSecret {
		return entry.Value
	}
	runes := []rune(entry.Value)
	if len(runes) == 0 {
		return "(empty secret)"
	}
	if len(runes) <= 4 {
		return strings.Repeat("*", len(runes))
	}
	prefixLen := min(4, len(runes))
	suffixLen := min(4, len(runes)-prefixLen)
	maskedLen := len(runes) - prefixLen - suffixLen
	if maskedLen < 4 {
		maskedLen = 4
	}
	return string(runes[:prefixLen]) + strings.Repeat("*", maskedLen) + string(runes[len(runes)-suffixLen:])
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	runes := []rune(value)
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func wrapLines(lines []string, width int) string {
	wrapped := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			wrapped = append(wrapped, "")
			continue
		}
		for part := range strings.SplitSeq(line, "\n") {
			wrapped = append(wrapped, lipgloss.NewStyle().Width(width).Render(part))
		}
	}
	return strings.Join(wrapped, "\n")
}

func lineCount(value string) int {
	return len(strings.Split(value, "\n"))
}

func truncateLines(value string, limit int) string {
	lines := strings.Split(value, "\n")
	if len(lines) <= limit {
		return value
	}
	return strings.Join(lines[:limit], "\n")
}
