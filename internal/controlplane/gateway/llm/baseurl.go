package llm

import (
	"net/url"
	"strings"
)

// NormalizeBaseURL accepts either an API root or a legacy full endpoint URL and
// returns a base URL compatible with SDK clients that append their own paths.
func NormalizeBaseURL(baseURL, endpointPath string) string {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/")
	endpointPath = strings.TrimRight(strings.TrimSpace(endpointPath), "/")
	if endpointPath != "" && strings.HasSuffix(parsed.Path, endpointPath) {
		parsed.Path = strings.TrimSuffix(parsed.Path, endpointPath)
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	} else {
		parsed.Path += "/"
	}

	return parsed.String()
}
