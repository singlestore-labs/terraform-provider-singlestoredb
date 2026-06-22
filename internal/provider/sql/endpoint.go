package sql

import (
	"fmt"
	"net"
	"strings"
)

// DataAPIURL returns "https://<host>" given a workspace SQL endpoint.
// Strips any host port suffix (e.g. :3306). Rejects scheme-prefixed values.
func DataAPIURL(sqlEndpoint string) (string, error) {
	trimmed := strings.TrimSpace(sqlEndpoint)
	if trimmed == "" {
		return "", fmt.Errorf("workspace SQL endpoint must not be empty; use singlestoredb_workspace.<n>.endpoint")
	}

	if strings.Contains(trimmed, "://") {
		return "", fmt.Errorf(
			"workspace SQL endpoint must be a host or host:port, not a URL with a scheme; use singlestoredb_workspace.<n>.endpoint",
		)
	}

	host := trimmed
	if h, _, err := net.SplitHostPort(trimmed); err == nil {
		host = h
	}

	if host == "" {
		return "", fmt.Errorf("workspace SQL endpoint must not be empty; use singlestoredb_workspace.<n>.endpoint")
	}

	return "https://" + host, nil
}

// HostFromDataAPIURL extracts the host from a Data API base URL for diagnostics.
func HostFromDataAPIURL(baseURL string) string {
	trimmed := strings.TrimPrefix(baseURL, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	if h, _, err := net.SplitHostPort(trimmed); err == nil {
		return h
	}

	return trimmed
}
