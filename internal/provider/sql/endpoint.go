package sql

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// DataAPIURL returns "https://<host>" for a workspace SQL endpoint (host or host:port).
// Any port suffix is stripped; the Data API is always reached on HTTPS port 443.
func DataAPIURL(endpoint string) (string, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return "", fmt.Errorf("workspace SQL endpoint must not be empty; use singlestoredb_workspace.<n>.endpoint")
	}

	if strings.Contains(endpoint, "://") {
		return "", fmt.Errorf(
			"workspace SQL endpoint must be a host or host:port, not a URL with a scheme; use singlestoredb_workspace.<n>.endpoint",
		)
	}

	host := endpoint
	if strings.ContainsRune(endpoint, ':') {
		h, port, err := net.SplitHostPort(endpoint)
		if err != nil {
			return "", fmt.Errorf("workspace SQL endpoint %q is not a valid host or host:port: %w", endpoint, err)
		}

		if port != "" {
			if _, err := strconv.Atoi(port); err != nil {
				return "", fmt.Errorf("workspace SQL endpoint %q has an invalid port %q; use a host or host:port", endpoint, port)
			}
		}

		host = h
	}

	if host == "" {
		return "", fmt.Errorf("workspace SQL endpoint %q must include a host", endpoint)
	}

	// Reject paths, query strings, fragments, and embedded credentials so we do
	// not build a malformed base URL like "https://host/foo" and push the error
	// downstream to the request.
	if i := strings.IndexAny(host, "/?#@ "); i >= 0 {
		return "", fmt.Errorf(
			"workspace SQL endpoint %q must be a bare host or host:port without a path, query, fragment, or credentials",
			endpoint,
		)
	}

	return "https://" + host, nil
}
