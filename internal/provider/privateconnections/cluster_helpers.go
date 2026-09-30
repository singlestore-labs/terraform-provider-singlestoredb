package privateconnections

import (
	"context"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// resolveClusterID prefers cluster_id, then workspace_id (deprecated alias), then looks up
// any cluster in workspace_group_id.
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

	groupID := uuid.MustParse(plan.WorkspaceGroupID.ValueString())
	clustersResp, err := c.GetV2ClustersWithResponse(ctx, &management.GetV2ClustersParams{})
	if serr := util.StatusOK(clustersResp, err); serr != nil {
		return uuid.UUID{}, serr
	}

	for _, cluster := range util.Deref(clustersResp.JSON200) {
		if cluster.GroupID != nil && *cluster.GroupID == groupID && cluster.ClusterID != nil {
			return *cluster.ClusterID, nil
		}
	}

	return uuid.UUID{}, &util.SummaryWithDetailError{
		Summary: "No cluster found in workspace group",
		Detail:  "Could not resolve a cluster ID from workspace_group_id. Set cluster_id or workspace_id explicitly.",
	}
}
