package clusters

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestWaitConditions(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	size := "S-00"
	cluster := management.Cluster{
		ClusterID: &clusterID,
		SizeConfig: &management.SizeConfig{
			Size: &size,
		},
		FirewallRanges: util.Ptr([]string{"10.0.0.0/8"}),
	}

	require.NoError(t, waitConditionSize(size)(cluster))
	require.Error(t, waitConditionSize("S-1")(cluster))
	require.Error(t, waitConditionSize(size)(management.Cluster{ClusterID: &clusterID}))

	require.NoError(t, waitConditionFirewallRanges([]types.String{types.StringValue("10.0.0.0/8")})(cluster))
	require.Error(t, waitConditionFirewallRanges([]types.String{types.StringValue("192.168.0.0/16")})(cluster))

	require.NoError(t, waitConditionTakesAtLeast(-time.Second)(cluster))
	require.Error(t, waitConditionTakesAtLeast(time.Hour)(cluster))
}

func TestWaitClusterFailures(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	failed := management.ClusterStateFAILED

	failedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(management.Cluster{ClusterID: &clusterID, State: &failed})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(failedServer.Close)

	failedClient, err := management.NewClientWithResponses(failedServer.URL)
	require.NoError(t, err)
	_, serr := wait(t.Context(), failedClient, clusterID, time.Second, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "failed")

	terminatedGets := 0
	terminated := management.ClusterStateTERMINATED
	terminatedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		terminatedGets++
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(management.Cluster{ClusterID: &clusterID, State: &terminated})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(terminatedServer.Close)
	terminatedClient, err := management.NewClientWithResponses(terminatedServer.URL)
	require.NoError(t, err)
	// A terminated cluster fails the wait on the first poll instead of at the timeout.
	_, serr = wait(t.Context(), terminatedClient, clusterID, time.Hour, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "terminated")
	require.Equal(t, 1, terminatedGets)

	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(statusServer.Close)
	statusClient, err := management.NewClientWithResponses(statusServer.URL)
	require.NoError(t, err)
	_, serr = wait(t.Context(), statusClient, clusterID, time.Millisecond, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)

	notFoundGets := 0
	notFoundServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		notFoundGets++
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(notFoundServer.Close)
	notFoundClient, err := management.NewClientWithResponses(notFoundServer.URL)
	require.NoError(t, err)
	// A deleted cluster fails the wait after the not-found grace instead of at the timeout.
	setNotFoundGrace(t, 600*time.Millisecond)
	start := time.Now()
	_, serr = wait(t.Context(), notFoundClient, clusterID, time.Hour, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "deleted outside Terraform")
	require.Greater(t, notFoundGets, 1)
	require.Less(t, time.Since(start), 10*time.Second)

	down, err := management.NewClientWithResponses("http://127.0.0.1:1")
	require.NoError(t, err)
	_, serr = wait(t.Context(), down, clusterID, time.Second, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "failed to get cluster")
}

// TestWaitRetriesTransportErrors retries a transport error until the cluster can be read.
func TestWaitRetriesTransportErrors(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	active := management.ClusterStateACTIVE
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(management.Cluster{ClusterID: &clusterID, State: &active})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	attempts := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, errors.New("connection reset by peer")
		}

		return http.DefaultTransport.RoundTrip(r)
	})}
	client, err := management.NewClientWithResponses(server.URL, management.WithHTTPClient(httpClient))
	require.NoError(t, err)

	got, serr := wait(t.Context(), client, clusterID, 5*time.Second, func(management.Cluster) error { return nil })
	require.Nil(t, serr)
	require.Equal(t, clusterID, util.Deref(got.ClusterID))
	require.Greater(t, attempts, 1)
}

func TestWaitStopsWhenContextIsCanceled(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	client, err := management.NewClientWithResponses("http://127.0.0.1:1")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	_, serr := wait(ctx, client, clusterID, time.Minute, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "canceled")
	require.Less(t, time.Since(start), 2*time.Second)
}

func TestClusterPollError(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	canceled := clusterPollError(ctx, clusterID, errors.New("connection reset by peer"))
	require.False(t, canceled.Retryable)
	require.ErrorContains(t, canceled.Err, "failed to get cluster")

	retryable := clusterPollError(t.Context(), clusterID, errors.New("connection reset by peer"))
	require.True(t, retryable.Retryable)
	require.ErrorContains(t, retryable.Err, "connection reset by peer")
}

func setNotFoundGrace(t *testing.T, d time.Duration) {
	t.Helper()
	prev := clusterNotFoundGrace
	clusterNotFoundGrace = d
	t.Cleanup(func() { clusterNotFoundGrace = prev })
}

