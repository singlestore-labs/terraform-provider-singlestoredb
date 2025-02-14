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
	if !plan.Size.Equal(state.Size) ||
		!plan.CacheConfig.Equal(state.CacheConfig) ||
		!plan.ScaleFactor.Equal(state.ScaleFactor) ||
		!plan.AutoScale.MaxScaleFactor.Equal(state.AutoScale.MaxScaleFactor) ||
		!plan.AutoScale.Sensitivity.Equal(state.AutoScale.Sensitivity) {
		!plan.AutoSuspend.SuspendType.Equal(state.AutoSuspend.SuspendType) ||
		!plan.AutoSuspend.SuspendAfterSeconds.Equal(state.AutoSuspend.SuspendAfterSeconds) {
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

func applyWorkspaceConfiguration(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	desiredSize := plan.Size.ValueString()

	worspaceUpdate := management.WorkspaceUpdate{}

	if !plan.Size.Equal(state.Size) {
		worspaceUpdate.Size = util.Ptr(desiredSize)
	}

	if !plan.CacheConfig.Equal(state.CacheConfig) {
		worspaceUpdate.CacheConfig = util.MaybeFloat32(plan.CacheConfig)
	}

	if !plan.ScaleFactor.Equal(state.ScaleFactor) {
		worspaceUpdate.ScaleFactor = util.MaybeFloat32(plan.ScaleFactor)
	}

	if !plan.AutoScale.MaxScaleFactor.Equal(state.AutoScale.MaxScaleFactor) ||
		!plan.AutoScale.Sensitivity.Equal(state.AutoScale.Sensitivity) {
		worspaceUpdate.AutoScale = toAutoScale(plan)
	}

	if !plan.AutoSuspend.SuspendType.Equal(state.AutoSuspend.SuspendType) ||
		!plan.AutoSuspend.SuspendAfterSeconds.Equal(state.AutoSuspend.SuspendAfterSeconds) {
		worspaceUpdate.AutoSuspend = toUpdateAutoSuspend(plan)
	}

	workspaceUpdateResponse, err := c.PatchV1WorkspacesWorkspaceIDWithResponse(ctx, id, worspaceUpdate)
	if serr := util.StatusOK(workspaceUpdateResponse, err); serr != nil {
		return workspaceResourceModel{}, serr
	}

	workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.WorkspaceStateACTIVE),
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
	workspaceResumeResponse, err := c.PostV1WorkspacesWorkspaceIDResumeWithResponse(ctx, id, management.WorkspaceResume{})
	if serr := util.StatusOK(workspaceResumeResponse, err); serr != nil {
		return workspaceResourceModel{}, serr
	}

	workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.WorkspaceStateACTIVE),
	)
	if werr != nil {
		return workspaceResourceModel{}, werr
	}

	return toWorkspaceResourceModel(workspace), nil
}

func suspend(ctx context.Context, c management.ClientWithResponsesInterface, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	workspaceSuspendResponse, err := c.PostV1WorkspacesWorkspaceIDSuspendWithResponse(ctx, id)
	if serr := util.StatusOK(workspaceSuspendResponse, err); serr != nil {
		return workspaceResourceModel{}, serr
	}

	workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
		waitConditionState(management.WorkspaceStateSUSPENDED),
	)
	if werr != nil {
		return workspaceResourceModel{}, werr
	}

	return toWorkspaceResourceModel(workspace), nil
}

func toUpdateAutoSuspend(plan workspaceResourceModel) *struct {
	SuspendAfterSeconds *float32                                          `json:"suspendAfterSeconds,omitempty"`
	SuspendType         *management.WorkspaceUpdateAutoSuspendSuspendType `json:"suspendType,omitempty"`
} {
	return &struct {
		SuspendAfterSeconds *float32                                          `json:"suspendAfterSeconds,omitempty"`
		SuspendType         *management.WorkspaceUpdateAutoSuspendSuspendType `json:"suspendType,omitempty"`
	}{
		SuspendAfterSeconds: util.MaybeFloat32(plan.AutoSuspend.SuspendAfterSeconds),
		SuspendType:         util.WorkspaceUpdateAutoSuspendSuspendTypeString(plan.AutoSuspend.SuspendType),
	}
}
