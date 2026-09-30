package workspaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// updateWorkspace updates workspace configuration(deploymentType, size) and suspends/resumes if necessary.
func applyWorkspaceConfigOrToggleSuspension(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	if hasGeneralConfigChanged(state, plan) {
		return applyWorkspaceConfiguration(ctx, c, state, plan)
	}

	if suspendedChanged := !plan.Suspended.Equal(state.Suspended); suspendedChanged {
		if plan.Suspended.ValueBool() {
			return suspend(ctx, c, plan)
		}

		return resume(ctx, c, plan)
	}

	return state, nil
}

func hasGeneralConfigChanged(state, plan workspaceResourceModel) bool {
	return !plan.Size.Equal(state.Size) ||
		!plan.CacheConfig.Equal(state.CacheConfig) ||
		!plan.ScaleFactor.Equal(state.ScaleFactor) ||
		!plan.AutoScale.MaxScaleFactor.Equal(state.AutoScale.MaxScaleFactor) || !plan.AutoScale.Sensitivity.Equal(state.AutoScale.Sensitivity) ||
		!plan.AutoSuspend.SuspendType.Equal(state.AutoSuspend.SuspendType) || !plan.AutoSuspend.SuspendAfterSeconds.Equal(state.AutoSuspend.SuspendAfterSeconds)
}

func applyWorkspaceConfiguration(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	desiredSize := plan.Size.ValueString()

	clusterUpdate := management.Cluster{
		Name: plan.Name.ValueString(),
	}

	sizeConfig := &management.SizeConfig{}
	sizeChanged := false
	if !plan.Size.Equal(state.Size) {
		sizeConfig.Size = util.Ptr(desiredSize)
		sizeChanged = true
	}

	if !plan.CacheConfig.Equal(state.CacheConfig) {
		sizeConfig.CacheConfig = util.MaybeFloat32(plan.CacheConfig)
		sizeChanged = true
	}

	if !plan.ScaleFactor.Equal(state.ScaleFactor) {
		sizeConfig.ScaleFactor = util.MaybeFloat32(plan.ScaleFactor)
		sizeChanged = true
	}

	if sizeChanged {
		clusterUpdate.SizeConfig = sizeConfig
	}

	if !plan.AutoScale.MaxScaleFactor.Equal(state.AutoScale.MaxScaleFactor) ||
		!plan.AutoScale.Sensitivity.Equal(state.AutoScale.Sensitivity) {
		clusterUpdate.AutoScale = toAutoScale(plan)
	}

	if !plan.AutoSuspend.SuspendType.Equal(state.AutoSuspend.SuspendType) ||
		!plan.AutoSuspend.SuspendAfterSeconds.Equal(state.AutoSuspend.SuspendAfterSeconds) {
		clusterUpdate.AutoSuspend = toClusterAutoSuspend(plan)
	}

	workspaceUpdateResponse, err := c.PatchV2ClustersClusterIDWithResponse(ctx, id, clusterUpdate)
	if serr := util.StatusOK(workspaceUpdateResponse, err); serr != nil {
		return workspaceResourceModel{}, serr
	}

	workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.ClusterStateACTIVE),
		waitConditionSize(desiredSize),
		waitConditionTakesAtLeast(config.WorkspaceScaleTakesAtLeast),
	)
	if werr != nil {
		return workspaceResourceModel{}, werr
	}

	return toWorkspaceResourceModel(workspace), nil
}

func resume(ctx context.Context, c management.ClientWithResponsesInterface, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	workspaceResumeResponse, err := c.PostV2ClustersClusterIDResumeWithResponse(ctx, id, management.ClusterResume{})
	if serr := util.StatusOK(workspaceResumeResponse, err); serr != nil {
		return workspaceResourceModel{}, serr
	}

	workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.ClusterStateACTIVE),
	)
	if werr != nil {
		return workspaceResourceModel{}, werr
	}

	return toWorkspaceResourceModel(workspace), nil
}

func suspend(ctx context.Context, c management.ClientWithResponsesInterface, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	workspaceSuspendResponse, err := c.PostV2ClustersClusterIDSuspendWithResponse(ctx, id)
	if serr := util.StatusOK(workspaceSuspendResponse, err); serr != nil {
		return workspaceResourceModel{}, serr
	}

	workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.ClusterStateSUSPENDED),
	)
	if werr != nil {
		return workspaceResourceModel{}, werr
	}

	return toWorkspaceResourceModel(workspace), nil
}
