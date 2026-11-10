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

func TestEquivalentExpiresAtDoesNotCallAPI(t *testing.T) {
	state := readyClusterPlan()
	state.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123Z")
	plan := state
	plan.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123000Z")

	calls := 0
	client := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, _ *http.Request) bool {
		calls++
		w.WriteHeader(http.StatusInternalServerError)

		return true
	})

	got, err := applyClusterConfigOrToggleSuspension(t.Context(), client, state, plan)
	require.Nil(t, err)
	require.Equal(t, plan.ExpiresAt.ValueString(), got.ExpiresAt.ValueString())
	require.Zero(t, calls)
}

// TestApplyClusterConfigurationIgnoresEquivalentExpiresAt does not send expiresAt when the plan is the same instant.
func TestApplyClusterConfigurationIgnoresEquivalentExpiresAt(t *testing.T) {
	state := readyClusterPlan()
	state.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123Z")
	plan := state
	plan.DeploymentType = types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))
	plan.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123000Z")

	var patched management.Cluster
	cluster := lifecycleCluster(t)
	cluster.DeploymentType = utilPtr(management.ClusterDeploymentTypeNONPRODUCTION)
	cluster.ExpiresAt = utilPtr("2222-01-01T00:00:00.123Z")
	client := clusterLifecycleClient(t, cluster, management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != http.MethodPatch {
			return false
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&patched))
		writeClusterAction(t, w, cluster.ClusterID)

		return true
	})

	got, err := applyClusterConfiguration(t.Context(), client, lifecycleClusterUUID, lifecycleCluster(t), state, plan, "")
	require.Nil(t, err)
	require.Equal(t, plan.ExpiresAt.ValueString(), got.ExpiresAt.ValueString())
	require.Nil(t, patched.ExpiresAt)
	require.Equal(t, management.ClusterDeploymentTypeNONPRODUCTION, *patched.DeploymentType)
}

// TestApplyClusterConfiguration patches the cluster settings that differ from state.
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

	got, err := applyClusterConfiguration(t.Context(), client, lifecycleClusterUUID, lifecycleCluster(t), state, plan, "")
	require.Nil(t, err)
	require.Equal(t, plan.DeploymentType.ValueString(), got.DeploymentType.ValueString())
	require.Equal(t, plan.ExpiresAt.ValueString(), got.ExpiresAt.ValueString())
	require.Equal(t, management.ClusterDeploymentTypeNONPRODUCTION, *patched.DeploymentType)
	require.Equal(t, plan.ExpiresAt.ValueString(), *patched.ExpiresAt)
	require.Equal(t, []string{"10.0.0.0/8"}, *patched.FirewallRanges)
	require.Equal(t, cluster.ProjectID, patched.ProjectID, "PATCH reuses the project ID of the settled cluster")
}

// TestConfigUpdateRejectsClusterThatSettledSuspended stops a configuration update when the cluster settles suspended.
func TestConfigUpdateRejectsClusterThatSettledSuspended(t *testing.T) {
	state := readyClusterPlan()
	plan := state
	plan.DeploymentType = types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))

	patches := 0
	client := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateSUSPENDED, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPatch {
			patches++
			w.WriteHeader(http.StatusOK)

			return true
		}

		return false
	})

	// The update timeout is hours long. A context deadline keeps this test short
	// if the suspended cluster is patched and then polled for ACTIVE.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	_, err := applyClusterConfigOrToggleSuspension(ctx, client, state, plan)
	require.NotNil(t, err)
	require.Equal(t, "Cannot update the configuration for a suspended cluster.", err.Summary)
	require.Contains(t, err.Detail, "SUSPENDED")
	require.Zero(t, patches)
}

func TestApplyClusterConfigOrToggleSuspensionInvalidID(t *testing.T) {
	state := readyClusterPlan()
	state.ID = types.StringValue("not-a-uuid")
	plan := state
	plan.DeploymentType = types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))

	_, err := applyClusterConfigOrToggleSuspension(t.Context(), nil, state, plan)
	require.NotNil(t, err)
	require.Equal(t, "Invalid cluster ID", err.Summary)
}

// TestApplyClusterConfigurationSizeChangeStopsWhenContextEnds stops a size change when the context is canceled.
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
	_, err := applyClusterConfiguration(ctx, client, lifecycleClusterUUID, lifecycleCluster(t), state, plan, "")
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "canceled")
}

