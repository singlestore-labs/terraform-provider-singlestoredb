package clusters

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// applyClusterConfigOrToggleSuspension suspends, resumes, or patches the cluster.
// It waits for a pending cluster to settle before sending the change.
func applyClusterConfigOrToggleSuspension(ctx context.Context, c management.ClientWithResponsesInterface, state, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) {
	// ModifyPlan already rejects changing suspended together with other configuration.
	// Prefer suspend/resume routing when suspended toggles so computed-only plan noise
	// (e.g. unknown update_window) cannot divert the call to PATCH.
	suspendedChanged := !plan.Suspended.Equal(state.Suspended)
	if !suspendedChanged && !hasGeneralConfigChanged(state, plan) {
		// Adopt the configured spelling so an equivalent timestamp does not stay a diff.
		state.ExpiresAt = expiresAtToStore(state.ExpiresAt, plan.ExpiresAt)
		// A reordered allowlist is the same set, so nothing is sent, but the stored
		// list has to match the planned order.
		state.FirewallRanges = plan.FirewallRanges

		return state, nil
	}

	// Refresh keeps PENDING clusters in state. Wait until a suspend, resume, or
	// scale already in progress finishes before sending another change.
	id, perr := parseClusterID(plan.ID.ValueString())
	if perr != nil {
		return clusterResourceModel{}, perr
	}

	current, werr := waitSettled(ctx, c, id)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	// admin_password is computed-only. Keep the password already in state when the
	// plan value is still unknown, including before UseStateForUnknown runs.
	adminPassword := storedAdminPassword(state.AdminPassword, plan.AdminPassword)
	if suspendedChanged {
		if plan.Suspended.ValueBool() {
			return suspend(ctx, c, id, plan, current, adminPassword)
		}

		return resume(ctx, c, id, plan, current, adminPassword)
	}

	// Refresh keeps a PENDING cluster's previous suspended value, so the plan can
	// still say active after the cluster finishes suspending. Reject that instead
	// of patching and polling for ACTIVE until the update timeout.
	if util.Deref(current.State) == management.ClusterStateSUSPENDED {
		return clusterResourceModel{}, suspendedConfigUpdateError()
	}

	return applyClusterConfiguration(ctx, c, id, current, state, plan, adminPassword)
}

// waitSettled polls until the cluster is ACTIVE or SUSPENDED.
func waitSettled(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID) (management.Cluster, *util.SummaryWithDetailError) {
	var lastState management.ClusterState
	recordState := func(c management.Cluster) error {
		lastState = util.Deref(c.State)

		return nil
	}

	cluster, err, timedOut := waitReportingTimeout(ctx, c, id, config.ClusterUpdateTimeout, recordState, waitConditionSettled())
	if err != nil && timedOut && lastState == management.ClusterStatePENDING {
		// TODO: fail fast when GET /v2/clusters reports offline and impacted clusters
		// separately from PENDING.
		err.Detail += ". Management API also reports an offline or impacted cluster as PENDING, " +
			"so a cluster that stays PENDING may need attention; check its status in the SingleStore Portal"
	}

	return cluster, err
}

// hasGeneralConfigChanged reports whether any cluster setting other than suspended differs.
func hasGeneralConfigChanged(state, plan clusterResourceModel) bool {
	return sizeConfigChanged(state, plan) ||
		!sameFirewallRanges(state.FirewallRanges, plan.FirewallRanges) ||
		!util.SameTimestamp(plan.ExpiresAt, state.ExpiresAt) ||
		!plan.DeploymentType.Equal(state.DeploymentType) ||
		autoScaleChanged(state, plan) ||
		autoSuspendChanged(state, plan) ||
		updateWindowChanged(state.UpdateWindow, plan.UpdateWindow)
}

// sizeConfigChanged reports whether size, scale factor, or cache multiplier differs.
func sizeConfigChanged(state, plan clusterResourceModel) bool {
	return !plan.Size.Equal(state.Size) ||
		!plan.ScaleFactor.Equal(state.ScaleFactor) ||
		!plan.CacheConfig.Equal(state.CacheConfig)
}

// autoScaleChanged reports whether the autoscale factor or sensitivity differs.
func autoScaleChanged(state, plan clusterResourceModel) bool {
	return !plan.AutoScale.MaxScaleFactor.Equal(state.AutoScale.MaxScaleFactor) ||
		!plan.AutoScale.Sensitivity.Equal(state.AutoScale.Sensitivity)
}

