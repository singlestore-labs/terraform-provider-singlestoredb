package util_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"testing/iotest"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestHTTPClientStatusOK(t *testing.T) {
	body := []byte("fizz")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := util.NewHTTPClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	result, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, result)
}

func TestHTTPClientStatusInternalServerError(t *testing.T) {
	body := []byte("fizz")
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := util.NewHTTPClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	_, err = client.Do(req) //nolint: bodyclose
	require.ErrorContains(t, err, string(body), "returns an error on 500s")
	require.ErrorContains(t, err, http.StatusText(http.StatusInternalServerError))
	require.Greater(t, attempts, 1, "retries 500s")
}

func TestHTTPClientStatusConflict(t *testing.T) {
	body := []byte("insufficient credits")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, err := w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := util.NewHTTPClient()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err, "returns no error and a body on not 500s")
	defer resp.Body.Close()
	result, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, body, result)
}

func TestHandleError(t *testing.T) {
	readErr := errors.New("failed to read")
	extra := errors.New("extra")
	numTries := 3
	_, err := util.HandleError(&http.Response{Body: io.NopCloser(iotest.ErrReader(readErr))}, extra, numTries) //nolint: bodyclose
	require.ErrorContains(t, err, readErr.Error())
	require.ErrorContains(t, err, extra.Error())
	require.ErrorContains(t, err, strconv.Itoa(numTries))
}
