package clusters

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	// An unknown suspended value is not a toggle yet.
	suspendedChanged := changedKnown(plan.Suspended, state.Suspended)
	if !suspendedChanged && !hasGeneralConfigChanged(state, plan) {
		// Adopt the configured spelling so an equivalent timestamp does not stay a diff.
		state.ExpiresAt = expiresAtToStore(state.ExpiresAt, plan.ExpiresAt)
		// A reordered allowlist is the same set, so nothing is sent, but the stored
		// list has to match the planned order. An unknown range cannot be stored.
		if firewallRangesKnown(plan.FirewallRanges) {
			state.FirewallRanges = plan.FirewallRanges
		}

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
	return wait(ctx, c, id, config.ClusterUpdateTimeout, waitConditionSettled())
}

// hasGeneralConfigChanged reports whether any cluster setting other than suspended differs.
// A value that is not known yet is not a difference. ModifyPlan checks it again during
// apply, once every value is known.
func hasGeneralConfigChanged(state, plan clusterResourceModel) bool {
	return sizeConfigChanged(state, plan) ||
		firewallRangesChanged(state.FirewallRanges, plan.FirewallRanges) ||
		expiresAtChanged(plan.ExpiresAt, state.ExpiresAt) ||
		changedKnown(plan.DeploymentType, state.DeploymentType) ||
		autoScaleChanged(state, plan) ||
		autoSuspendChanged(state, plan) ||
		updateWindowChanged(state.UpdateWindow, plan.UpdateWindow)
}

// generalConfigMayChange reports whether a setting other than suspended changes,
// or is not known yet and may change once Terraform plans again during apply.
func generalConfigMayChange(state, plan clusterResourceModel) bool {
	if hasGeneralConfigChanged(state, plan) || !firewallRangesKnown(plan.FirewallRanges) {
		return true
	}

	values := []attr.Value{plan.Size, plan.ScaleFactor, plan.CacheConfig, plan.ExpiresAt, plan.DeploymentType, plan.UpdateWindow}
	if plan.AutoScale != nil {
		values = append(values, plan.AutoScale.MaxScaleFactor, plan.AutoScale.Sensitivity)
	}
	if plan.AutoSuspend != nil {
		values = append(values, plan.AutoSuspend.SuspendType, plan.AutoSuspend.SuspendAfterSeconds)
	}
	for _, v := range values {
		if v.IsUnknown() {
			return true
		}
	}

	return false
}

// sizeConfigChanged reports whether a known size, scale factor, or cache multiplier differs.
func sizeConfigChanged(state, plan clusterResourceModel) bool {
	return changedKnown(plan.Size, state.Size) ||
		changedKnown(plan.ScaleFactor, state.ScaleFactor) ||
		changedKnown(plan.CacheConfig, state.CacheConfig)
}

// firewallRangesKnown reports whether every range is known.
func firewallRangesKnown(ranges []types.String) bool {
	for _, firewallRange := range ranges {
		if firewallRange.IsUnknown() {
			return false
		}
	}

	return true
}

// firewallRangesChanged reports whether the allowlists differ. A list that still
// contains an unknown range is not a confirmed change.
func firewallRangesChanged(state, plan []types.String) bool {
	if !firewallRangesKnown(state) || !firewallRangesKnown(plan) {
		return false
	}

	return !util.SameFirewallRanges(state, plan)
}

// expiresAtChanged reports whether the expiration differs. An unknown timestamp
// is not a confirmed change.
func expiresAtChanged(plan, state types.String) bool {
	if plan.IsUnknown() || state.IsUnknown() {
		return false
	}

	return !util.SameTimestamp(plan, state)
}

// autoScaleChanged reports whether the known autoscale factor or sensitivity differs.
func autoScaleChanged(state, plan clusterResourceModel) bool {
	return changedKnown(plan.AutoScale.MaxScaleFactor, state.AutoScale.MaxScaleFactor) ||
		changedKnown(plan.AutoScale.Sensitivity, state.AutoScale.Sensitivity)
}

