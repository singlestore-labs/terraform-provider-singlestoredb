package workspacegroups

import (
	"context"
	"time"

	otypes "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const defaultStarterSize = "S-00"

func clusterState(c management.Cluster) management.ClusterState {
	return util.Deref(c.State)
}

func groupIDOrNil(c management.Cluster) *otypes.UUID {
	return c.GroupID
}

func findClusterByGroupID(clusters []management.Cluster, groupID uuid.UUID) (management.Cluster, bool) {
	for _, c := range clusters {
		if c.GroupID != nil && *c.GroupID == groupID {
			return c, true
		}
	}

	return management.Cluster{}, false
}

func filterClustersByGroupID(clusters []management.Cluster, groupID uuid.UUID) []management.Cluster {
	result := make([]management.Cluster, 0)
	for _, c := range clusters {
		if c.GroupID != nil && *c.GroupID == groupID {
			result = append(result, c)
		}
	}

	return result
}

// uniqueGroupsByGroupID returns one representative cluster per GroupID.
func uniqueGroupsByGroupID(clusters []management.Cluster) []management.Cluster {
	seen := make(map[uuid.UUID]struct{})
	result := make([]management.Cluster, 0)
	for _, c := range clusters {
		if c.GroupID == nil {
			continue
		}
		if _, ok := seen[*c.GroupID]; ok {
			continue
		}
		seen[*c.GroupID] = struct{}{}
		result = append(result, c)
	}

	return result
}

func listClusters(ctx context.Context, c management.ClientWithResponsesInterface) ([]management.Cluster, *util.SummaryWithDetailError) {
	resp, err := c.GetV2ClustersWithResponse(ctx, &management.GetV2ClustersParams{})
	if serr := util.StatusOK(resp, err); serr != nil {
		return nil, serr
	}

	return util.Deref(resp.JSON200), nil
}

func getClusterInGroup(ctx context.Context, c management.ClientWithResponsesInterface, groupID uuid.UUID) (management.Cluster, *util.SummaryWithDetailError) {
	clusters, serr := listClusters(ctx, c)
	if serr != nil {
		return management.Cluster{}, serr
	}

	cluster, ok := findClusterByGroupID(clusters, groupID)
	if !ok {
		return management.Cluster{}, &util.SummaryWithDetailError{
			Summary: "Workspace group not found",
			Detail:  "No cluster was found for the workspace group ID.",
		}
	}

	return cluster, nil
}

func clusterCreatedAtString(c management.Cluster) types.String {
	return util.MaybeTimeValue(c.CreatedAt)
}

func clusterTerminatedAtString(c management.Cluster) *string {
	if c.TerminatedAt == nil {
		return nil
	}

	s := c.TerminatedAt.Format(time.RFC3339)

	return &s
}

func resolveProjectName(ctx context.Context, c management.ClientWithResponsesInterface, projectID uuid.UUID) types.String {
	projectsResponse, err := c.GetV2ProjectsWithResponse(ctx)
	if serr := util.StatusOK(projectsResponse, err); serr != nil {
		return types.StringNull()
	}

	for _, p := range util.Deref(projectsResponse.JSON200) {
		if p.ProjectID == projectID {
			return types.StringValue(p.Name)
		}
	}

	return types.StringNull()
}
