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
	// A deleted cluster fails the wait after a few polls instead of at the timeout.
	_, serr = wait(t.Context(), notFoundClient, clusterID, time.Hour, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "deleted outside Terraform")
	require.Equal(t, clusterNotFoundThreshold, notFoundGets)

	down, err := management.NewClientWithResponses("http://127.0.0.1:1")
	require.NoError(t, err)
	_, serr = wait(t.Context(), down, clusterID, time.Second, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "failed to get cluster")
}

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

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
	require.NotNil(t, waitTerminated(t.Context(), stuckClient, clusterID, time.Millisecond))
}

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
		result := applyAutoScaleAfterCreate(t.Context(), client, clusterID, created, plan, &diags)
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
		require.Equal(t, created, applyAutoScaleAfterCreate(t.Context(), client, clusterID, created, notDeferred, &diags))
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
		result := applyAutoScaleAfterCreate(t.Context(), client, clusterID, created, plan, &diags)
		require.False(t, diags.HasError())
		require.Len(t, diags.Warnings(), 1)
		// State keeps the autoscale the API reports, so the next plan sets 4 again.
		require.Nil(t, result.AutoScale)
	})
}