// autoSuspendChanged reports whether the known auto suspend type or delay differs.
func autoSuspendChanged(state, plan clusterResourceModel) bool {
	return changedKnown(plan.AutoSuspend.SuspendType, state.AutoSuspend.SuspendType) ||
		changedKnown(plan.AutoSuspend.SuspendAfterSeconds, state.AutoSuspend.SuspendAfterSeconds)
}

// updateWindowChanged reports whether the maintenance window in the plan differs from state.
func updateWindowChanged(state, plan types.Object) bool {
	if plan.IsUnknown() || state.IsUnknown() {
		return false
	}

	return !plan.Equal(state)
}

// applyClusterConfiguration patches the cluster. current is the settled cluster
// read just before; project_name cannot change, so its project ID is reused
// instead of resolving the name again. A cluster that is already suspended is
// rejected by the caller. When suspended is omitted, the wait also accepts a
// later policy suspension once the requested settings land.
func applyClusterConfiguration(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, current management.Cluster, state, plan clusterResourceModel, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	clusterUpdateResponse, uerr := c.PatchV2ClustersClusterIDWithResponse(ctx, id, clusterPatchBody(current, state, plan))
	if serr := util.StatusOK(clusterUpdateResponse, uerr); serr != nil {
		return clusterResourceModel{}, serr
	}

	cluster, werr := wait(ctx, c, id, config.ClusterUpdateTimeout, updateWaitConditions(state, plan)...)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return toClusterResourceModel(cluster, adminPassword, plan, plan.ProjectName), nil
}

// clusterPatchBody builds PATCH /v2/clusters with only the settings that changed.
func clusterPatchBody(current management.Cluster, state, plan clusterResourceModel) management.Cluster {
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

	if firewallRangesChanged(state.FirewallRanges, plan.FirewallRanges) {
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
		patchBody.UpdateWindow = toManagementUpdateWindow(plan.UpdateWindow)
	}

	return patchBody
}

// updateWaitConditions waits until the Management API reports every changed
// setting. Update stores these settings from the API response, so returning
// before they land would store the previous values and fail the apply.
// An explicit suspended = false still waits for ACTIVE. An omitted value does
// not: auto_suspend can suspend the cluster after the settings land, including
// during the minimum wait for a size change, and the stored status follows the API.
func updateWaitConditions(state, plan clusterResourceModel) []waitCondition {
	conditions := []waitCondition{
		updateStateCondition(plan.Suspended),
		waitConditionSize(plan.Size.ValueString()),
	}
	if sizeConfigChanged(state, plan) {
		conditions = append(conditions,
			waitConditionSizeConfig(plan.ScaleFactor, plan.CacheConfig),
			waitConditionTakesAtLeast(config.WorkspaceScaleTakesAtLeast),
		)
	}
	if firewallRangesChanged(state.FirewallRanges, plan.FirewallRanges) {
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

// updateStateCondition waits for ACTIVE when suspended is configured. Only false
// reaches a configuration update, because a suspended cluster is rejected. An
// omitted value is null or unknown, so either settled state ends the wait once the
// other conditions hold, and apply stores the state the Management API reports.
func updateStateCondition(suspended types.Bool) waitCondition {
	if !suspended.IsNull() && !suspended.IsUnknown() {
		return waitConditionState(management.ClusterStateACTIVE)
	}

	return waitConditionState(management.ClusterStateACTIVE, management.ClusterStateSUSPENDED)
}

// resume starts a suspended cluster and waits until it is active.
func resume(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, plan clusterResourceModel, current management.Cluster, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	if util.Deref(current.State) == management.ClusterStateACTIVE {
		// A resume that timed out earlier has finished since.
		return toClusterResourceModel(current, adminPassword, plan, plan.ProjectName), nil
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

	return toClusterResourceModel(cluster, adminPassword, plan, plan.ProjectName), nil
}

// suspend stops an active cluster and waits until it is suspended.
func suspend(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, plan clusterResourceModel, current management.Cluster, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	if util.Deref(current.State) == management.ClusterStateSUSPENDED {
		// A suspend that timed out earlier has finished since.
		return toClusterResourceModel(current, adminPassword, plan, plan.ProjectName), nil
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

	return toClusterResourceModel(cluster, adminPassword, plan, plan.ProjectName), nil
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
