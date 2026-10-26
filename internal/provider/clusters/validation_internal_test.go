package clusters

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestClusterSizeValidator(t *testing.T) {
	v := NewSizeValidator()
	ctx := t.Context()
	require.Contains(t, v.MarkdownDescription(ctx), "S-00")

	var skipped validator.StringResponse
	v.ValidateString(ctx, validator.StringRequest{Path: path.Root("size"), ConfigValue: types.StringNull()}, &skipped)
	v.ValidateString(ctx, validator.StringRequest{Path: path.Root("size"), ConfigValue: types.StringUnknown()}, &skipped)
	require.False(t, skipped.Diagnostics.HasError())

	var valid validator.StringResponse
	v.ValidateString(ctx, validator.StringRequest{Path: path.Root("size"), ConfigValue: types.StringValue("S-1")}, &valid)
	require.False(t, valid.Diagnostics.HasError())

	var invalid validator.StringResponse
	v.ValidateString(ctx, validator.StringRequest{Path: path.Root("size"), ConfigValue: types.StringValue("nope")}, &invalid)
	require.True(t, invalid.Diagnostics.HasError())
}

func TestValidateCreateParameters(t *testing.T) {
	plan := validClusterPlan()
	require.Nil(t, validateCreateParameters(&plan))

	missingProject := plan
	missingProject.ProjectName = types.StringNull()
	require.Equal(t, "Missing project_name", validateCreateParameters(&missingProject).Summary)

	regionOnly := plan
	regionOnly.CloudProvider = types.StringNull()
	require.Equal(t, "Invalid region configuration", validateCreateParameters(&regionOnly).Summary)

	preview := plan
	preview.OptInPreviewFeature = types.BoolValue(true)
	require.Contains(t, validateCreateParameters(&preview).Detail, "NON-PRODUCTION")

	badSuspend := plan
	suspend := *plan.AutoSuspend
	badSuspend.AutoSuspend = &suspend
	badSuspend.AutoSuspend.SuspendAfterSeconds = types.Float32Value(60)
	require.Equal(t, "Invalid auto_suspend configuration.", validateCreateParameters(&badSuspend).Summary)

	badScale := plan
	scale := *plan.AutoScale
	badScale.AutoScale = &scale
	badScale.AutoScale.Sensitivity = types.StringValue(string(management.HIGH))
	require.Equal(t, "Invalid auto_scale configuration.", validateCreateParameters(&badScale).Summary)
}

func TestValidateModifyProjectName(t *testing.T) {
	plan := validClusterPlan()
	state := validClusterPlan()
	require.Nil(t, validateModifyProjectName(&plan, &state))

	unset := plan
	unset.ProjectName = types.StringNull()
	require.Nil(t, validateModifyProjectName(&unset, &state))

	state.ProjectName = types.StringNull()
	require.Equal(t, "Cannot update cluster project_name", validateModifyProjectName(&plan, &state).Summary)

	state.ProjectName = types.StringValue("Other Project")
	require.Contains(t, validateModifyProjectName(&plan, &state).Detail, "Other Project")
}

func TestValidateSuspendedAndConfigChanges(t *testing.T) {
	state := validClusterPlan()
	plan := validClusterPlan()
	require.Nil(t, validateSuspendedAndConfigChanges(&state, &plan))

	plan.Suspended = types.BoolValue(true)
	plan.Size = types.StringValue("S-1")
	require.Contains(t, validateSuspendedAndConfigChanges(&state, &plan).Summary, "suspension state")

	plan.Suspended = types.BoolValue(true)
	state.Suspended = types.BoolValue(true)
	require.Contains(t, validateSuspendedAndConfigChanges(&state, &plan).Summary, "suspended cluster")
}

