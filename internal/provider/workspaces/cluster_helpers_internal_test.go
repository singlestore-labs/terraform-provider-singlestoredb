package workspaces

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestSoleAdoptableCluster(t *testing.T) {
	groupID := uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")
	other := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	starter := management.Cluster{Name: "group", GroupID: &groupID}
	extra := management.Cluster{Name: "other", GroupID: &groupID}

	_, ok := soleAdoptableCluster(nil, "workspace")
	require.False(t, ok)
	_, ok = soleAdoptableCluster([]management.Cluster{starter, extra}, "workspace")
	require.False(t, ok)
	_, ok = soleAdoptableCluster([]management.Cluster{starter}, " Group ")
	require.False(t, ok)

	got, ok := soleAdoptableCluster([]management.Cluster{starter}, "workspace")
	require.True(t, ok)
	require.Equal(t, "group", got.Name)

	filtered := filterClustersByGroupID([]management.Cluster{starter, {Name: "elsewhere", GroupID: &other}, {}}, groupID)
	require.Equal(t, []management.Cluster{starter}, filtered)
	require.False(t, clusterNameExists(filtered, "missing"))
}

func TestWorkspacePatchHelpers(t *testing.T) {
	require.Nil(t, float32ToIntPtr(nil))
	require.Equal(t, 3, *float32ToIntPtr(util.Ptr[float32](3)))
	require.Nil(t, intToFloat32Ptr(nil))
	require.Equal(t, float32(3), *intToFloat32Ptr(util.Ptr(3)))

	idle := workspacePlan(string(management.IDLE), 90)
	got := toClusterAutoSuspend(idle)
	require.Equal(t, management.IDLE, *got.SuspendType)
	require.Equal(t, 90, *got.IdleAfterSeconds)

	scheduled := toClusterAutoSuspend(workspacePlan(string(management.SCHEDULED), 45))
	require.Equal(t, 45, *scheduled.ScheduledAfterSeconds)

	disabled := toClusterAutoSuspend(workspacePlan(string(management.DISABLED), 0))
	require.Nil(t, disabled.IdleAfterSeconds)
	require.Nil(t, disabled.ScheduledAfterSeconds)

	unknown := toClusterAutoSuspend(workspacePlan("NOPE", 10))
	require.Nil(t, unknown.SuspendType)

	starter := management.Cluster{SizeConfig: &management.SizeConfig{Size: util.Ptr("S-00"), CacheConfig: util.Ptr[float32](1), ScaleFactor: util.Ptr[float32](1)}}
	same := workspacePlan(string(management.DISABLED), 0)
	require.False(t, sizeConfigNeedsPatch(starter, same))
	resized := same
	resized.Size = types.StringValue("S-1")
	require.True(t, sizeConfigNeedsPatch(starter, resized))
	cached := same
	cached.CacheConfig = types.Float32Value(4)
	require.True(t, sizeConfigNeedsPatch(management.Cluster{}, cached))

	require.False(t, autoSuspendNeedsPatch(starter, same))
	require.True(t, autoSuspendNeedsPatch(starter, idle))
	require.False(t, autoSuspendNeedsPatch(management.Cluster{AutoSuspend: &management.AutoSuspend{SuspendType: util.Ptr(management.DISABLED)}}, same))
	require.True(t, autoSuspendNeedsPatch(management.Cluster{AutoSuspend: &management.AutoSuspend{SuspendType: util.Ptr(management.IDLE)}}, same))

	_, err := adoptStarterCluster(t.Context(), nil, management.Cluster{}, same)
	require.NotNil(t, err)
	require.Equal(t, "Missing cluster ID", err.Summary)

	require.Nil(t, clusterCacheConfig(management.Cluster{}))
	require.Nil(t, clusterScaleFactor(management.Cluster{}))
	require.False(t, autoSuspendNeedsPatch(management.Cluster{}, workspacePlan("NOPE", 10)))
}

func TestAdoptStarterClusterPatchError(t *testing.T) {
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	starter := management.Cluster{
		ClusterID: &clusterID,
		Name:      "group",
		Kai:       util.Ptr(false),
		SizeConfig: &management.SizeConfig{
			Size:        util.Ptr("S-00"),
			CacheConfig: util.Ptr[float32](1),
			ScaleFactor: util.Ptr[float32](1),
		},
	}
	plan := workspacePlan(string(management.IDLE), 90)
	plan.Size = types.StringValue("S-1")
	plan.KaiEnabled = types.BoolValue(true)
	plan.AutoScale.MaxScaleFactor = types.Float32Value(2)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	_, serr := adoptStarterCluster(t.Context(), client, starter, plan)
	require.NotNil(t, serr)
}

func workspacePlan(suspendType string, seconds float32) workspaceResourceModel {
	after := types.Float32Null()
	if suspendType != string(management.DISABLED) {
		after = types.Float32Value(seconds)
	}

	return workspaceResourceModel{
		Name:        types.StringValue("workspace"),
		Size:        types.StringValue("S-00"),
		CacheConfig: types.Float32Value(1),
		ScaleFactor: types.Float32Value(1),
		KaiEnabled:  types.BoolValue(false),
		AutoSuspend: &workspaceAutoSuspendResourceModel{
			SuspendType:         types.StringValue(suspendType),
			SuspendAfterSeconds: after,
		},
		AutoScale: &autoScaleResourceModel{
			MaxScaleFactor: types.Float32Value(1),
			Sensitivity:    types.StringValue(string(management.NORMAL)),
		},
	}
}
