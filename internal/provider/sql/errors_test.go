package sql_test

import (
	"errors"
	"net"
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/sql"
	"github.com/stretchr/testify/require"
)

func TestDiagnosticFromError_API401(t *testing.T) {
	t.Parallel()

	diag := sql.DiagnosticFromError(&sql.APIError{
		StatusCode: 401,
		Body:       "unauthorized",
		Host:       "svc.example.com",
	})
	require.NotNil(t, diag)
	require.Contains(t, diag.Summary, "svc.example.com")
	require.Contains(t, diag.Detail, "username and password")
}

func TestDiagnosticFromError_Exec400(t *testing.T) {
	t.Parallel()

	diag := sql.DiagnosticFromError(&sql.APIError{
		StatusCode: 400,
		Body:       "syntax error near 'FOO'",
		Host:       "svc.example.com",
	})
	require.NotNil(t, diag)
	require.Equal(t, "SQL execution failed", diag.Summary)
	require.Equal(t, "syntax error near 'FOO'", diag.Detail)
}

func TestDiagnosticFromError_RequestTooLarge(t *testing.T) {
	t.Parallel()

	diag := sql.DiagnosticFromError(sql.RequestTooLargeError{})
	require.NotNil(t, diag)
	require.Contains(t, diag.Summary, "1 MB")
}

func TestDiagnosticFromError_QueryInBody(t *testing.T) {
	t.Parallel()

	diag := sql.DiagnosticFromError(&sql.QueryError{Message: "table not found"})
	require.NotNil(t, diag)
	require.Equal(t, "SQL query failed", diag.Summary)
	require.Equal(t, "table not found", diag.Detail)
}

func TestDiagnosticFromError_Unreachable503(t *testing.T) {
	t.Parallel()

	diag := sql.DiagnosticFromError(&sql.APIError{
		StatusCode: 503,
		Body:       "unavailable",
		Host:       "svc.example.com",
	})
	require.NotNil(t, diag)
	require.Contains(t, diag.Summary, "Could not reach")
	require.Contains(t, diag.Detail, "active (not suspended)")
}

func TestDiagnosticFromError_ConnectionRefused(t *testing.T) {
	t.Parallel()

	diag := sql.DiagnosticFromError(errors.New("dial tcp: connection refused"))
	require.NotNil(t, diag)
	require.Contains(t, diag.Summary, "Could not reach")
}

func TestIsUnreachable(t *testing.T) {
	t.Parallel()

	require.True(t, sql.IsUnreachable(&sql.APIError{StatusCode: 503}))
	require.True(t, sql.IsUnreachable(&net.DNSError{IsNotFound: true}))
	require.True(t, sql.IsUnreachable(errors.New("dial tcp: connection refused")))
	require.False(t, sql.IsUnreachable(&sql.APIError{StatusCode: 401}))
	require.False(t, sql.IsUnreachable(nil))
}

func TestInvalidEndpointDiagnostic(t *testing.T) {
	t.Parallel()

	diag := sql.InvalidEndpointDiagnostic(errors.New("bad endpoint"))
	require.Equal(t, "Invalid workspace SQL endpoint", diag.Summary)
	require.Equal(t, "bad endpoint", diag.Detail)
}

func TestErrorMessages(t *testing.T) {
	t.Parallel()

	apiErr := &sql.APIError{StatusCode: 500, Body: "boom", Host: "svc.example.com"}
	require.Equal(t, "data api error 500: boom", apiErr.Error())

	require.Equal(t, "sql statement exceeds the data api 1 mb request limit", sql.RequestTooLargeError{}.Error())

	queryErr := &sql.QueryError{Message: "table not found", Host: "svc.example.com"}
	require.Equal(t, "table not found", queryErr.Error())
}