func TestClusterModelHelpers(t *testing.T) {
	require.True(t, normalizeCloudProvider(nil).IsNull())
	require.Equal(t, "AWS", normalizeCloudProvider(util.Ptr(management.CloudProviderAWS)).ValueString())
	unknown := management.CloudProvider("DigitalOcean")
	require.Equal(t, "DigitalOcean", normalizeCloudProvider(&unknown).ValueString())

	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	model := toClusterResourceModel(management.Cluster{
		Name:      "cluster",
		ClusterID: &clusterID,
		CreatedAt: &createdAt,
		AutoSuspend: &management.AutoSuspend{
			SuspendType:      util.Ptr(management.AutoSuspendSuspendTypeIDLE),
			IdleAfterSeconds: util.Ptr(120),
		},
		AutoScale: &management.AutoScale{
			MaxScaleFactor: util.Ptr[float32](2),
			Sensitivity:    util.Ptr(management.LOW),
		},
		UpdateWindow: &management.UpdateWindow{Hour: 6, Day: 2},
	}, "secret", nil, types.StringValue("Standard Project"))
	require.Equal(t, clusterID.String(), model.ID.ValueString())
	require.True(t, model.Size.IsNull())
	require.Equal(t, float32(120), model.AutoSuspend.SuspendAfterSeconds.ValueFloat32())
	require.False(t, model.UpdateWindow.IsNull())

	scheduled := toAutoSuspendResourceModel(management.Cluster{AutoSuspend: &management.AutoSuspend{
		SuspendType:           util.Ptr(management.AutoSuspendSuspendTypeSCHEDULED),
		ScheduledAfterSeconds: util.Ptr(30),
	}})
	require.Equal(t, float32(30), scheduled.SuspendAfterSeconds.ValueFloat32())

	plan := validClusterPlan()
	plan.AutoScale.MaxScaleFactor = types.Float32Value(2)
	require.Equal(t, float32(2), *toCreateAutoScale(plan).MaxScaleFactor)
	require.Nil(t, float32ToIntPtr(types.Float32Null()))
	require.Equal(t, 4, *float32ToIntPtr(types.Float32Value(4)))
	require.True(t, intPtrToFloat32(nil).IsNull())
	require.Equal(t, float32(4), intPtrToFloat32(util.Ptr(4)).ValueFloat32())

	state := validClusterPlan()
	changed := validClusterPlan()
	require.False(t, hasGeneralConfigChanged(state, changed))
	changed.FirewallRanges = []types.String{types.StringValue("10.0.0.0/8")}
	require.True(t, hasGeneralConfigChanged(state, changed))
	require.False(t, updateWindowChanged(types.ObjectUnknown(updateWindowAttrTypes), types.ObjectNull(updateWindowAttrTypes)))
	require.True(t, equalFirewallRangesLists(nil, nil))
}

func TestClusterPlanConverters(t *testing.T) {
	plan := validClusterPlan()
	require.Nil(t, toAutoScale(plan).Sensitivity)

	scaled := plan
	autoScale := *plan.AutoScale
	scaled.AutoScale = &autoScale
	scaled.AutoScale.MaxScaleFactor = types.Float32Value(2)
	scaled.AutoScale.Sensitivity = types.StringValue(string(management.LOW))
	require.Equal(t, management.LOW, *toAutoScale(scaled).Sensitivity)

	idle := plan
	idleSuspend := *plan.AutoSuspend
	idle.AutoSuspend = &idleSuspend
	idle.AutoSuspend.SuspendType = types.StringValue(string(management.AutoSuspendSuspendTypeIDLE))
	idle.AutoSuspend.SuspendAfterSeconds = types.Float32Value(90)
	require.Equal(t, 90, *toAutoSuspend(idle).IdleAfterSeconds)

	scheduled := plan
	scheduledSuspend := *plan.AutoSuspend
	scheduled.AutoSuspend = &scheduledSuspend
	scheduled.AutoSuspend.SuspendType = types.StringValue(string(management.AutoSuspendSuspendTypeSCHEDULED))
	scheduled.AutoSuspend.SuspendAfterSeconds = types.Float32Value(30)
	require.Equal(t, 30, *toAutoSuspend(scheduled).ScheduledAfterSeconds)

	window, diags := types.ObjectValue(
		updateWindowAttrTypes,
		map[string]attr.Value{
			"hour": types.Int64Value(6),
			"day":  types.Int64Value(2),
		},
	)
	require.False(t, diags.HasError())
	gotWindow := toManagementUpdateWindow(t.Context(), window)
	require.Equal(t, float32(6), gotWindow.Hour)
	require.Equal(t, float32(2), gotWindow.Day)
	require.Nil(t, toManagementUpdateWindow(t.Context(), types.ObjectNull(updateWindowAttrTypes)))
	require.Nil(t, toManagementUpdateWindow(t.Context(), types.ObjectUnknown(updateWindowAttrTypes)))

	state := validClusterPlan()
	badScale := validClusterPlan()
	badScaleConfig := *badScale.AutoScale
	badScale.AutoScale = &badScaleConfig
	badScale.AutoScale.Sensitivity = types.StringValue(string(management.HIGH))
	require.Equal(t, "Invalid auto_scale configuration.", validateSuspendedAndConfigChanges(&state, &badScale).Summary)

	badSuspend := validClusterPlan()
	badSuspendConfig := *badSuspend.AutoSuspend
	badSuspend.AutoSuspend = &badSuspendConfig
	badSuspend.AutoSuspend.SuspendAfterSeconds = types.Float32Value(10)
	require.Equal(t, "Invalid auto_suspend configuration.", validateSuspendedAndConfigChanges(&state, &badSuspend).Summary)
}