// TestNotFoundTracker tolerates 404s for the grace and restarts it after any other response.
func TestNotFoundTracker(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tracker := &notFoundTracker{grace: time.Minute, now: func() time.Time { return now }}

	require.False(t, tracker.notFound())
	now = now.Add(59 * time.Second)
	require.False(t, tracker.notFound())

	// A read in between restarts the grace.
	tracker.found()
	now = now.Add(time.Second)
	require.False(t, tracker.notFound())
	now = now.Add(59 * time.Second)
	require.False(t, tracker.notFound())
	now = now.Add(time.Second)
	require.True(t, tracker.notFound())
}

// TestWaitNotFoundResets keeps waiting when 404s are interrupted by a successful read.
func TestWaitNotFoundResets(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	pending := management.ClusterStatePENDING
	active := management.ClusterStateACTIVE
	setNotFoundGrace(t, 3*time.Second)

	// Polls back off to 0s, 0.5s, 1.5s, 3.5s, 7.5s: 404, PENDING, 404, 404, ACTIVE.
	// The last run of 404s spans 2s, under the grace; without the PENDING reset the
	// 404s would span 3.5s and fail the wait.
	responses := []*management.ClusterState{nil, &pending, nil, nil, &active}
	gets := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		state := responses[min(gets, len(responses)-1)]
		gets++
		if state == nil {
			w.WriteHeader(http.StatusNotFound)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(mustJSON(t, management.Cluster{ClusterID: &clusterID, State: state}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	activeNow := func(c management.Cluster) error {
		if util.Deref(c.State) != management.ClusterStateACTIVE {
			return errors.New("not active")
		}

		return nil
	}
	got, serr := wait(t.Context(), client, clusterID, time.Minute, activeNow)
	require.Nil(t, serr)
	require.Equal(t, management.ClusterStateACTIVE, util.Deref(got.State))
	require.Equal(t, len(responses), gets)
}

// TestWaitRetriesNonJSONOK retries a 200 response without a JSON body instead of panicking.
func TestWaitRetriesNonJSONOK(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	active := management.ClusterStateACTIVE
	gets := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		gets++
		if gets == 1 {
			w.Header().Set("Content-Type", "text/html")
			_, err := w.Write([]byte("<html>proxy</html>"))
			require.NoError(t, err)

			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(mustJSON(t, management.Cluster{ClusterID: &clusterID, State: &active}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	_, serr := wait(t.Context(), client, clusterID, time.Minute, func(management.Cluster) error { return nil })
	require.Nil(t, serr)
	require.Equal(t, 2, gets)
}

// TestWaitTerminatedRetriesNonJSONOK retries a 200 response without a JSON body instead of panicking.
func TestWaitTerminatedRetriesNonJSONOK(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write([]byte("<html>proxy</html>"))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	serr := waitTerminated(t.Context(), client, clusterID, 50*time.Millisecond)
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "not a JSON cluster")
}

// TestWaitErrorDetail reports a timeout or cancellation instead of only the last poll error.
func TestWaitErrorDetail(t *testing.T) {
	last := errors.New("cluster x state is PENDING, but should be ACTIVE")

	timedOut := waitErrorDetail(t.Context(), time.Hour, false, last, "Cluster did not reach the expected state")
	require.Equal(t, "Cluster did not reach the expected state within 1h0m0s: "+last.Error(), timedOut)

	require.Equal(t, last.Error(), waitErrorDetail(t.Context(), time.Hour, true, last, "unused"))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.Contains(t, waitErrorDetail(ctx, time.Hour, true, last, "unused"), "canceled")
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	require.NoError(t, err)

	return body
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// TestWaitTerminated finishes a delete once the cluster is terminated.
func TestWaitTerminated(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	active := management.ClusterStateACTIVE
	terminated := management.ClusterStateTERMINATED

	gets := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		gets++
		state := active
		if gets > 1 {
			state = terminated
		}
		body, err := json.Marshal(management.Cluster{ClusterID: &clusterID, State: &state})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)
	require.Nil(t, waitTerminated(t.Context(), client, clusterID, time.Minute))
	require.Equal(t, 2, gets)

	notFound := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(notFound.Close)
	notFoundClient, err := management.NewClientWithResponses(notFound.URL)
	require.NoError(t, err)
	require.Nil(t, waitTerminated(t.Context(), notFoundClient, clusterID, time.Minute))

	stuck := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(management.Cluster{ClusterID: &clusterID, State: &active})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(stuck.Close)
	stuckClient, err := management.NewClientWithResponses(stuck.URL)
	require.NoError(t, err)
	serr := waitTerminated(t.Context(), stuckClient, clusterID, time.Millisecond)
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "did not terminate within 1ms")
}

// TestClusterReadStateDiagnostics keeps a cluster that is not terminated in state.
func TestClusterReadStateDiagnostics(t *testing.T) {
	id := "3ca3d359-021d-45ed-86cb-38b8d14ac507"
	for _, state := range []management.ClusterState{management.ClusterStateACTIVE, management.ClusterStateSUSPENDED, management.ClusterStateTERMINATED} {
		require.Empty(t, clusterReadStateDiagnostics(id, state))
	}
	for _, state := range []management.ClusterState{management.ClusterStatePENDING, management.ClusterStateFAILED, management.ClusterStateUNKNOWN, ""} {
		diags := clusterReadStateDiagnostics(id, state)
		require.False(t, diags.HasError(), "state %q must not block refresh", state)
		require.Equal(t, 1, diags.WarningsCount())
	}
	require.Contains(t, clusterReadStateDiagnostics(id, management.ClusterStateFAILED)[0].Detail(), "-replace")
}

// TestTerminatedAtIsTerminal stops a wait when the cluster has terminatedAt set.
func TestTerminatedAtIsTerminal(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	terminatedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	for _, state := range []management.ClusterState{management.ClusterStateFAILED, management.ClusterStatePENDING} {
		t.Run(string(state), func(t *testing.T) {
			gets := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				gets++
				w.Header().Set("Content-Type", "application/json")
				body, err := json.Marshal(management.Cluster{ClusterID: &clusterID, State: &state, TerminatedAt: &terminatedAt})
				require.NoError(t, err)
				_, err = w.Write(body)
				require.NoError(t, err)
			}))
			t.Cleanup(server.Close)
			client, err := management.NewClientWithResponses(server.URL)
			require.NoError(t, err)

			require.True(t, isTerminatedCluster(management.Cluster{State: &state, TerminatedAt: &terminatedAt}))
			require.False(t, isTerminatedCluster(management.Cluster{State: &state}))

			// Delete finishes on the first poll instead of waiting for TERMINATED.
			require.Nil(t, waitTerminated(t.Context(), client, clusterID, time.Hour))
			require.Equal(t, 1, gets)

			// Create and update waits stop instead of polling until the timeout.
			_, serr := wait(t.Context(), client, clusterID, time.Hour, waitConditionState(management.ClusterStateACTIVE))
			require.NotNil(t, serr)
			require.Contains(t, serr.Detail, "terminated at 2026-01-02T03:04:05Z")
			require.Equal(t, 2, gets)
		})
	}
}

func TestCreateDefersMaxScaleFactorFour(t *testing.T) {
	plan := func(maxScaleFactor float32) clusterResourceModel {
		return clusterResourceModel{AutoScale: &autoScaleResourceModel{
			MaxScaleFactor: types.Float32Value(maxScaleFactor),
			Sensitivity:    types.StringValue(string(management.HIGH)),
		}}
	}

	require.Nil(t, toCreateAutoScale(plan(scaleX1)))
	require.Nil(t, toCreateAutoScale(plan(scaleX4)), "POST stores only the 4x factor, so 4 is set with PATCH")
	require.Equal(t, float32(scaleX2), *toCreateAutoScale(plan(scaleX2)).MaxScaleFactor)
	require.True(t, createDefersAutoScale(plan(scaleX4)))
	require.False(t, createDefersAutoScale(plan(scaleX2)))
}

// TestApplyAutoScaleAfterCreate patches a deferred max scale factor of 4 after create.
func TestApplyAutoScaleAfterCreate(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	projectID := uuid.New()
	active := management.ClusterStateACTIVE
	plan := clusterResourceModel{AutoScale: &autoScaleResourceModel{
		MaxScaleFactor: types.Float32Value(scaleX4),
		Sensitivity:    types.StringValue(string(management.HIGH)),
	}}
	created := management.Cluster{ClusterID: &clusterID, Name: "cluster-1", ProjectID: projectID, State: &active}

	t.Run("success", func(t *testing.T) {
		var patched *management.Cluster
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.Method == http.MethodPatch {
				patched = &management.Cluster{}
				require.NoError(t, json.NewDecoder(r.Body).Decode(patched))
				_, err := w.Write([]byte(`{}`))
				require.NoError(t, err)

				return
			}
			require.Equal(t, http.MethodGet, r.Method)
			current := created
			if patched != nil {
				current.AutoScale = patched.AutoScale
			}
			body, err := json.Marshal(current)
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)
		}))
		t.Cleanup(server.Close)
		client, err := management.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		var diags diag.Diagnostics
		result, applied := applyAutoScaleAfterCreate(t.Context(), client, clusterID, created, plan, &diags)
		require.True(t, applied)
		require.False(t, diags.HasError())
		require.Empty(t, diags.Warnings())
		require.NotNil(t, patched)
		require.Equal(t, "cluster-1", patched.Name)
		require.Equal(t, projectID, patched.ProjectID)
		require.Equal(t, float32(scaleX4), *patched.AutoScale.MaxScaleFactor)
		require.Equal(t, management.HIGH, *patched.AutoScale.Sensitivity)
		require.Equal(t, float32(scaleX4), *result.AutoScale.MaxScaleFactor)
	})

	t.Run("not deferred", func(t *testing.T) {
		client, err := management.NewClientWithResponses("http://127.0.0.1:1")
		require.NoError(t, err)
		notDeferred := clusterResourceModel{AutoScale: &autoScaleResourceModel{MaxScaleFactor: types.Float32Value(scaleX2)}}

		var diags diag.Diagnostics
		result, applied := applyAutoScaleAfterCreate(t.Context(), client, clusterID, created, notDeferred, &diags)
		require.True(t, applied)
		require.Equal(t, created, result)
		require.Empty(t, diags)
	})

	t.Run("failure is a warning", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		t.Cleanup(server.Close)
		client, err := management.NewClientWithResponses(server.URL)
		require.NoError(t, err)

		var diags diag.Diagnostics
		result, applied := applyAutoScaleAfterCreate(t.Context(), client, clusterID, created, plan, &diags)
		require.False(t, applied)
		require.False(t, diags.HasError())
		require.Len(t, diags.Warnings(), 1)
		// The helper returns the API cluster. Create keeps the planned autoscale.
		require.Nil(t, result.AutoScale)
	})
}

