package sql

import (
	"fmt"
	"net"
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
	if h, _, err := net.SplitHostPort(endpoint); err == nil {
		host = h
	}

	return "https://" + host, nil
}
