package clusters

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/stretchr/testify/require"
)

func TestApplyClusterConfigOrToggleSuspensionNoop(t *testing.T) {
	state := readyClusterPlan()
	calls := 0
	client := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, _ *http.Request) bool {
		calls++
		w.WriteHeader(http.StatusInternalServerError)

		return true
	})

	got, err := applyClusterConfigOrToggleSuspension(t.Context(), client, state, state)
	require.Nil(t, err)
	require.Equal(t, state.DeploymentType, got.DeploymentType)
	require.Zero(t, calls)
}

func TestApplyClusterConfiguration(t *testing.T) {
	state := readyClusterPlan()
	plan := state
	plan.DeploymentType = types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))
	plan.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123Z")
	plan.FirewallRanges = []types.String{types.StringValue("10.0.0.0/8")}

	var patched management.Cluster
	cluster := lifecycleCluster(t)
	cluster.DeploymentType = utilPtr(management.ClusterDeploymentTypeNONPRODUCTION)
	cluster.ExpiresAt = utilPtr("2222-01-01T00:00:00.123Z")
	cluster.FirewallRanges = utilPtr([]string{"10.0.0.0/8"})
	client := clusterLifecycleClient(t, cluster, management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != http.MethodPatch {
			return false
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
		writeClusterAction(t, w, cluster.ClusterID)

		return true
	})

	got, err := applyClusterConfiguration(t.Context(), client, state, plan)
	require.Nil(t, err)
	require.Equal(t, plan.DeploymentType.ValueString(), got.DeploymentType.ValueString())
	require.Equal(t, plan.ExpiresAt.ValueString(), got.ExpiresAt.ValueString())
	require.Equal(t, management.ClusterDeploymentTypeNONPRODUCTION, *patched.DeploymentType)
	require.Equal(t, plan.ExpiresAt.ValueString(), *patched.ExpiresAt)
	require.Equal(t, []string{"10.0.0.0/8"}, *patched.FirewallRanges)
}

func TestApplyClusterConfigurationSizeChangeStopsWhenContextEnds(t *testing.T) {
	state := readyClusterPlan()
	plan := state
	plan.Size = types.StringValue("S-1")
	client := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != http.MethodPatch {
			return false
		}
		writeClusterAction(t, w, lifecycleCluster(t).ClusterID)

		return true
	})

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	_, err := applyClusterConfiguration(ctx, client, state, plan)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "not ready")
}

func TestApplyClusterConfigurationErrors(t *testing.T) {
	state := readyClusterPlan()
	plan := state
	plan.DeploymentType = types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))

	getFailed := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusInternalServerError)

			return true
		}

		return false
	})
	_, err := applyClusterConfiguration(t.Context(), getFailed, state, plan)
	require.NotNil(t, err)

	patchFailed := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusBadRequest)
			_, writeErr := io.WriteString(w, `{"message":"nope"}`)
			require.NoError(t, writeErr)

			return true
		}

		return false
	})
	_, err = applyClusterConfiguration(t.Context(), patchFailed, state, plan)
	require.NotNil(t, err)

	badWindow, diags := types.ObjectValue(
		map[string]attr.Type{"hour": types.StringType, "day": types.Int64Type},
		map[string]attr.Value{"hour": types.StringValue("6"), "day": types.Int64Value(2)},
	)
	require.False(t, diags.HasError())
	windowPlan := plan
	windowPlan.UpdateWindow = badWindow
	_, err = applyClusterConfiguration(t.Context(), patchFailed, state, windowPlan)
	require.NotNil(t, err)
}

func TestSuspendAndResumeCluster(t *testing.T) {
	active := readyClusterPlan()
	suspended := active
	suspended.Suspended = types.BoolValue(true)

	suspendClient := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateSUSPENDED, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/suspend") {
			writeClusterAction(t, w, lifecycleCluster(t).ClusterID)

			return true
		}

		return false
	})
	got, err := applyClusterConfigOrToggleSuspension(t.Context(), suspendClient, active, suspended)
	require.Nil(t, err)
	require.True(t, got.Suspended.ValueBool())

	resumeClient := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/resume") {
			writeClusterAction(t, w, lifecycleCluster(t).ClusterID)

			return true
		}

		return false
	})
	got, err = applyClusterConfigOrToggleSuspension(t.Context(), resumeClient, suspended, active)
	require.Nil(t, err)
	require.False(t, got.Suspended.ValueBool())

	failed := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusInternalServerError)

			return true
		}

		return false
	})
	_, err = suspend(t.Context(), failed, suspended)
	require.NotNil(t, err)
	_, err = resume(t.Context(), failed, active)
	require.NotNil(t, err)
}

func readyClusterPlan() clusterResourceModel {
	plan := validClusterPlan()
	plan.ID = types.StringValue(lifecycleClusterID)

	return plan
}

const lifecycleClusterID = "3ca3d359-021d-45ed-86cb-38b8d14ac507"

func lifecycleCluster(t *testing.T) management.Cluster {
	t.Helper()

	clusterID := uuid.MustParse(lifecycleClusterID)
	projectID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	size := "S-00"
	state := management.ClusterStateACTIVE

	return management.Cluster{
		Name:       "cluster",
		ClusterID:  &clusterID,
		ProjectID:  projectID,
		State:      &state,
		SizeConfig: &management.SizeConfig{Size: &size},
	}
}

func clusterLifecycleClient(
	t *testing.T,
	cluster management.Cluster,
	ready management.ClusterState,
	override func(http.ResponseWriter, *http.Request) bool,
) *management.ClientWithResponses {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if override != nil && override(w, r) {
			return
		}
		if r.Method == http.MethodGet {
			cluster.State = &ready
			body, err := json.Marshal(cluster)
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)

			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	return client
}

func writeClusterAction(t *testing.T, w http.ResponseWriter, id *uuid.UUID) {
	t.Helper()

	_, err := io.WriteString(w, `{"clusterID":"`+id.String()+`"}`)
	require.NoError(t, err)
}

func utilPtr[T any](v T) *T {
	return &v
}
