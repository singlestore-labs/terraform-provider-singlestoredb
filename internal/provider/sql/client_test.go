package sql_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/sql"
	"github.com/stretchr/testify/require"
)

func TestClientExec_HappyPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/v2/exec", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "admin", user)
		require.Equal(t, "secret", pass)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"sql":"CREATE DATABASE foo","database":"test"}`, string(body))

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"lastInsertId":1,"rowsAffected":0}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "secret")

	resp, err := client.Exec(t.Context(), sql.ExecRequest{
		SQL:      "CREATE DATABASE foo",
		Database: "test",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.LastInsertID)
	require.Equal(t, int64(0), resp.RowsAffected)
}

func TestClientExec_JWTBasicAuth(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "*", user)
		require.Equal(t, "jwt-token", pass)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"lastInsertId":0,"rowsAffected":1}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "*", "jwt-token")
	resp, err := client.Exec(t.Context(), sql.ExecRequest{SQL: "SELECT 1"})
	require.NoError(t, err)
	require.Equal(t, int64(1), resp.RowsAffected)
}

func TestClientExec_401(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, err := w.Write([]byte("invalid credentials"))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "bad")
	_, err := client.Exec(t.Context(), sql.ExecRequest{SQL: "SELECT 1"})
	require.Error(t, err)

	var apiErr *sql.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusUnauthorized, apiErr.StatusCode)
	require.Equal(t, "invalid credentials", apiErr.Body)
}

func TestClientExec_400PlainText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, err := w.Write([]byte("syntax error"))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "secret")
	_, err := client.Exec(t.Context(), sql.ExecRequest{SQL: "BAD SQL"})
	require.Error(t, err)

	var apiErr *sql.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, "syntax error", apiErr.Body)
}

func TestClientExec_RequestTooLarge(t *testing.T) {
	t.Parallel()

	client := sql.NewClient("https://example.com", "admin", "secret")
	largeSQL := strings.Repeat("x", sql.MaxRequestBodyBytes)

	_, err := client.Exec(t.Context(), sql.ExecRequest{SQL: largeSQL})
	require.Error(t, err)
	require.ErrorAs(t, err, new(sql.RequestTooLargeError))
}

func TestClientExec_InBodyErrorOn200(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/exec", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"error":{"code":1142,"message":"DROP command denied to user 'app'"}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "secret")
	_, err := client.Exec(t.Context(), sql.ExecRequest{SQL: "DROP TABLE t"})
	require.Error(t, err)

	var queryErr *sql.QueryError
	require.ErrorAs(t, err, &queryErr)
	require.Equal(t, "DROP command denied to user 'app'", queryErr.Message)
}

func TestClientQueryRows_HappyPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/query/rows", r.URL.Path)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"sql":"SELECT id FROM users WHERE id = ?","args":["42"]}`, string(body))

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"results":[{"rows":[{"id":42,"name":"alice"}]}]}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "secret")
	resp, err := client.QueryRows(t.Context(), sql.ExecRequest{
		SQL:  "SELECT id FROM users WHERE id = ?",
		Args: []any{"42"},
	})
	require.NoError(t, err)
	require.Len(t, resp.Results, 1)
	require.Len(t, resp.Results[0].Rows, 1)

	stringified, err := sql.StringifyRows(resp.Results[0].Rows)
	require.NoError(t, err)
	require.Equal(t, "42", stringified[0]["id"])
	require.Equal(t, "alice", stringified[0]["name"])
}

func TestClientQueryRows_InBodyErrorOn200(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"error":{"code":1054,"message":"Unknown column 'x'"}}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "secret")
	_, err := client.QueryRows(t.Context(), sql.ExecRequest{SQL: "SELECT x"})
	require.Error(t, err)

	var queryErr *sql.QueryError
	require.ErrorAs(t, err, &queryErr)
	require.Equal(t, "Unknown column 'x'", queryErr.Message)
}

func TestClientHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "secret")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := client.Exec(ctx, sql.ExecRequest{SQL: "SELECT 1"})
	require.Error(t, err)
}
