package content

import (
	"strings"
)

func AppendHashtags(text string, tags []string) string {
	normalized := normalizeTags(tags)
	if len(normalized) == 0 {
		return strings.TrimSpace(text)
	}

	current := strings.TrimSpace(text)
	if current == "" {
		return strings.Join(formatTags(normalized), " ")
	}

	return current + "\n" + strings.Join(formatTags(normalized), " ")
}

func normalizeTags(tags []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		trimmed := strings.TrimSpace(strings.TrimPrefix(t, "#"))
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func formatTags(tags []string) []string {
	formatted := make([]string, 0, len(tags))
	for _, t := range tags {
		formatted = append(formatted, "#"+t)
	}
	return formatted
}
