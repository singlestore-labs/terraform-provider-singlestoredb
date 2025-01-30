package workspaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// updateWorkspace updates workspace configuration(deploymentType, autoSuspend, autoScale, size) and suspends/resumes if necessary.
func updateWorkspace(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	if !isWorkspaceUpdated(state, plan) && plan.Suspended.Equal(state.Suspended) {
		return state, nil
	}

	if isWorkspaceUpdated(state, plan) {
		return update(ctx, c, state, plan)
	}

	if suspendedChanged := !plan.Suspended.Equal(state.Suspended); suspendedChanged {
		if plan.Suspended.ValueBool() {
			return suspend(ctx, c, plan)
		}

		return resume(ctx, c, plan)
	}

	return state, nil
}

func isWorkspaceUpdated(state, plan workspaceResourceModel) bool {
	if !plan.Size.Equal(state.Size) ||
		(!plan.DeploymentType.IsNull() && !plan.DeploymentType.IsUnknown() && !plan.DeploymentType.Equal(state.DeploymentType)) ||
		(!plan.ScaleFactor.IsNull() && !plan.ScaleFactor.IsUnknown() && !plan.ScaleFactor.Equal(state.ScaleFactor)) {
		return true
	}

	return false
}

func update(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	desiredSize := plan.Size.ValueString()

	worspaceUpdate := management.WorkspaceUpdate{}

	if !plan.Size.Equal(state.Size) {
		worspaceUpdate.Size = util.Ptr(desiredSize)
	}

	if !plan.DeploymentType.IsNull() && !plan.DeploymentType.IsUnknown() && !plan.DeploymentType.Equal(state.DeploymentType) {
		worspaceUpdate.DeploymentType = util.WorkspaceDeploymentTypeString(plan.DeploymentType)
	}

	if !plan.ScaleFactor.IsNull() && !plan.ScaleFactor.IsUnknown() && !plan.ScaleFactor.Equal(state.ScaleFactor) {
		worspaceUpdate.ScaleFactor = util.MaybeFloat32(plan.ScaleFactor)
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
