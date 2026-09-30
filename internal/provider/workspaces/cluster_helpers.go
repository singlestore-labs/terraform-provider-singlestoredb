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

func filterClustersByGroupID(clusters []management.Cluster, groupID otypes.UUID) []management.Cluster {
	result := make([]management.Cluster, 0)
	for _, c := range clusters {
		if c.GroupID != nil && *c.GroupID == groupID {
			result = append(result, c)
		}
	}

	return result
}

func clusterNameExists(clusters []management.Cluster, name string) bool {
	for _, c := range clusters {
		if strings.EqualFold(strings.TrimSpace(c.Name), strings.TrimSpace(name)) {
			return true
		}
	}

	return false
}

// soleAdoptableCluster returns the group's only cluster when it is still the
// workspace_group starter (not already renamed to the desired workspace name).
func soleAdoptableCluster(groupClusters []management.Cluster, workspaceName string) (management.Cluster, bool) {
	if len(groupClusters) != 1 {
		return management.Cluster{}, false
	}
	if clusterNameExists(groupClusters, workspaceName) {
		return management.Cluster{}, false
	}

	return groupClusters[0], true
}

// kaiPatchIfChanged returns a Kai value only when the plan requests a change from
// the live cluster. Omitting an unchanged false avoids PATCH bodies that disable Kai.
func kaiPatchIfChanged(plan types.Bool, current *bool) *bool {
	desired := util.MaybeBool(plan)
	if desired == nil || *desired == util.Deref(current) {
		return nil
	}

	return desired
}

// adoptStarterCluster renames/resizes the workspace_group starter cluster into the
// configured workspace. /v2/clusters cannot attach a second cluster to an existing
// GroupID, so this keeps password/firewall/group identity aligned for the classic
// workspace_group → workspace flow.
func adoptStarterCluster( //nolint:cyclop
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
	// Name is required on the Cluster PATCH shape but /v2/clusters often ignores renames.
	// Only send fields that actually differ from the starter to avoid long PENDING states
	// (e.g. rewriting default SizeConfig/AutoSuspend on an already-ready S-00 cluster).
	patch := management.Cluster{
		Name: plan.Name.ValueString(),
	}
	needsPatch := false
	desiredSize := plan.Size.ValueString()
	sizeChanged := sizeConfigNeedsPatch(starter, plan)
	if sizeChanged {
		patch.SizeConfig = toSizeConfig(plan)
		needsPatch = true
	}
	if autoScale := toCreateAutoScale(plan); autoScale != nil {
		patch.AutoScale = autoScale
		needsPatch = true
	}
	if autoSuspendNeedsPatch(starter, plan) {
		patch.AutoSuspend = toClusterAutoSuspend(plan)
		needsPatch = true
	}
	// kai_enabled defaults to false in the schema. Sending "kai": false on PATCH makes
	// /v2/clusters try to tear down mongoproxy and can 500 when it was never provisioned.
	if kai := kaiPatchIfChanged(plan.KaiEnabled, starter.Kai); kai != nil {
		patch.Kai = kai
		needsPatch = true
	}
	if needsPatch {
		updateResponse, err := c.PatchV2ClustersClusterIDWithResponse(ctx, id, patch)
		if serr := util.StatusOK(updateResponse, err); serr != nil {
			return management.Cluster{}, serr
		}
	}

	conditions := []waitCondition{
		waitConditionState(management.ClusterStateACTIVE),
	}
	if sizeChanged && desiredSize != "" && desiredSize != clusterSize(starter) {
		conditions = append(conditions,
			waitConditionSize(desiredSize),
			waitConditionTakesAtLeast(config.WorkspaceScaleTakesAtLeast),
		)
	}

	return wait(ctx, c, id, config.WorkspaceCreationTimeout, conditions...)
}

func sizeConfigNeedsPatch(starter management.Cluster, plan workspaceResourceModel) bool { //nolint:cyclop
	if plan.Size.ValueString() != "" && plan.Size.ValueString() != clusterSize(starter) {
		return true
	}

	starterCache, starterScale := float32(1), float32(1)
	if starter.SizeConfig != nil {
		if starter.SizeConfig.CacheConfig != nil {
			starterCache = *starter.SizeConfig.CacheConfig
		}
		if starter.SizeConfig.ScaleFactor != nil {
			starterScale = *starter.SizeConfig.ScaleFactor
		}
	}

	planCache, planScale := float32(1), float32(1)
	if !plan.CacheConfig.IsNull() && !plan.CacheConfig.IsUnknown() {
		planCache = plan.CacheConfig.ValueFloat32()
	}
	if !plan.ScaleFactor.IsNull() && !plan.ScaleFactor.IsUnknown() {
		planScale = plan.ScaleFactor.ValueFloat32()
	}

	return planCache != starterCache || planScale != starterScale
}

func autoSuspendNeedsPatch(starter management.Cluster, plan workspaceResourceModel) bool {
	desired := toClusterAutoSuspend(plan)
	if desired == nil || desired.SuspendType == nil {
		return false
	}
	if *desired.SuspendType == management.DISABLED {
		if starter.AutoSuspend == nil || starter.AutoSuspend.SuspendType == nil ||
			*starter.AutoSuspend.SuspendType == management.DISABLED {
			return false
		}
	}

	return true
}