// TestUpdateWaitConditions holds only once the API reports the planned values.
func TestUpdateWaitConditions(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	cluster := management.Cluster{ClusterID: &clusterID}

	// Omitted size config, autoscale, and auto suspend read as the schema defaults.
	require.NoError(t, waitConditionSizeConfig(types.Float32Value(1), types.Float32Value(1))(cluster))
	require.Error(t, waitConditionSizeConfig(types.Float32Value(2), types.Float32Value(1))(cluster))
	require.Error(t, waitConditionSizeConfig(types.Float32Value(1), types.Float32Value(4))(cluster))

	scaleOff := autoScaleResourceModel{MaxScaleFactor: types.Float32Value(1), Sensitivity: types.StringValue(string(management.NORMAL))}
	scaleUp := autoScaleResourceModel{MaxScaleFactor: types.Float32Value(2), Sensitivity: types.StringValue(string(management.HIGH))}
	require.NoError(t, waitConditionAutoScale(scaleOff)(cluster))
	require.Error(t, waitConditionAutoScale(scaleUp)(cluster))
	wrongSensitivity := cluster
	wrongSensitivity.AutoScale = &management.AutoScale{MaxScaleFactor: util.Ptr[float32](2), Sensitivity: util.Ptr(management.LOW)}
	require.Error(t, waitConditionAutoScale(scaleUp)(wrongSensitivity))
	// Factor 1 does not send sensitivity, so a leftover value is not waited on.
	leftover := cluster
	leftover.AutoScale = &management.AutoScale{MaxScaleFactor: util.Ptr[float32](1), Sensitivity: util.Ptr(management.HIGH)}
	require.NoError(t, waitConditionAutoScale(scaleOff)(leftover))

	idle := autoSuspendResourceModel{SuspendType: types.StringValue(string(management.AutoSuspendSuspendTypeIDLE)), SuspendAfterSeconds: types.Int64Value(600)}
	require.Error(t, waitConditionAutoSuspend(idle)(cluster))
	landed := cluster
	landed.AutoSuspend = &management.AutoSuspend{SuspendType: util.Ptr(management.AutoSuspendSuspendTypeIDLE), IdleAfterSeconds: util.Ptr(600)}
	require.NoError(t, waitConditionAutoSuspend(idle)(landed))

	nonProduction := types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))
	require.NoError(t, waitConditionDeploymentType(nonProduction)(cluster), "an omitted type keeps the plan")
	production := cluster
	production.DeploymentType = util.Ptr(management.ClusterDeploymentTypePRODUCTION)
	require.Error(t, waitConditionDeploymentType(nonProduction)(production))
}
