package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	listItem = regexp.MustCompile(`^[[:space:]]*(?:[-+*]|[0-9]+\.)[[:space:]]+`)
	heading  = regexp.MustCompile(`^[[:space:]]*#{1,6}[[:space:]]+`)
	ruleLine = regexp.MustCompile(`^[-*_]{3,}$`)
)

// markdownLineKind classifies lines relevant to the one-physical-line contract.
func markdownLineKind(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case trimmed == "":
		return "blank"
	case strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "-->"):
		return "markup"
	case heading.MatchString(line):
		return "heading"
	case listItem.MatchString(line):
		return "list"
	case strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|"):
		return "table"
	case strings.HasPrefix(trimmed, ">"):
		return "quote"
	case ruleLine.MatchString(trimmed):
		return "rule"
	default:
		return "prose"
	}
}

// checkSingleLineMarkdown reports prose or list continuations outside fenced blocks.
func checkSingleLineMarkdown(root string, markdownFiles []string) []string {
	var failures []string
	for _, relativePath := range markdownFiles {
		data, err := os.ReadFile(filepath.Join(root, relativePath))
		if err != nil {
			continue
		}
		fence := ""
		previousKind := "blank"
		for index, line := range strings.Split(string(data), "\n") {
			trimmedLeft := strings.TrimLeft(line, " \t")
			marker := ""
			if len(trimmedLeft) >= 3 && (strings.HasPrefix(trimmedLeft, "```") || strings.HasPrefix(trimmedLeft, "~~~")) {
				marker = trimmedLeft[:3]
			}
			if marker != "" {
				if fence == "" {
					fence = marker
				} else if marker == fence {
					fence = ""
				}
				previousKind = "blank"
				continue
			}
			if fence != "" {
				continue
			}
			kind := markdownLineKind(line)
			if kind == "prose" && (previousKind == "prose" || previousKind == "list" || previousKind == "table") {
				failures = append(failures, fmt.Sprintf("%s:%d: prose paragraphs and list or table rows must remain on one physical line", filepath.ToSlash(relativePath), index+1))
			}
			previousKind = kind
		}
	}
	return failures
}
