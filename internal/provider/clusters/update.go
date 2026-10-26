package clusters

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

func applyClusterConfigOrToggleSuspension(ctx context.Context, c management.ClientWithResponsesInterface, state, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) {
	// ModifyPlan already rejects changing suspended together with other configuration.
	// Prefer suspend/resume routing when suspended toggles so computed-only plan noise
	// (e.g. unknown update_window) cannot divert the call to PATCH.
	if suspendedChanged := !plan.Suspended.Equal(state.Suspended); suspendedChanged {
		if plan.Suspended.ValueBool() {
			return suspend(ctx, c, plan)
		}

		return resume(ctx, c, plan)
	}

	if hasGeneralConfigChanged(state, plan) {
		return applyClusterConfiguration(ctx, c, state, plan)
	}

	return state, nil
}

func hasGeneralConfigChanged(state, plan clusterResourceModel) bool { //nolint:cyclop
	return !plan.Size.Equal(state.Size) ||
		!plan.CacheConfig.Equal(state.CacheConfig) ||
		!plan.ScaleFactor.Equal(state.ScaleFactor) ||
		!plan.AdminPassword.Equal(state.AdminPassword) ||
		!equalFirewallRangesLists(state.FirewallRanges, plan.FirewallRanges) ||
		!plan.ExpiresAt.Equal(state.ExpiresAt) ||
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

func applyClusterConfiguration(ctx context.Context, c management.ClientWithResponsesInterface, state, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) { //nolint:cyclop
	id := uuid.MustParse(plan.ID.ValueString())
	desiredSize := plan.Size.ValueString()

	var projectID *uuid.UUID
	if util.IsConfiguredString(plan.ProjectName) {
		resolved, err := resolveProjectIDByName(ctx, c, plan.ProjectName.ValueString())
		if err != nil {
			return clusterResourceModel{}, err
		}
		projectID = resolved
	} else {
		current, gerr := c.GetV2ClustersClusterIDWithResponse(ctx, id, &management.GetV2ClustersClusterIDParams{})
		if serr := util.StatusOK(current, gerr); serr != nil {
			return clusterResourceModel{}, serr
		}
		projectID = &current.JSON200.ProjectID
	}

	firewallRanges := util.StringFirewallRanges(plan.FirewallRanges)
	patchBody := management.Cluster{
		Name:      plan.Name.ValueString(),
		ProjectID: *projectID,
	}

	if !plan.Size.Equal(state.Size) || !plan.CacheConfig.Equal(state.CacheConfig) || !plan.ScaleFactor.Equal(state.ScaleFactor) {
		patchBody.SizeConfig = &management.SizeConfig{
			Size:        util.MaybeString(plan.Size),
			ScaleFactor: util.MaybeFloat32(plan.ScaleFactor),
			CacheConfig: util.MaybeFloat32(plan.CacheConfig),
		}
	}

	if !plan.AdminPassword.Equal(state.AdminPassword) {
		patchBody.AdminPassword = util.MaybeNonEmptyString(plan.AdminPassword)
	}

	if !equalFirewallRangesLists(state.FirewallRanges, plan.FirewallRanges) {
		patchBody.FirewallRanges = &firewallRanges
	}

	if !plan.ExpiresAt.Equal(state.ExpiresAt) {
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

	if !plan.UpdateWindow.Equal(state.UpdateWindow) {
		patchBody.UpdateWindow = toManagementUpdateWindow(ctx, plan.UpdateWindow)
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

	cluster, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout, conditions...)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return toClusterResourceModel(cluster, plan.AdminPassword.ValueString(), plan.FirewallRanges, plan.ProjectName), nil
}

func resume(ctx context.Context, c management.ClientWithResponsesInterface, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	clusterResumeResponse, err := c.PostV2ClustersClusterIDResumeWithResponse(ctx, id, management.ClusterResume{})
	if serr := util.StatusOK(clusterResumeResponse, err); serr != nil {
		return clusterResourceModel{}, serr
	}

	cluster, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.ClusterStateACTIVE),
	)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return toClusterResourceModel(cluster, plan.AdminPassword.ValueString(), plan.FirewallRanges, plan.ProjectName), nil
}

func suspend(ctx context.Context, c management.ClientWithResponsesInterface, plan clusterResourceModel) (clusterResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	clusterSuspendResponse, err := c.PostV2ClustersClusterIDSuspendWithResponse(ctx, id)
	if serr := util.StatusOK(clusterSuspendResponse, err); serr != nil {
		return clusterResourceModel{}, serr
	}

	cluster, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.ClusterStateSUSPENDED),
	)
	if werr != nil {
		return clusterResourceModel{}, werr
	}

	return toClusterResourceModel(cluster, plan.AdminPassword.ValueString(), plan.FirewallRanges, plan.ProjectName), nil
}
