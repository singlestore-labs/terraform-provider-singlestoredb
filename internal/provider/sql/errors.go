package sql

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// APIError is returned when the Data API responds with a non-2xx HTTP status.
type APIError struct {
	StatusCode int
	Body       string
	Host       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("data api error %d: %s", e.StatusCode, e.Body)
}

// RequestTooLargeError is returned when a request body exceeds MaxRequestBodyBytes.
type RequestTooLargeError struct{}

func (RequestTooLargeError) Error() string {
	return "sql statement exceeds the data api 1 mb request limit"
}

// QueryError is returned when query/rows responds with HTTP 200 but an in-body error field.
type QueryError struct {
	Message string
	Host    string
}

func (e *QueryError) Error() string {
	return e.Message
}

// ExecError is returned when /exec responds with HTTP 200 but an in-body error field.
// It mirrors the HTTP 4xx exec failure so execute/revert statements get a
// consistent "SQL execution failed" diagnostic regardless of how the Data API
// reports the failure.
type ExecError struct {
	Message string
	Host    string
}

func (e *ExecError) Error() string {
	return e.Message
}

// DiagnosticFromError maps Data API client errors to provider diagnostics.
func DiagnosticFromError(err error) *util.SummaryWithDetailError {
	if err == nil {
		return nil
	}

	var tooLarge RequestTooLargeError
	if errors.As(err, &tooLarge) {
		return &util.SummaryWithDetailError{
			Summary: "SQL statement exceeds the Data API 1 MB request limit",
			Detail:  "Shorten the statement or split across multiple singlestoredb_sql_execute resources.",
		}
	}

	var execErr *ExecError
	if errors.As(err, &execErr) {
		return &util.SummaryWithDetailError{
			Summary: "SQL execution failed",
			Detail:  execErr.Message,
		}
	}

	var queryErr *QueryError
	if errors.As(err, &queryErr) {
		return &util.SummaryWithDetailError{
			Summary: "SQL query failed",
			Detail:  queryErr.Message,
		}
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return diagnosticFromAPIError(apiErr)
	}

	if IsUnreachable(err) {
		host := unreachableHost(err)

		return &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Could not reach the SingleStore Data API at %s", host),
			Detail:  "Verify the endpoint and that the workspace is active (not suspended).",
		}
	}

	return &util.SummaryWithDetailError{
		Summary: "SingleStore Data API client call failed",
		Detail:  err.Error(),
	}
}

func diagnosticFromAPIError(apiErr *APIError) *util.SummaryWithDetailError {
	host := apiErr.Host
	if host == "" {
		host = "workspace"
	}

	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Invalid SingleStore SQL credentials for %s", host),
			Detail:  "Check username and password (or JWT JWKS when username is \"*\").",
		}
	case http.StatusBadRequest, http.StatusInternalServerError:
		return &util.SummaryWithDetailError{
			Summary: "SQL execution failed",
			Detail:  apiErr.Body,
		}
	default:
		if apiErr.StatusCode == http.StatusServiceUnavailable {
			return &util.SummaryWithDetailError{
				Summary: fmt.Sprintf("Could not reach the SingleStore Data API at %s", host),
				Detail:  "Verify the endpoint and that the workspace is active (not suspended).",
			}
		}

		return &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("SingleStore Data API returned status %s", http.StatusText(apiErr.StatusCode)),
			Detail:  apiErr.Body,
		}
	}
}

// InvalidEndpointDiagnostic maps DataAPIURL validation failures.
func InvalidEndpointDiagnostic(err error) *util.SummaryWithDetailError {
	return &util.SummaryWithDetailError{
		Summary: "Invalid workspace SQL endpoint",
		Detail:  err.Error(),
	}
}

// IsUnreachable reports whether err indicates the workspace Data API is unreachable.
// Used during destroy to succeed silently when the workspace is already gone.
func IsUnreachable(err error) bool {
	if err == nil {
		return false
	}

	// A canceled context means the caller aborted (e.g. Terraform is shutting
	// down), not that the workspace is gone; do not skip revert for it.
	if errors.Is(err, context.Canceled) {
		return false
	}

	if isUnreachableNetworkError(err) {
		return true
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout")
}

// isUnreachableNetworkError reports whether err is a typed error that indicates
// the workspace could not be reached.
func isUnreachableNetworkError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusServiceUnavailable
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Only connection-establishment (dial) failures indicate the workspace is
	// unreachable. TLS handshake errors, abrupt EOFs, and read/write failures
	// can occur against a reachable server, so they must not silently skip
	// revert and drop the resource from state.
	var opErr *net.OpError

	return errors.As(err, &opErr) && opErr.Op == "dial"
}

func unreachableHost(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Host != "" {
		return apiErr.Host
	}

	return "workspace"
}
