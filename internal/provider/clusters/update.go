package clusters

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

func applyClusterConfigOrToggleSuspension(ctx context.Context, c management.ClientWithResponsesInterface, state, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) {
	// ModifyPlan already rejects changing suspended together with other configuration.
	// Prefer suspend/resume routing when suspended toggles so computed-only plan noise
	// (e.g. unknown update_window) cannot divert the call to PATCH.
	suspendedChanged := !plan.Suspended.Equal(state.Suspended)
	if !suspendedChanged && !hasGeneralConfigChanged(state, plan) {
		// Adopt the configured spelling so an equivalent timestamp does not stay a diff.
		state.ExpiresAt = expiresAtToStore(state.ExpiresAt, plan.ExpiresAt)

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

	return applyClusterConfiguration(ctx, c, id, current, state, plan)
}

func waitSettled(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID) (management.Cluster, *util.SummaryWithDetailError) {
	return wait(ctx, c, id, config.ClusterUpdateTimeout, waitConditionSettled())
}

func hasGeneralConfigChanged(state, plan clusterResourceModel) bool { //nolint:cyclop
	return !plan.Size.Equal(state.Size) ||
		!plan.CacheConfig.Equal(state.CacheConfig) ||
		!plan.ScaleFactor.Equal(state.ScaleFactor) ||
		!equalFirewallRangesLists(state.FirewallRanges, plan.FirewallRanges) ||
		!util.SameTimestamp(plan.ExpiresAt, state.ExpiresAt) ||
		!plan.DeploymentType.Equal(state.DeploymentType) ||
		!plan.AutoScale.MaxScaleFactor.Equal(state.AutoScale.MaxScaleFactor) || !plan.AutoScale.Sensitivity.Equal(state.AutoScale.Sensitivity) ||
		!plan.AutoSuspend.SuspendType.Equal(state.AutoSuspend.SuspendType) || !plan.AutoSuspend.SuspendAfterSeconds.Equal(state.AutoSuspend.SuspendAfterSeconds) ||
		updateWindowChanged(state.UpdateWindow, plan.UpdateWindow)
}

func updateWindowChanged(state, plan types.Object) bool {
	if plan.IsUnknown() || state.IsUnknown() {
		return false
	}

	return !plan.Equal(state)
}

func equalFirewallRangesLists(a, b []types.String) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}

	return true
}

// applyClusterConfiguration patches the cluster. current is the settled cluster
// read just before; project_name cannot change, so its project ID is reused
// instead of resolving the name again. The cluster must be ACTIVE: the wait
// below requires ACTIVE, and a suspended cluster is rejected by the caller.
func applyClusterConfiguration(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, current management.Cluster, state, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) { //nolint:cyclop
	desiredSize := plan.Size.ValueString()

	firewallRanges := util.StringFirewallRanges(plan.FirewallRanges)
	patchBody := management.Cluster{
		Name:      plan.Name.ValueString(),
		ProjectID: current.ProjectID,
	}

	if !plan.Size.Equal(state.Size) || !plan.CacheConfig.Equal(state.CacheConfig) || !plan.ScaleFactor.Equal(state.ScaleFactor) {
		patchBody.SizeConfig = &management.SizeConfig{
			Size:        util.MaybeString(plan.Size),
			ScaleFactor: util.MaybeFloat32(plan.ScaleFactor),
			CacheConfig: util.MaybeFloat32(plan.CacheConfig),
		}
	}

	if !equalFirewallRangesLists(state.FirewallRanges, plan.FirewallRanges) {
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

	if !plan.AutoScale.MaxScaleFactor.Equal(state.AutoScale.MaxScaleFactor) ||
		!plan.AutoScale.Sensitivity.Equal(state.AutoScale.Sensitivity) {
		patchBody.AutoScale = toAutoScale(plan)
	}

	if !plan.AutoSuspend.SuspendType.Equal(state.AutoSuspend.SuspendType) ||
		!plan.AutoSuspend.SuspendAfterSeconds.Equal(state.AutoSuspend.SuspendAfterSeconds) {
		patchBody.AutoSuspend = toAutoSuspend(plan)
	}

	// TODO: reached once update_window is Optional, when POST and PATCH /v2/clusters apply updateWindow.
	if !plan.UpdateWindow.Equal(state.UpdateWindow) {
		updateWindow, updateWindowDiags := toManagementUpdateWindow(ctx, plan.UpdateWindow)
		if err := errorFromDiagnostics(updateWindowDiags); err != nil {
			return clusterResourceModel{}, err
		}
		patchBody.UpdateWindow = updateWindow
	}

	clusterUpdateResponse, uerr := c.PatchV2ClustersClusterIDWithResponse(ctx, id, patchBody)
	if serr := util.StatusOK(clusterUpdateResponse, uerr); serr != nil {
		return clusterResourceModel{}, serr
	}

	conditions := []waitCondition{
		waitConditionState(management.ClusterStateACTIVE),
		waitConditionSize(desiredSize),
	}
	if !plan.Size.Equal(state.Size) || !plan.ScaleFactor.Equal(state.ScaleFactor) || !plan.CacheConfig.Equal(state.CacheConfig) {
		conditions = append(conditions, waitConditionTakesAtLeast(config.WorkspaceScaleTakesAtLeast))
	}
	if !equalFirewallRangesLists(state.FirewallRanges, plan.FirewallRanges) {
		conditions = append(conditions, waitConditionFirewallRanges(plan.FirewallRanges))
	}

	cluster, werr := wait(ctx, c, id, config.ClusterUpdateTimeout, conditions...)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return clusterModelFromAPI(cluster, storedAdminPassword(state.AdminPassword, plan.AdminPassword), plan.FirewallRanges, plan.ProjectName, plan.ExpiresAt)
}

func resume(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, plan clusterResourceModel, current management.Cluster, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	if util.Deref(current.State) == management.ClusterStateACTIVE {
		// A resume that timed out earlier has finished since.
		return clusterModelFromAPI(current, adminPassword, plan.FirewallRanges, plan.ProjectName, plan.ExpiresAt)
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

	return clusterModelFromAPI(cluster, adminPassword, plan.FirewallRanges, plan.ProjectName, plan.ExpiresAt)
}

func suspend(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, plan clusterResourceModel, current management.Cluster, adminPassword string) (clusterResourceModel, *util.SummaryWithDetailError) {
	if util.Deref(current.State) == management.ClusterStateSUSPENDED {
		// A suspend that timed out earlier has finished since.
		return clusterModelFromAPI(current, adminPassword, plan.FirewallRanges, plan.ProjectName, plan.ExpiresAt)
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

	return clusterModelFromAPI(cluster, adminPassword, plan.FirewallRanges, plan.ProjectName, plan.ExpiresAt)
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

func knownAdminPassword(value types.String) (string, bool) {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return "", false
	}

	return value.ValueString(), true
}

func clusterModelFromAPI(cluster management.Cluster, adminPassword string, firewallRanges []types.String, projectName, expiresAt types.String) (clusterResourceModel, *util.SummaryWithDetailError) {
	model, diags := toClusterResourceModel(cluster, adminPassword, firewallRanges, projectName, expiresAt)
	if err := errorFromDiagnostics(diags); err != nil {
		return clusterResourceModel{}, err
	}

	return model, nil
}
