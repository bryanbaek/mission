package llm

import "strings"

func MergeMessageContent(cachedContent, dynamicContent string) string {
	cached := strings.TrimSpace(cachedContent)
	dynamic := strings.TrimSpace(dynamicContent)
	switch {
	case cached == "":
		return dynamic
	case dynamic == "":
		return cached
	default:
		return cached + "\n\n" + dynamic
	}
}