// TestApplyClusterConfigurationErrors surfaces Management API errors from a configuration update.
func TestApplyClusterConfigurationErrors(t *testing.T) {
	state := readyClusterPlan()
	plan := state
	plan.DeploymentType = types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))

	patchFailed := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusBadRequest)
			_, writeErr := io.WriteString(w, `{"message":"nope"}`)
			require.NoError(t, writeErr)

			return true
		}

		return false
	})
	_, err := applyClusterConfiguration(t.Context(), patchFailed, lifecycleClusterUUID, lifecycleCluster(t), state, plan, "")
	require.NotNil(t, err)

	badWindow, diags := types.ObjectValue(
		map[string]attr.Type{"hour": types.StringType, "day": types.Int64Type},
		map[string]attr.Value{"hour": types.StringValue("6"), "day": types.Int64Value(2)},
	)
	require.False(t, diags.HasError())
	windowPlan := plan
	windowPlan.UpdateWindow = badWindow
	_, err = applyClusterConfiguration(t.Context(), patchFailed, lifecycleClusterUUID, lifecycleCluster(t), state, windowPlan, "")
	require.NotNil(t, err)
}

// TestSuspendAndResumeCluster suspends and resumes a cluster through the Management API.
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
	_, err = suspend(t.Context(), failed, lifecycleClusterUUID, suspended, lifecycleCluster(t), "")
	require.NotNil(t, err)
	suspendedCluster := lifecycleCluster(t)
	suspendedCluster.State = utilPtr(management.ClusterStateSUSPENDED)
	_, err = resume(t.Context(), failed, lifecycleClusterUUID, active, suspendedCluster, "")
	require.NotNil(t, err)
}

// TestUpdateKeepsStoredAdminPasswordWhenPlanIsUnknown keeps the stored password when the plan value is unknown.
func TestUpdateKeepsStoredAdminPasswordWhenPlanIsUnknown(t *testing.T) {
	state := readyClusterPlan()
	state.AdminPassword = types.StringValue("stored-secret")
	plan := state
	plan.AdminPassword = types.StringUnknown()
	plan.DeploymentType = types.StringValue(string(management.ClusterDeploymentTypeNONPRODUCTION))

	cluster := lifecycleCluster(t)
	cluster.DeploymentType = utilPtr(management.ClusterDeploymentTypeNONPRODUCTION)
	client := clusterLifecycleClient(t, cluster, management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method != http.MethodPatch {
			return false
		}
		writeClusterAction(t, w, cluster.ClusterID)

		return true
	})

	got, err := applyClusterConfigOrToggleSuspension(t.Context(), client, state, plan)
	require.Nil(t, err)
	require.Equal(t, "stored-secret", got.AdminPassword.ValueString())

	suspended := state
	suspended.AdminPassword = types.StringUnknown()
	suspended.Suspended = types.BoolValue(true)
	suspendClient := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateSUSPENDED, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/suspend") {
			writeClusterAction(t, w, lifecycleCluster(t).ClusterID)

			return true
		}

		return false
	})
	got, err = applyClusterConfigOrToggleSuspension(t.Context(), suspendClient, state, suspended)
	require.Nil(t, err)
	require.Equal(t, "stored-secret", got.AdminPassword.ValueString())

	resumed := suspended
	resumed.Suspended = types.BoolValue(false)
	resumeClient := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/resume") {
			writeClusterAction(t, w, lifecycleCluster(t).ClusterID)

			return true
		}

		return false
	})
	got, err = applyClusterConfigOrToggleSuspension(t.Context(), resumeClient, suspendedState(state), resumed)
	require.Nil(t, err)
	require.Equal(t, "stored-secret", got.AdminPassword.ValueString())
}

func suspendedState(state clusterResourceModel) clusterResourceModel {
	state.Suspended = types.BoolValue(true)

	return state
}

func TestSuspendAfterEarlierSuspendFinished(t *testing.T) {
	active := readyClusterPlan()
	suspended := active
	suspended.Suspended = types.BoolValue(true)

	// The earlier suspend timed out but has finished since. No second POST is sent.
	client := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateSUSPENDED, nil)
	got, err := applyClusterConfigOrToggleSuspension(t.Context(), client, active, suspended)
	require.Nil(t, err)
	require.True(t, got.Suspended.ValueBool())
}

// TestApplyWaitsForPendingCluster waits for a pending cluster to settle before applying a change.
func TestApplyWaitsForPendingCluster(t *testing.T) {
	active := readyClusterPlan()
	suspended := active
	suspended.Suspended = types.BoolValue(true)

	gets := 0
	suspendCalled := false
	client := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateSUSPENDED, func(w http.ResponseWriter, r *http.Request) bool {
		switch {
		case r.Method == http.MethodGet && !suspendCalled:
			gets++
			cluster := lifecycleCluster(t)
			if gets == 1 {
				cluster.State = utilPtr(management.ClusterStatePENDING)
			}
			body, err := json.Marshal(cluster)
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)

			return true
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/suspend"):
			require.Greater(t, gets, 1, "suspend must wait until the cluster leaves PENDING")
			suspendCalled = true
			writeClusterAction(t, w, lifecycleCluster(t).ClusterID)

			return true
		}

		return false
	})
	got, err := applyClusterConfigOrToggleSuspension(t.Context(), client, active, suspended)
	require.Nil(t, err)
	require.True(t, suspendCalled)
	require.True(t, got.Suspended.ValueBool())
}

