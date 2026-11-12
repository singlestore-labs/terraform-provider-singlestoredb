package privateconnectionsv2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestClassifyGet(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	active := management.ClusterPrivateConnectionStatusACTIVE
	pending := management.ClusterPrivateConnectionStatusPENDING
	deleted := management.ClusterPrivateConnectionStatusDELETED
	conn := &management.ClusterPrivateConnection{PrivateConnectionID: id, Status: &active}

	tests := []struct {
		name      string
		code      int
		body      *management.ClusterPrivateConnection
		mode      waitMode
		wantReady bool
		retryable *bool
		contains  string
	}{
		{name: "active", code: http.StatusOK, body: conn, mode: waitUntilActive, wantReady: true},
		{name: "pending", code: http.StatusOK, body: &management.ClusterPrivateConnection{PrivateConnectionID: id, Status: &pending}, mode: waitUntilActive, wantReady: true},
		{name: "deleted stops create", code: http.StatusOK, body: &management.ClusterPrivateConnection{PrivateConnectionID: id, Status: &deleted}, mode: waitUntilActive, retryable: util.Ptr(false), contains: "DELETED"},
		{name: "deleted completes delete", code: http.StatusOK, body: &management.ClusterPrivateConnection{PrivateConnectionID: id, Status: &deleted}, mode: waitUntilDeleted, wantReady: true},
		{name: "deleted is readable", code: http.StatusOK, body: &management.ClusterPrivateConnection{PrivateConnectionID: id, Status: &deleted}, mode: readExisting, wantReady: true},
		{name: "unauthorized stops", code: http.StatusUnauthorized, mode: waitUntilActive, retryable: util.Ptr(false), contains: "authentication or authorization"},
		{name: "forbidden stops", code: http.StatusForbidden, mode: waitUntilActive, retryable: util.Ptr(false), contains: "authentication or authorization"},
		{name: "server error retries", code: http.StatusBadGateway, mode: waitUntilActive, retryable: util.Ptr(true), contains: "Bad Gateway"},
		{name: "empty json retries", code: http.StatusOK, mode: waitUntilActive, retryable: util.Ptr(true), contains: "not a JSON"},
		{name: "not found retries", code: http.StatusNotFound, mode: waitUntilActive, retryable: util.Ptr(true), contains: "Not Found"},
		{name: "not found completes delete", code: http.StatusNotFound, mode: waitUntilDeleted},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tracker := &notFoundTracker{grace: time.Minute, now: func() time.Time { return time.Unix(0, 0) }}
			got := classifyGet(id, test.code, test.body, "text/html", tracker, test.mode)
			if test.wantReady {
				require.NotNil(t, got.ready)
				require.Nil(t, got.retry)

				return
			}
			if test.retryable == nil {
				require.Nil(t, got.retry)
				require.Nil(t, got.ready)

				return
			}
			require.NotNil(t, got.retry)
			require.Equal(t, *test.retryable, got.retry.Retryable)
			require.ErrorContains(t, got.retry.Err, test.contains)
		})
	}
}

func TestNotFoundGraceStops(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 0)
	tracker := &notFoundTracker{grace: time.Minute, now: func() time.Time { return now }}
	id := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")

	first := classifyGet(id, http.StatusNotFound, nil, "", tracker, waitUntilActive)
	require.True(t, first.retry.Retryable)

	now = now.Add(time.Minute)
	second := classifyGet(id, http.StatusNotFound, nil, "", tracker, waitUntilActive)
	require.False(t, second.retry.Retryable)
	require.ErrorContains(t, second.retry.Err, "deleted outside Terraform")

	// A successful response restarts the grace.
	tracker.found()
	now = now.Add(time.Second)
	restarted := classifyGet(id, http.StatusNotFound, nil, "", tracker, readExisting)
	require.True(t, restarted.retry.Retryable)
}

func TestWaitConditionStatusIsStable(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	active := management.ClusterPrivateConnectionStatusACTIVE
	pending := management.ClusterPrivateConnectionStatusPENDING
	condition := waitConditionStatus(active)
	conn := management.ClusterPrivateConnection{PrivateConnectionID: id, Status: &pending}
	require.ErrorContains(t, condition(conn), "PENDING")

	conn.Status = &active
	for range 4 {
		require.ErrorContains(t, condition(conn), "consequent")
	}
	require.NoError(t, condition(conn))
}

func TestWaitConditionAllowListAndRegions(t *testing.T) {
	t.Parallel()

	id := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	current := "123"
	conn := management.ClusterPrivateConnection{PrivateConnectionID: id, AllowList: &current}
	require.ErrorContains(t, waitConditionAllowList("456")(conn), "should be 456")
	require.NoError(t, waitConditionAllowList(current)(conn))

	regions := []string{"b", "a"}
	conn.SupportedCrossRegions = &regions
	require.NoError(t, waitConditionRegions([]string{"a", "b"})(conn))
	require.Error(t, waitConditionRegions([]string{"a"})(conn))
	require.NoError(t, waitConditionRegions(nil)(management.ClusterPrivateConnection{PrivateConnectionID: id}))
}

func TestWaitStopsOnUnauthorizedWithoutRetryingUntilTimeout(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	_, serr := wait(t.Context(), client, uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"), time.Hour, waitUntilActive, waitConditionStatus(management.ClusterPrivateConnectionStatusACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "authentication or authorization")
	require.Equal(t, 1, calls)
}

func TestWaitTimeoutIncludesLastStatus(t *testing.T) {
	t.Parallel()

	pending := management.ClusterPrivateConnectionStatusPENDING
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(mustConnectionJSON(t, management.ClusterPrivateConnection{
			PrivateConnectionID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
			Status:              &pending,
		}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	_, serr := wait(t.Context(), client, uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"), time.Second, waitUntilActive, waitConditionStatus(management.ClusterPrivateConnectionStatusACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "PENDING")
	require.Contains(t, serr.Detail, "within")
}

func TestReadReturnsConnectionThatAppearsDuringGrace(t *testing.T) {
	// The helper returns the connection when it appears before the grace ends.
	active := management.ClusterPrivateConnectionStatusACTIVE
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusNotFound)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		body := mustConnectionJSON(t, management.ClusterPrivateConnection{
			PrivateConnectionID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
			Status:              &active,
		})
		_, err := w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	prev := notFoundGrace
	notFoundGrace = 2 * time.Second
	t.Cleanup(func() { notFoundGrace = prev })

	conn, notFound, serr := readConnection(t.Context(), client, uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"))
	require.Nil(t, serr)
	require.False(t, notFound)
	require.NotNil(t, conn)
	require.GreaterOrEqual(t, calls, 2)
}

func mustConnectionJSON(t *testing.T, conn management.ClusterPrivateConnection) []byte {
	t.Helper()

	body, err := json.Marshal(conn)
	require.NoError(t, err)

	return body
}
