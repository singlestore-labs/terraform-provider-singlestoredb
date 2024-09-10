package workspaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// updateSizeOrSuspended either scales or suspends/resumes if necessary.
func updateSizeOrSuspended(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	if plan.Size.Equal(state.Size) && plan.Suspended.Equal(state.Suspended) {
		return state, nil
	}

	if sizeChanged := !plan.Size.Equal(state.Size); sizeChanged {
		return scale(ctx, c, plan)
	}

	if suspendedChanged := !plan.Suspended.Equal(state.Suspended); suspendedChanged {
		if plan.Suspended.ValueBool() {
			return suspend(ctx, c, plan)
		}

		return resume(ctx, c, plan)
	}

	return state, nil
}

func scale(ctx context.Context, c management.ClientWithResponsesInterface, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	id := uuid.MustParse(plan.ID.ValueString())
	desiredSize := plan.Size.ValueString()

	workspaceUpdateResponse, err := c.PatchV1WorkspacesWorkspaceIDWithResponse(ctx, id,
		management.WorkspaceUpdate{
			Size: util.Ptr(desiredSize),
		},
	)
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
