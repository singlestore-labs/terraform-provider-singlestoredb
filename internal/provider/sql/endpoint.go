package sql

import (
	"fmt"
	"net"
	"strings"
)

// DataAPIURL returns "https://<host>" for a workspace SQL endpoint (bare host).
// The Data API is always reached on HTTPS port 443; port suffixes are rejected.
func DataAPIURL(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("workspace SQL endpoint must not be empty; use singlestoredb_workspace.<n>.endpoint")
	}

	if strings.Contains(endpoint, "://") {
		return "", fmt.Errorf(
			"workspace SQL endpoint must be a bare host, not a URL with a scheme; use singlestoredb_workspace.<n>.endpoint",
		)
	}

	host := endpoint
	if strings.ContainsRune(endpoint, ':') {
		_, port, err := net.SplitHostPort(endpoint)
		if err != nil {
			return "", fmt.Errorf("workspace SQL endpoint %q is not a valid host: %w", endpoint, err)
		}

		if port != "" {
			return "", fmt.Errorf(
				"workspace SQL endpoint %q must not include a port; use a bare host (the Data API uses HTTPS on port 443)",
				endpoint,
			)
		}
	}

	if host == "" {
		return "", fmt.Errorf("workspace SQL endpoint %q must include a host", endpoint)
	}

	// Reject paths, query strings, fragments, and embedded credentials so we do
	// not build a malformed base URL like "https://host/foo" and push the error
	// downstream to the request.
	if i := strings.IndexAny(host, "/?#@ "); i >= 0 {
		return "", fmt.Errorf(
			"workspace SQL endpoint %q must be a bare host without a path, query, fragment, or credentials",
			endpoint,
		)
	}

	return "https://" + host, nil
}
