package workspaces

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
)

type updateSizeScenario int

const (
	sameSize updateSizeScenario = iota
	fromSuspendedToActive
	fromActiveToSuspended
	fromActiveToActive
)

var updateSizeScenarios = []struct { //nolint:gochecknoglobals
	scenario        updateSizeScenario
	matchesScenario func(state, plan workspaceResourceModel) bool
	updateSize      func(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError)
}{
	{
		scenario: sameSize,
		matchesScenario: func(state, plan workspaceResourceModel) bool {
			return plan.Size.Equal(state.Size)
		},
		updateSize: func(_ context.Context, _ management.ClientWithResponsesInterface, state, _ workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
			return state, nil
		},
	},
	{
		scenario: fromSuspendedToActive,
		matchesScenario: func(state, plan workspaceResourceModel) bool {
			return state.Size.ValueString() == config.WorkspaceSizeSuspended &&
				plan.Size.ValueString() != config.WorkspaceSizeSuspended
		},
		updateSize: func(ctx context.Context, c management.ClientWithResponsesInterface, _, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
			id := uuid.MustParse(plan.ID.ValueString())
			if rerr := resume(ctx, c, id); rerr != nil {
				return workspaceResourceModel{}, rerr
			}

			workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
				waitConditionState(management.WorkspaceStateACTIVE),
			)
			if werr != nil {
				return workspaceResourceModel{}, werr
			}

			size, perr := ParseSize(workspace.Size, workspace.State)
			if perr != nil {
				return workspaceResourceModel{}, perr
			}

			desiredSize, perr := ParseSize(plan.Size.ValueString(), workspace.State)
			if perr != nil {
				return workspaceResourceModel{}, perr
			}

			if size.Eq(desiredSize) {
				result, terr := toWorkspaceResourceModel(workspace)
				if terr != nil {
					return workspaceResourceModel{}, terr
				}

				return result, nil // Early exit because no scale necessary.
			}

			if serr := scale(ctx, c, id, desiredSize); serr != nil {
				return workspaceResourceModel{}, serr
			}

			workspace, werr = wait(ctx, c, id, config.WorkspaceResumeTimeout,
				waitConditionState(management.WorkspaceStateACTIVE),
				waitConditionSize(desiredSize),
			)
			if werr != nil {
				return workspaceResourceModel{}, werr
			}

			result, terr := toWorkspaceResourceModel(workspace)
			if terr != nil {
				return workspaceResourceModel{}, terr
			}

			return result, nil
		},
	},
	{
		scenario: fromActiveToSuspended,
		matchesScenario: func(state, plan workspaceResourceModel) bool {
			return state.Size.ValueString() != config.WorkspaceSizeSuspended &&
				plan.Size.ValueString() == config.WorkspaceSizeSuspended
		},
		updateSize: func(ctx context.Context, c management.ClientWithResponsesInterface, _, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
			id := uuid.MustParse(plan.ID.ValueString())
			if rerr := suspend(ctx, c, id); rerr != nil {
				return workspaceResourceModel{}, rerr
			}

			workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
				waitConditionState(management.WorkspaceStateSUSPENDED),
			)
			if werr != nil {
				return workspaceResourceModel{}, werr
			}

			result, terr := toWorkspaceResourceModel(workspace)
			if terr != nil {
				return workspaceResourceModel{}, terr
			}

			return result, nil
		},
	},
	{
		scenario: fromActiveToActive,
		matchesScenario: func(state, plan workspaceResourceModel) bool {
			return !plan.Size.Equal(state.Size) &&
				state.Size.ValueString() != config.WorkspaceSizeSuspended &&
				plan.Size.ValueString() != config.WorkspaceSizeSuspended
		},
		updateSize: func(ctx context.Context, c management.ClientWithResponsesInterface, _, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
			id := uuid.MustParse(plan.ID.ValueString())

			desiredSize, perr := ParseSize(plan.Size.ValueString(), management.WorkspaceStateACTIVE)
			if perr != nil {
				return workspaceResourceModel{}, perr
			}

			if serr := scale(ctx, c, id, desiredSize); serr != nil {
				return workspaceResourceModel{}, serr
			}

			workspace, werr := wait(ctx, c, id, config.WorkspaceResumeTimeout,
				waitConditionState(management.WorkspaceStateACTIVE),
				waitConditionSize(desiredSize),
			)
			if werr != nil {
				return workspaceResourceModel{}, werr
			}

			result, terr := toWorkspaceResourceModel(workspace)
			if terr != nil {
				return workspaceResourceModel{}, terr
			}

			return result, nil
		},
	},
}

// updateSize brings a workspace to the desired state of the size and returns the previous workspace
// state with the relevant size.
//
// The following table summarizes the process of resolving the size update.
//
// .-----------------------------------------------------------.
// | ID | Current Size  | Desired Size  | Actions              |
// | -- | ------------  | ------------  | -------------------- |
// | 0  | 0 (suspended) | 0 (suspended) | None                 |
// | 1  | 0 (suspended) | Any (active)  | Resume & Maybe Scale |
// | 2  | Any (active)  | 0 (suspended) | Suspend              |
// | 3  | Any (active)  | Any (active)  | Scale                |
// .-----------------------------------------------------------.
func updateSize(ctx context.Context, c management.ClientWithResponsesInterface, state, plan workspaceResourceModel) (workspaceResourceModel, *util.SummaryWithDetailError) {
	if plan.Size.Equal(state.Size) {
		return state, nil
	}

	for _, u := range updateSizeScenarios {
		if u.matchesScenario(state, plan) {
			return u.updateSize(ctx, c, state, plan)
		}
	}

	return workspaceResourceModel{}, &util.SummaryWithDetailError{
		Summary: fmt.Sprintf("An internal error occurred while resolving the workspace size change from %s to %s.", state.Size.ValueString(), plan.Size.ValueString()),
		Detail:  "Please, contact the provider developers.",
	}
}

func resume(ctx context.Context, c management.ClientWithResponsesInterface, id management.WorkspaceID) *util.SummaryWithDetailError {
	workspaceResumeResponse, err := c.PostV1WorkspacesWorkspaceIDResumeWithResponse(ctx, id)
	if serr := util.StatusOK(workspaceResumeResponse, err); serr != nil {
		return serr
	}

	return nil
}

func suspend(ctx context.Context, c management.ClientWithResponsesInterface, id management.WorkspaceID) *util.SummaryWithDetailError {
	workspaceSuspendResponse, err := c.PostV1WorkspacesWorkspaceIDSuspendWithResponse(ctx, id)
	if serr := util.StatusOK(workspaceSuspendResponse, err); serr != nil {
		return serr
	}

	return nil
}

func scale(ctx context.Context, c management.ClientWithResponsesInterface, id management.WorkspaceID, size Size) *util.SummaryWithDetailError {
	workspaceUpdateResponse, err := c.PatchV1WorkspacesWorkspaceIDWithResponse(ctx, id,
		management.WorkspaceUpdate{
			Size: util.Ptr(size.String()),
		},
	)
	if serr := util.StatusOK(workspaceUpdateResponse, err); serr != nil {
		return serr
	}

	return nil
}
