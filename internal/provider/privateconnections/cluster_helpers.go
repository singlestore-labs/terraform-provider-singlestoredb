package privateconnections

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// resolveClusterID prefers cluster_id, then workspace_id (a workspace ID is a cluster ID),
// then the only active workspace in workspace_group_id. cluster_id and workspace_id are
// aliases, so both may be set only when they are the same value.
func resolveClusterID(ctx context.Context, c management.ClientWithResponsesInterface, plan PrivateConnectionModel) (uuid.UUID, *util.SummaryWithDetailError) {
	if err := conflictingClusterIdentifiers(plan); err != nil {
		return uuid.UUID{}, err
	}

	if util.IsConfiguredString(plan.ClusterID) {
		return uuid.MustParse(plan.ClusterID.ValueString()), nil
	}

	if util.IsConfiguredString(plan.WorkspaceID) {
		return uuid.MustParse(plan.WorkspaceID.ValueString()), nil
	}

	if !util.IsConfiguredString(plan.WorkspaceGroupID) {
		return uuid.UUID{}, &util.SummaryWithDetailError{
			Summary: "Missing cluster identifier",
			Detail:  "Set cluster_id (preferred), workspace_id, or workspace_group_id so the private connection can target a cluster.",
		}
	}

	return resolveClusterIDFromGroup(ctx, c, uuid.MustParse(plan.WorkspaceGroupID.ValueString()))
}

// lookupWorkspaceGroupID finds the group for a cluster. The v2 private connection
// API does not return a workspace group ID, so import has to look it up. A missing
// workspace leaves the alias unset instead of failing the read.
func lookupWorkspaceGroupID(ctx context.Context, c management.ClientWithResponsesInterface, clusterID uuid.UUID) (*uuid.UUID, *util.SummaryWithDetailError) {
	workspace, err := c.GetV1WorkspacesWorkspaceIDWithResponse(ctx, clusterID, &management.GetV1WorkspacesWorkspaceIDParams{})
	if serr := util.StatusOK(workspace, err, util.ReturnNilOnNotFound); serr != nil {
		return nil, serr
	}
	if workspace == nil || workspace.JSON200 == nil {
		return nil, nil
	}

	groupID := workspace.JSON200.WorkspaceGroupID

	return &groupID, nil
}

// restoreWorkspaceGroupID fills the deprecated alias when the v2 connection does not
// return it and state does not already have it. A missing workspace leaves the alias unset.
func restoreWorkspaceGroupID(ctx context.Context, c management.ClientWithResponsesInterface, model *PrivateConnectionModel) *util.SummaryWithDetailError {
	if util.IsConfiguredString(model.WorkspaceGroupID) || !util.IsConfiguredString(model.ClusterID) {
		return nil
	}

	groupID, gerr := lookupWorkspaceGroupID(ctx, c, uuid.MustParse(model.ClusterID.ValueString()))
	if gerr != nil || groupID == nil {
		return gerr
	}
	model.WorkspaceGroupID = util.UUIDStringValue(*groupID)

	return nil
}

func resolveClusterIDFromGroup(ctx context.Context, c management.ClientWithResponsesInterface, groupID uuid.UUID) (uuid.UUID, *util.SummaryWithDetailError) {
	workspacesResp, err := c.GetV1WorkspacesWithResponse(ctx, &management.GetV1WorkspacesParams{WorkspaceGroupID: groupID})
	if serr := util.StatusOK(workspacesResp, err); serr != nil {
		return uuid.UUID{}, serr
	}

	var candidates []uuid.UUID
	for _, workspace := range util.Deref(workspacesResp.JSON200) {
		if workspace.WorkspaceGroupID == groupID && workspace.State != management.WorkspaceStateTERMINATED {
			candidates = append(candidates, workspace.WorkspaceID)
		}
	}

	switch len(candidates) {
	case 0:
		return uuid.UUID{}, &util.SummaryWithDetailError{
			Summary: "No workspace found in workspace group",
			Detail:  "Could not resolve a workspace from workspace_group_id. Set cluster_id or workspace_id explicitly.",
		}
	case 1:
		return candidates[0], nil
	default:
		return uuid.UUID{}, &util.SummaryWithDetailError{
			Summary: "Ambiguous workspace group",
			Detail:  fmt.Sprintf("Workspace group %s contains %d active workspaces. Set cluster_id explicitly to choose the cluster for the private connection.", groupID, len(candidates)),
		}
	}
}