// autoSuspendChanged reports whether the auto suspend type or delay differs.
func autoSuspendChanged(state, plan clusterResourceModel) bool {
	return !plan.AutoSuspend.SuspendType.Equal(state.AutoSuspend.SuspendType) ||
		!plan.AutoSuspend.SuspendAfterSeconds.Equal(state.AutoSuspend.SuspendAfterSeconds)
}

// updateWindowChanged reports whether the maintenance window in the plan differs from state.
func updateWindowChanged(state, plan types.Object) bool {
	if plan.IsUnknown() || state.IsUnknown() {
		return false
	}

	return !plan.Equal(state)
}

// sameFirewallRanges reports whether two allowlists hold the same ranges in any
// order. The Management API does not keep the order, so a reordered list is not a
// change to send. A list with an unknown range is treated as changed.
func sameFirewallRanges(a, b []types.String) bool {
	for _, r := range append(append([]types.String(nil), a...), b...) {
		if r.IsUnknown() {
			return false
		}
	}

	setA := stringSet(util.StringFirewallRanges(a))
	setB := stringSet(util.StringFirewallRanges(b))
	if len(setA) != len(setB) {
		return false
	}
	for r := range setA {
		if _, ok := setB[r]; !ok {
			return false
		}
	}

	return true
}

// applyClusterConfiguration patches the cluster. current is the settled cluster
// read just before; project_name cannot change, so its project ID is reused
// instead of resolving the name again. The cluster must be ACTIVE: the wait
// below requires ACTIVE, and a suspended cluster is rejected by the caller.
func applyClusterConfiguration(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, current management.Cluster, state, plan clusterResourceModel, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	patchBody, perr := clusterPatchBody(ctx, current, state, plan)
	if perr != nil {
		return clusterResourceModel{}, perr
	}

	clusterUpdateResponse, uerr := c.PatchV2ClustersClusterIDWithResponse(ctx, id, patchBody)
	if serr := util.StatusOK(clusterUpdateResponse, uerr); serr != nil {
		return clusterResourceModel{}, serr
	}

	cluster, werr := wait(ctx, c, id, config.ClusterUpdateTimeout, updateWaitConditions(state, plan)...)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return clusterModelFromAPI(cluster, adminPassword, plan)
}

// clusterPatchBody builds PATCH /v2/clusters with only the settings that changed.
func clusterPatchBody(ctx context.Context, current management.Cluster, state, plan clusterResourceModel) (management.Cluster, *util.SummaryWithDetailError) {
	patchBody := management.Cluster{
		Name:      plan.Name.ValueString(),
		ProjectID: current.ProjectID,
	}

	if sizeConfigChanged(state, plan) {
		patchBody.SizeConfig = &management.SizeConfig{
			Size:        util.MaybeString(plan.Size),
			ScaleFactor: util.MaybeFloat32(plan.ScaleFactor),
			CacheConfig: util.MaybeFloat32(plan.CacheConfig),
		}
	}

	if !sameFirewallRanges(state.FirewallRanges, plan.FirewallRanges) {
		firewallRanges := util.StringFirewallRanges(plan.FirewallRanges)
		patchBody.FirewallRanges = &firewallRanges
	}

	// TODO: reached once validateModifyExpiresAt allows changes, when PATCH /v2/clusters applies expiresAt.
	// Equivalent RFC3339 spellings are the same expiration and are not sent.
	if !util.SameTimestamp(plan.ExpiresAt, state.ExpiresAt) {
		patchBody.ExpiresAt = util.MaybeString(plan.ExpiresAt)
	}

	if !plan.DeploymentType.Equal(state.DeploymentType) {
		patchBody.DeploymentType = util.ClusterDeploymentTypeString(plan.DeploymentType)
	}

	if autoScaleChanged(state, plan) {
		patchBody.AutoScale = toAutoScale(plan)
	}

	if autoSuspendChanged(state, plan) {
		patchBody.AutoSuspend = toAutoSuspend(plan)
	}

	// TODO: reached once update_window is Optional, when POST and PATCH /v2/clusters apply updateWindow.
	if updateWindowChanged(state.UpdateWindow, plan.UpdateWindow) {
		updateWindow, updateWindowDiags := toManagementUpdateWindow(ctx, plan.UpdateWindow)
		if err := errorFromDiagnostics(updateWindowDiags); err != nil {
			return management.Cluster{}, err
		}
		patchBody.UpdateWindow = updateWindow
	}

	return patchBody, nil
}