func readyClusterPlan() clusterResourceModel {
	plan := validClusterPlan()
	plan.ID = types.StringValue(lifecycleClusterID)

	return plan
}

const lifecycleClusterID = "3ca3d359-021d-45ed-86cb-38b8d14ac507"

var lifecycleClusterUUID = uuid.MustParse(lifecycleClusterID)

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

// clusterLifecycleClient serves cluster reads and lifecycle actions for update tests.
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

// TestUpdateWaitsForSettingsToLand keeps polling until GET reports the patched
// autoscale and auto suspend, instead of storing the previous values.
func TestUpdateWaitsForSettingsToLand(t *testing.T) {
	state := readyClusterPlan()
	plan := state
	autoScale := *state.AutoScale
	autoScale.MaxScaleFactor = types.Float32Value(scaleX2)
	autoScale.Sensitivity = types.StringValue(string(management.HIGH))
	plan.AutoScale = &autoScale
	autoSuspend := *state.AutoSuspend
	autoSuspend.SuspendType = types.StringValue(string(management.AutoSuspendSuspendTypeIDLE))
	autoSuspend.SuspendAfterSeconds = types.Int64Value(600)
	plan.AutoSuspend = &autoSuspend

	stale := lifecycleCluster(t)
	landed := lifecycleCluster(t)
	landed.AutoScale = &management.AutoScale{MaxScaleFactor: utilPtr[float32](scaleX2), Sensitivity: utilPtr(management.HIGH)}
	landed.AutoSuspend = &management.AutoSuspend{SuspendType: utilPtr(management.AutoSuspendSuspendTypeIDLE), IdleAfterSeconds: utilPtr(600)}

	// The first GETs after PATCH still report the old settings.
	const staleGets = 6
	gets := 0
	patched := false
	client := clusterLifecycleClient(t, stale, management.ClusterStateACTIVE, func(w http.ResponseWriter, r *http.Request) bool {
		switch {
		case r.Method == http.MethodPatch:
			patched = true
			writeClusterAction(t, w, stale.ClusterID)

			return true
		case r.Method == http.MethodGet && patched:
			gets++
			cluster := stale
			if gets > staleGets {
				cluster = landed
			}
			body, err := json.Marshal(cluster)
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)

			return true
		}

		return false
	})

	got, err := applyClusterConfigOrToggleSuspension(t.Context(), client, state, plan)
	require.Nil(t, err)
	require.Greater(t, gets, staleGets)
	require.Equal(t, float32(scaleX2), got.AutoScale.MaxScaleFactor.ValueFloat32())
	require.Equal(t, string(management.HIGH), got.AutoScale.Sensitivity.ValueString())
	require.Equal(t, string(management.AutoSuspendSuspendTypeIDLE), got.AutoSuspend.SuspendType.ValueString())
	require.Equal(t, int64(600), got.AutoSuspend.SuspendAfterSeconds.ValueInt64())
}

// TestReorderedFirewallRangesDoNotCallAPI stores the planned order without a PATCH.
func TestReorderedFirewallRangesDoNotCallAPI(t *testing.T) {
	state := readyClusterPlan()
	state.FirewallRanges = []types.String{types.StringValue("10.0.0.0/8"), types.StringValue("192.168.0.0/16")}
	plan := state
	plan.FirewallRanges = []types.String{types.StringValue("192.168.0.0/16"), types.StringValue("10.0.0.0/8")}

	got, err := applyClusterConfigOrToggleSuspension(t.Context(), nil, state, plan)
	require.Nil(t, err)
	require.Equal(t, plan.FirewallRanges, got.FirewallRanges)
}

// TestWaitSettledHintsOnlyForPending adds the PENDING hint when the wait runs
// out of time on a PENDING cluster; a failed cluster does not get it.
func TestWaitSettledHintsOnlyForPending(t *testing.T) {
	failed := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStateFAILED, nil)
	_, err := waitSettled(t.Context(), failed, lifecycleClusterUUID)
	require.NotNil(t, err)
	require.NotContains(t, err.Detail, "offline or impacted")

	pending := clusterLifecycleClient(t, lifecycleCluster(t), management.ClusterStatePENDING, nil)
	_, err = wait(t.Context(), pending, lifecycleClusterUUID, 300*time.Millisecond, waitConditionSettled())
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "offline or impacted")
}
