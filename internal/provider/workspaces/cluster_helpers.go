package workspaces

import (
	"context"
	"strings"

	otypes "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

func clusterIDValue(c management.Cluster) types.String {
	return util.MaybeUUIDStringValue(c.ClusterID)
}

func groupIDValue(c management.Cluster) types.String {
	return util.MaybeUUIDStringValue(c.GroupID)
}

func clusterState(c management.Cluster) management.ClusterState {
	return util.Deref(c.State)
}

func clusterSize(c management.Cluster) string {
	if c.SizeConfig == nil || c.SizeConfig.Size == nil {
		return ""
	}

	return *c.SizeConfig.Size
}

func clusterCacheConfig(c management.Cluster) *float32 {
	if c.SizeConfig == nil {
		return nil
	}

	return c.SizeConfig.CacheConfig
}

func clusterScaleFactor(c management.Cluster) *float32 {
	if c.SizeConfig == nil {
		return nil
	}

	return c.SizeConfig.ScaleFactor
}

func clusterCreatedAtString(c management.Cluster) types.String {
	return util.MaybeTimeValue(c.CreatedAt)
}

func clusterLastResumedAtString(c management.Cluster) types.String {
	return util.MaybeTimeValue(c.LastResumedAt)
}

func float32ToIntPtr(f *float32) *int {
	if f == nil {
		return nil
	}

	v := int(*f)

	return &v
}

func intToFloat32Ptr(i *int) *float32 {
	if i == nil {
		return nil
	}

	v := float32(*i)

	return &v
}

func toClusterAutoSuspend(plan workspaceResourceModel) *management.AutoSuspend {
	suspendType := util.AutoSuspendSuspendTypeString(plan.AutoSuspend.SuspendType)
	as := &management.AutoSuspend{
		SuspendType: suspendType,
	}

	seconds := float32ToIntPtr(util.MaybeFloat32(plan.AutoSuspend.SuspendAfterSeconds))
	if suspendType != nil {
		switch *suspendType {
		case management.IDLE:
			as.IdleAfterSeconds = seconds
		case management.SCHEDULED:
			as.ScheduledAfterSeconds = seconds
		case management.DISABLED:
		}
	}

	return as
}

func toSizeConfig(plan workspaceResourceModel) *management.SizeConfig {
	return &management.SizeConfig{
		Size:        util.MaybeString(plan.Size),
		CacheConfig: util.MaybeFloat32(plan.CacheConfig),
		ScaleFactor: util.MaybeFloat32(plan.ScaleFactor),
	}
}

// findClusterInGroup returns a representative cluster from the workspace group.
func findClusterInGroup(clusters []management.Cluster, groupID otypes.UUID) (management.Cluster, bool) {
	for _, c := range clusters {
		if c.GroupID != nil && *c.GroupID == groupID {
			return c, true
		}
	}

	return management.Cluster{}, false
}

func filterClustersByGroupID(clusters []management.Cluster, groupID otypes.UUID) []management.Cluster {
	result := make([]management.Cluster, 0)
	for _, c := range clusters {
		if c.GroupID != nil && *c.GroupID == groupID {
			result = append(result, c)
		}
	}

	return result
}

func findClusterByName(clusters []management.Cluster, name string) (management.Cluster, bool) {
	for _, c := range clusters {
		if strings.EqualFold(strings.TrimSpace(c.Name), strings.TrimSpace(name)) {
			return c, true
		}
	}

	return management.Cluster{}, false
}

// soleAdoptableCluster returns the group's only cluster when it is still the
// workspace_group starter (not already renamed to the desired workspace name).
func soleAdoptableCluster(groupClusters []management.Cluster, workspaceName string) (management.Cluster, bool) {
	if len(groupClusters) != 1 {
		return management.Cluster{}, false
	}
	if _, exists := findClusterByName(groupClusters, workspaceName); exists {
		return management.Cluster{}, false
	}

	return groupClusters[0], true
}

// adoptStarterCluster renames/resizes the workspace_group starter cluster into the
// configured workspace. /v2/clusters cannot attach a second cluster to an existing
// GroupID, so this keeps password/firewall/group identity aligned for the classic
// workspace_group → workspace flow.
func adoptStarterCluster(
	ctx context.Context,
	c management.ClientWithResponsesInterface,
	starter management.Cluster,
	plan workspaceResourceModel,
) (management.Cluster, *util.SummaryWithDetailError) {
	if starter.ClusterID == nil {
		return management.Cluster{}, &util.SummaryWithDetailError{
			Summary: "Missing cluster ID",
			Detail:  "The workspace group starter cluster response did not include a cluster ID.",
		}
	}

	id := *starter.ClusterID
	patch := management.Cluster{
		Name:        plan.Name.ValueString(),
		SizeConfig:  toSizeConfig(plan),
		Kai:         util.MaybeBool(plan.KaiEnabled),
		AutoSuspend: toClusterAutoSuspend(plan),
		AutoScale:   toCreateAutoScale(plan),
	}
	updateResponse, err := c.PatchV2ClustersClusterIDWithResponse(ctx, id, patch)
	if serr := util.StatusOK(updateResponse, err); serr != nil {
		return management.Cluster{}, serr
	}

	desiredSize := plan.Size.ValueString()
	conditions := []waitCondition{
		waitConditionState(management.ClusterStateACTIVE),
	}
	if desiredSize != "" && desiredSize != clusterSize(starter) {
		conditions = append(conditions,
			waitConditionSize(desiredSize),
			waitConditionTakesAtLeast(config.WorkspaceScaleTakesAtLeast),
		)
	}

	return wait(ctx, c, id, config.WorkspaceCreationTimeout, conditions...)
}