// updateWaitConditions waits until the Management API reports every changed
// setting. Update stores these settings from the API response, so returning
// before they land would store the previous values and fail the apply.
func updateWaitConditions(state, plan clusterResourceModel) []waitCondition {
	conditions := []waitCondition{
		waitConditionState(management.ClusterStateACTIVE),
		waitConditionSize(plan.Size.ValueString()),
	}
	if sizeConfigChanged(state, plan) {
		conditions = append(conditions,
			waitConditionSizeConfig(plan.ScaleFactor, plan.CacheConfig),
			waitConditionTakesAtLeast(config.WorkspaceScaleTakesAtLeast),
		)
	}
	if !sameFirewallRanges(state.FirewallRanges, plan.FirewallRanges) {
		conditions = append(conditions, waitConditionFirewallRanges(plan.FirewallRanges))
	}
	if !plan.DeploymentType.Equal(state.DeploymentType) && util.IsConfiguredString(plan.DeploymentType) {
		conditions = append(conditions, waitConditionDeploymentType(plan.DeploymentType))
	}
	if autoScaleChanged(state, plan) {
		conditions = append(conditions, waitConditionAutoScale(*plan.AutoScale))
	}
	if autoSuspendChanged(state, plan) {
		conditions = append(conditions, waitConditionAutoSuspend(*plan.AutoSuspend))
	}

	return conditions
}

// resume starts a suspended cluster and waits until it is active.
func resume(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, plan clusterResourceModel, current management.Cluster, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	if util.Deref(current.State) == management.ClusterStateACTIVE {
		// A resume that timed out earlier has finished since.
		return clusterModelFromAPI(current, adminPassword, plan)
	}

	clusterResumeResponse, err := c.PostV2ClustersClusterIDResumeWithResponse(ctx, id, management.ClusterResume{})
	if serr := util.StatusOK(clusterResumeResponse, err); serr != nil {
		return clusterResourceModel{}, serr
	}

	cluster, werr := wait(ctx, c, id, config.ClusterUpdateTimeout,
		waitConditionState(management.ClusterStateACTIVE),
	)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return clusterModelFromAPI(cluster, adminPassword, plan)
}

// suspend stops an active cluster and waits until it is suspended.
func suspend(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, plan clusterResourceModel, current management.Cluster, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	if util.Deref(current.State) == management.ClusterStateSUSPENDED {
		// A suspend that timed out earlier has finished since.
		return clusterModelFromAPI(current, adminPassword, plan)
	}

	clusterSuspendResponse, err := c.PostV2ClustersClusterIDSuspendWithResponse(ctx, id)
	if serr := util.StatusOK(clusterSuspendResponse, err); serr != nil {
		return clusterResourceModel{}, serr
	}

	cluster, werr := wait(ctx, c, id, config.ClusterUpdateTimeout,
		waitConditionState(management.ClusterStateSUSPENDED),
	)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return clusterModelFromAPI(cluster, adminPassword, plan)
}

func suspendedConfigUpdateError() *util.SummaryWithDetailError {
	return &util.SummaryWithDetailError{
		Summary: "Cannot update the configuration for a suspended cluster.",
		Detail:  "The cluster settled to SUSPENDED. Resume the cluster by setting suspended to false before updating the configuration.",
	}
}

// storedAdminPassword is the password to keep when Management API omits adminPassword.
// A computed-only plan leaves the attribute unknown, and ValueString on that value is empty.
func storedAdminPassword(state, plan types.String) string {
	if knownPassword, ok := knownAdminPassword(plan); ok {
		return knownPassword
	}
	if knownPassword, ok := knownAdminPassword(state); ok {
		return knownPassword
	}

	return ""
}

// expiresAtToStore keeps the configured spelling when it is the same instant
// already stored. Import normalizes the API timestamp, so the config may use
// another RFC3339 spelling of that time.
func expiresAtToStore(state, plan types.String) types.String {
	if util.SameTimestamp(state, plan) {
		return plan
	}

	return state
}

// knownAdminPassword returns a password that was stored, ignoring null and unknown values.
func knownAdminPassword(value types.String) (string, bool) {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return "", false
	}

	return value.ValueString(), true
}

// clusterModelFromAPI converts a Management API cluster into state, or returns the conversion error.
func clusterModelFromAPI(cluster management.Cluster, adminPassword string, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) {
	model, diags := toClusterResourceModel(cluster, adminPassword, plan, plan.ProjectName)
	if err := errorFromDiagnostics(diags); err != nil {
		return clusterResourceModel{}, err
	}

	return model, nil
}
