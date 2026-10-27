package privateconnections

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// resolveClusterID prefers cluster_id, then workspace_id (a workspace ID is a cluster ID),
// then the only active workspace in workspace_group_id.
func resolveClusterID(ctx context.Context, c management.ClientWithResponsesInterface, plan PrivateConnectionModel) (uuid.UUID, *util.SummaryWithDetailError) {
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