func TestFirewallRangeHelpers(t *testing.T) {
	require.Equal(t, []string{unrestrictedCIDR}, effectiveFirewallRanges(management.Cluster{
		AllowAllTraffic: util.Ptr(true),
	}))
	require.True(t, firewallRangesConverged(
		[]types.String{types.StringValue(unrestrictedCIDR), types.StringValue("10.0.0.0/8")},
		management.Cluster{AllowAllTraffic: util.Ptr(true)},
	))
	require.False(t, firewallRangesConverged(
		[]types.String{types.StringValue("10.0.0.0/8"), types.StringValue("10.1.0.0/8")},
		management.Cluster{FirewallRanges: util.Ptr([]string{"10.0.0.0/8"})},
	))
	require.False(t, firewallRangesConverged(
		[]types.String{types.StringValue("10.0.0.0/8")},
		management.Cluster{FirewallRanges: util.Ptr([]string{"10.1.0.0/8"})},
	))
}

func TestResolveProjectLookup(t *testing.T) {
	projectID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	otherID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	client := projectClient(t, http.StatusOK, []management.Project{
		{Name: "Other", ProjectID: otherID},
	})
	_, err := resolveProjectIDByName(t.Context(), client, "Standard Project")
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "Other")

	empty := projectClient(t, http.StatusOK, []management.Project{})
	_, err = resolveProjectIDByName(t.Context(), empty, "Standard Project")
	require.Equal(t, "Project not found", err.Summary)

	dup := projectClient(t, http.StatusOK, []management.Project{
		{Name: "Standard Project", ProjectID: projectID},
		{Name: "Standard Project", ProjectID: otherID},
	})
	_, err = resolveProjectIDByName(t.Context(), dup, "Standard Project")
	require.Equal(t, "Multiple projects found", err.Summary)

	okClient := projectClient(t, http.StatusOK, []management.Project{
		{Name: "Standard Project", ProjectID: projectID},
	})
	got, err := resolveProjectNameByID(t.Context(), okClient, projectID)
	require.Nil(t, err)
	require.Equal(t, "Standard Project", got.ValueString())
	missing, err := resolveProjectNameByID(t.Context(), okClient, otherID)
	require.Nil(t, err)
	require.True(t, missing.IsNull())

	failed := projectClient(t, http.StatusInternalServerError, nil)
	_, err = resolveProjectIDByName(t.Context(), failed, "Standard Project")
	require.NotNil(t, err)
	_, err = resolveProjectNameByID(t.Context(), failed, projectID)
	require.NotNil(t, err)
}

func projectClient(t *testing.T, status int, projects []management.Project) *management.ClientWithResponses {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if projects == nil {
			return
		}
		body, err := json.Marshal(projects)
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	return client
}

var updateWindowAttrTypes = map[string]attr.Type{
	"hour": types.Int64Type,
	"day":  types.Int64Type,
}

func validClusterPlan() clusterResourceModel {
	return clusterResourceModel{
		Name:                types.StringValue("cluster"),
		ProjectName:         types.StringValue("Standard Project"),
		Size:                types.StringValue("S-00"),
		ScaleFactor:         types.Float32Value(1),
		CacheConfig:         types.Float32Value(1),
		Suspended:           types.BoolValue(false),
		Kai:                 types.BoolValue(false),
		AdminPassword:       types.StringValue("mockPassword193!"),
		FirewallRanges:      []types.String{types.StringValue("0.0.0.0/0")},
		ExpiresAt:           types.StringNull(),
		CloudProvider:       types.StringValue("AWS"),
		RegionName:          types.StringValue("us-east-1"),
		DeploymentType:      types.StringValue(string(management.ClusterDeploymentTypePRODUCTION)),
		OptInPreviewFeature: types.BoolValue(false),
		AutoScale: &autoScaleResourceModel{
			MaxScaleFactor: types.Float32Value(1),
			Sensitivity:    types.StringValue(string(management.NORMAL)),
		},
		AutoSuspend: &autoSuspendResourceModel{
			SuspendAfterSeconds: types.Float32Null(),
			SuspendType:         types.StringValue(string(management.AutoSuspendSuspendTypeDISABLED)),
		},
		UpdateWindow: types.ObjectNull(updateWindowAttrTypes),
	}
}
