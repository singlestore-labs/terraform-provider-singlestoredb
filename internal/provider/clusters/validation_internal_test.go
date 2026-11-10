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
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

// planWith returns validClusterPlan changed by mutate.
func planWith(mutate func(*clusterResourceModel)) clusterResourceModel {
	plan := validClusterPlan()
	mutate(&plan)

	return plan
}

// requireErr checks err against want: nil when want is empty, otherwise a
// summary or detail that contains want.
func requireErr(t *testing.T, want string, err *util.SummaryWithDetailError) {
	t.Helper()
	if want == "" {
		require.Nil(t, err)

		return
	}
	require.NotNil(t, err)
	require.Contains(t, err.Summary+"\n"+err.Detail, want)
}

func TestValidateCreateParameters(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*clusterResourceModel)
		want   string
	}{
		"valid":              {mutate: func(*clusterResourceModel) {}},
		"missing project":    {mutate: func(p *clusterResourceModel) { p.ProjectName = types.StringNull() }, want: "Missing project_name"},
		"missing provider":   {mutate: func(p *clusterResourceModel) { p.CloudProvider = types.StringNull() }, want: "Invalid region configuration"},
		"missing region":     {mutate: func(p *clusterResourceModel) { p.CloudProvider, p.RegionName = types.StringNull(), types.StringNull() }, want: "POST /v2/clusters"},
		"preview production": {mutate: func(p *clusterResourceModel) { p.OptInPreviewFeature = types.BoolValue(true) }, want: "NON-PRODUCTION"},
		"preview omitted type": {mutate: func(p *clusterResourceModel) {
			p.OptInPreviewFeature = types.BoolValue(true)
			p.DeploymentType = types.StringNull()
		}, want: "NON-PRODUCTION"},
		"disabled suspend with seconds": {mutate: func(p *clusterResourceModel) { p.AutoSuspend.SuspendAfterSeconds = types.Int64Value(60) }, want: "Invalid auto_suspend configuration."},
		"idle suspend without seconds":  {mutate: func(p *clusterResourceModel) { p.AutoSuspend.SuspendType = types.StringValue("IDLE") }, want: "suspend_after_seconds is required"},
		"scheduled without seconds":     {mutate: func(p *clusterResourceModel) { p.AutoSuspend.SuspendType = types.StringValue("SCHEDULED") }, want: "suspend_after_seconds is required"},
		"idle suspend with seconds": {mutate: func(p *clusterResourceModel) {
			p.AutoSuspend.SuspendType = types.StringValue("IDLE")
			p.AutoSuspend.SuspendAfterSeconds = types.Int64Value(60)
		}},
		"unknown suspend seconds":       {mutate: func(p *clusterResourceModel) { p.AutoSuspend.SuspendAfterSeconds = types.Int64Unknown() }},
		"sensitivity without autoscale": {mutate: func(p *clusterResourceModel) { p.AutoScale.Sensitivity = types.StringValue("HIGH") }, want: "Invalid auto_scale configuration."},
		"unknown autoscale sensitivity": {mutate: func(p *clusterResourceModel) { p.AutoScale.Sensitivity = types.StringUnknown() }},
	} {
		t.Run(name, func(t *testing.T) {
			plan := planWith(tc.mutate)
			requireErr(t, tc.want, validateCreateParameters(&plan))
		})
	}
}

func TestValidatePlannedCreate(t *testing.T) {
	known := validClusterPlan()
	for name, tc := range map[string]struct {
		mutate                     func(*clusterResourceModel)
		configProject, configDType types.String
		want                       string
	}{
		"valid":              {mutate: func(*clusterResourceModel) {}, configProject: known.ProjectName, configDType: known.DeploymentType},
		"omitted project":    {mutate: func(*clusterResourceModel) {}, configProject: types.StringNull(), configDType: known.DeploymentType, want: "Missing project_name"},
		"unknown project":    {mutate: func(p *clusterResourceModel) { p.ProjectName = types.StringUnknown() }, configProject: types.StringUnknown(), configDType: known.DeploymentType},
		"unknown region":     {mutate: func(p *clusterResourceModel) { p.RegionName = types.StringUnknown() }, configProject: known.ProjectName, configDType: known.DeploymentType},
		"missing region":     {mutate: func(p *clusterResourceModel) { p.RegionName = types.StringNull() }, configProject: known.ProjectName, configDType: known.DeploymentType, want: "Invalid region configuration"},
		"preview production": {mutate: func(p *clusterResourceModel) { p.OptInPreviewFeature = types.BoolValue(true) }, configProject: known.ProjectName, configDType: known.DeploymentType, want: "NON-PRODUCTION"},
		"preview type unknown": {mutate: func(p *clusterResourceModel) {
			p.OptInPreviewFeature = types.BoolValue(true)
			p.DeploymentType = types.StringUnknown()
		}, configProject: known.ProjectName, configDType: types.StringUnknown()},
		// An omitted deployment type is unknown in the plan but means PRODUCTION.
		"preview type omitted": {mutate: func(p *clusterResourceModel) {
			p.OptInPreviewFeature = types.BoolValue(true)
			p.DeploymentType = types.StringUnknown()
		}, configProject: known.ProjectName, configDType: types.StringNull(), want: "NON-PRODUCTION"},
		// A known-bad scale still fails when another create input is not yet known.
		"bad scale, unknown project": {mutate: func(p *clusterResourceModel) {
			p.AutoScale.Sensitivity = types.StringValue("HIGH")
			p.ProjectName = types.StringUnknown()
		}, configProject: known.ProjectName, configDType: known.DeploymentType, want: "Invalid auto_scale configuration."},
		"unknown scale factor": {mutate: func(p *clusterResourceModel) {
			p.AutoScale.MaxScaleFactor = types.Float32Unknown()
			p.AutoScale.Sensitivity = types.StringValue("HIGH")
		}, configProject: known.ProjectName, configDType: known.DeploymentType},
	} {
		t.Run(name, func(t *testing.T) {
			plan := planWith(tc.mutate)
			requireErr(t, tc.want, validatePlannedCreate(&plan, tc.configProject, tc.configDType))
		})
	}
}

func TestValidateModify(t *testing.T) {
	for name, tc := range map[string]struct {
		state, plan func(*clusterResourceModel)
		check       func(plan, state *clusterResourceModel) *util.SummaryWithDetailError
		want        string
	}{
		"project unchanged":     {check: validateModifyProjectName},
		"project unset in plan": {plan: func(p *clusterResourceModel) { p.ProjectName = types.StringNull() }, check: validateModifyProjectName},
		"project unknown":       {plan: func(p *clusterResourceModel) { p.ProjectName = types.StringUnknown() }, check: validateModifyProjectName},
		"project set from null": {state: func(s *clusterResourceModel) { s.ProjectName = types.StringNull() }, check: validateModifyProjectName, want: "Cannot update cluster project_name"},
		"project changed":       {state: func(s *clusterResourceModel) { s.ProjectName = types.StringValue("Other Project") }, check: validateModifyProjectName, want: "Other Project"},
		"expires_at unchanged":  {check: validateModifyExpiresAt},
		"expires_at changed": {
			state: func(s *clusterResourceModel) { s.ExpiresAt = types.StringValue("2221-01-02T15:04:05Z") },
			plan:  func(p *clusterResourceModel) { p.ExpiresAt = types.StringValue("2222-01-02T15:04:05Z") },
			check: validateModifyExpiresAt, want: "Cannot change cluster expires_at",
		},
		"expires_at removed": {
			state: func(s *clusterResourceModel) { s.ExpiresAt = types.StringValue("2221-01-02T15:04:05Z") },
			check: validateModifyExpiresAt, want: "Cannot change cluster expires_at",
		},
		// Import stores the normalized API timestamp. Another RFC3339 spelling of that
		// instant is not a change, so it must not block a later update.
		"expires_at fractional spelling": {
			state: func(s *clusterResourceModel) { s.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123Z") },
			plan:  func(p *clusterResourceModel) { p.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123000Z") },
			check: validateModifyExpiresAt,
		},
		"expires_at zero fraction": {
			state: func(s *clusterResourceModel) { s.ExpiresAt = types.StringValue("2222-01-01T00:00:00Z") },
			plan:  func(p *clusterResourceModel) { p.ExpiresAt = types.StringValue("2222-01-01T00:00:00.000Z") },
			check: validateModifyExpiresAt,
		},
		"no change": {check: flip(validateSuspendedAndConfigChanges)},
		"suspend with resize": {
			plan:  func(p *clusterResourceModel) { p.Suspended, p.Size = types.BoolValue(true), types.StringValue("S-1") },
			check: flip(validateSuspendedAndConfigChanges), want: "suspension state",
		},
		"resize while suspended": {
			state: func(s *clusterResourceModel) { s.Suspended = types.BoolValue(true) },
			plan:  func(p *clusterResourceModel) { p.Suspended, p.Size = types.BoolValue(true), types.StringValue("S-1") },
			check: flip(validateSuspendedAndConfigChanges), want: "suspended cluster",
		},
		"update with bad scale": {
			plan:  func(p *clusterResourceModel) { p.AutoScale.Sensitivity = types.StringValue("HIGH") },
			check: flip(validateSuspendedAndConfigChanges), want: "Invalid auto_scale configuration.",
		},
		"update with bad suspend": {
			plan:  func(p *clusterResourceModel) { p.AutoSuspend.SuspendAfterSeconds = types.Int64Value(10) },
			check: flip(validateSuspendedAndConfigChanges), want: "Invalid auto_suspend configuration.",
		},
	} {
		t.Run(name, func(t *testing.T) {
			noop := func(*clusterResourceModel) {}
			if tc.state == nil {
				tc.state = noop
			}
			if tc.plan == nil {
				tc.plan = noop
			}
			state, plan := planWith(tc.state), planWith(tc.plan)
			requireErr(t, tc.want, tc.check(&plan, &state))
		})
	}
}

// flip adapts a (state, plan) check to the (plan, state) order of the table above.
func flip(f func(state, plan *clusterResourceModel) *util.SummaryWithDetailError) func(plan, state *clusterResourceModel) *util.SummaryWithDetailError {
	return func(plan, state *clusterResourceModel) *util.SummaryWithDetailError { return f(state, plan) }
}

func TestHasGeneralConfigChanged(t *testing.T) {
	ab := []types.String{types.StringValue("10.0.0.0/8"), types.StringValue("192.168.0.0/16")}
	ba := []types.String{types.StringValue("192.168.0.0/16"), types.StringValue("10.0.0.0/8")}
	for name, tc := range map[string]struct {
		state, plan func(*clusterResourceModel)
		want        bool
	}{
		"same": {},
		"equivalent expires_at": {
			state: func(s *clusterResourceModel) { s.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123Z") },
			plan:  func(p *clusterResourceModel) { p.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123000Z") },
		},
		"different expires_at": {
			state: func(s *clusterResourceModel) { s.ExpiresAt = types.StringValue("2222-01-01T00:00:00.123Z") },
			plan:  func(p *clusterResourceModel) { p.ExpiresAt = types.StringValue("2223-01-01T00:00:00Z") },
			want:  true,
		},
		"admin password only":   {plan: func(p *clusterResourceModel) { p.AdminPassword = types.StringValue("another-generated-pw") }},
		"firewall ranges":       {plan: func(p *clusterResourceModel) { p.FirewallRanges = ab[:1] }, want: true},
		"reordered ranges":      {state: func(s *clusterResourceModel) { s.FirewallRanges = ab }, plan: func(p *clusterResourceModel) { p.FirewallRanges = ba }},
		"unknown update window": {state: func(s *clusterResourceModel) { s.UpdateWindow = types.ObjectUnknown(updateWindowAttrTypes) }},
		"size":                  {plan: func(p *clusterResourceModel) { p.Size = types.StringValue("S-1") }, want: true},
		"auto suspend":          {plan: func(p *clusterResourceModel) { p.AutoSuspend.SuspendType = types.StringValue("IDLE") }, want: true},
	} {
		t.Run(name, func(t *testing.T) {
			noop := func(*clusterResourceModel) {}
			if tc.state == nil {
				tc.state = noop
			}
			if tc.plan == nil {
				tc.plan = noop
			}
			require.Equal(t, tc.want, hasGeneralConfigChanged(planWith(tc.state), planWith(tc.plan)))
		})
	}
}

func TestToClusterResourceModel(t *testing.T) {
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	apiExpiresAt := "2222-01-01 00:00:00.123 +0000 UTC"
	configuredExpiresAt := "2222-01-01T00:00:00.123000Z"
	model := toClusterResourceModel(management.Cluster{
		Name:      "cluster",
		ClusterID: &clusterID,
		CreatedAt: &createdAt,
		ExpiresAt: &apiExpiresAt,
		AutoSuspend: &management.AutoSuspend{
			SuspendType:      util.Ptr(management.AutoSuspendSuspendTypeIDLE),
			IdleAfterSeconds: util.Ptr(120),
		},
		AutoScale: &management.AutoScale{
			MaxScaleFactor: util.Ptr[float32](2),
			Sensitivity:    util.Ptr(management.LOW),
		},
		UpdateWindow:  &management.UpdateWindow{Hour: 6, Day: 2},
		AdminPassword: util.Ptr("from-api"),
	}, "secret", clusterResourceModel{ExpiresAt: types.StringValue(configuredExpiresAt)}, types.StringValue("Standard Project"))
	require.Equal(t, "from-api", model.AdminPassword.ValueString())
	require.Equal(t, clusterID.String(), model.ID.ValueString())
	require.Equal(t, configuredExpiresAt, model.ExpiresAt.ValueString())
	require.Equal(t, string(management.ClusterDeploymentTypePRODUCTION), model.DeploymentType.ValueString())
	require.True(t, model.Size.IsNull())
	require.Equal(t, int64(120), model.AutoSuspend.SuspendAfterSeconds.ValueInt64())
	require.Equal(t, float32(2), model.AutoScale.MaxScaleFactor.ValueFloat32())
	require.Equal(t, string(management.LOW), model.AutoScale.Sensitivity.ValueString())
	require.Equal(t, int64(6), model.UpdateWindow.Attributes()["hour"].(types.Int64).ValueInt64())

	nonProduction := management.ClusterDeploymentTypeNONPRODUCTION
	explicit := toClusterResourceModel(management.Cluster{DeploymentType: &nonProduction}, "",
		clusterResourceModel{DeploymentType: types.StringValue("PRODUCTION")}, types.StringNull())
	require.Equal(t, string(nonProduction), explicit.DeploymentType.ValueString())
	require.True(t, explicit.AdminPassword.IsNull())
	require.True(t, explicit.UpdateWindow.IsNull())

	kept := toClusterResourceModel(management.Cluster{}, "kept-secret", clusterResourceModel{}, types.StringNull())
	require.Equal(t, "kept-secret", kept.AdminPassword.ValueString())
	require.False(t, kept.Kai.ValueBool())

	// Fields the response omits keep the planned value, so apply matches the plan.
	planned := clusterResourceModel{DeploymentType: types.StringValue(string(nonProduction)), Kai: types.BoolValue(true)}
	omitted := toClusterResourceModel(management.Cluster{}, "", planned, types.StringNull())
	require.Equal(t, string(nonProduction), omitted.DeploymentType.ValueString())
	require.True(t, omitted.Kai.ValueBool())

	// A value the API returns wins over the plan.
	require.False(t, toClusterResourceModel(management.Cluster{Kai: util.Ptr(false)}, "", planned, types.StringNull()).Kai.ValueBool())
}

func TestStateConverters(t *testing.T) {
	require.True(t, normalizeCloudProvider(nil).IsNull())
	require.Equal(t, "AWS", normalizeCloudProvider(util.Ptr(management.CloudProviderAWS)).ValueString())
	require.Equal(t, "DigitalOcean", normalizeCloudProvider(util.Ptr(management.CloudProvider("DigitalOcean"))).ValueString())

	for name, tc := range map[string]struct {
		in          *management.AutoSuspend
		wantType    string
		wantSeconds types.Int64
	}{
		"omitted":   {wantType: "DISABLED", wantSeconds: types.Int64Null()},
		"disabled":  {in: &management.AutoSuspend{SuspendType: util.Ptr(management.AutoSuspendSuspendTypeDISABLED)}, wantType: "DISABLED", wantSeconds: types.Int64Null()},
		"scheduled": {in: &management.AutoSuspend{SuspendType: util.Ptr(management.AutoSuspendSuspendTypeSCHEDULED), ScheduledAfterSeconds: util.Ptr(30)}, wantType: "SCHEDULED", wantSeconds: types.Int64Value(30)},
	} {
		got := toAutoSuspendResourceModel(management.Cluster{AutoSuspend: tc.in})
		require.Equal(t, tc.wantType, got.SuspendType.ValueString(), name)
		require.Equal(t, tc.wantSeconds, got.SuspendAfterSeconds, name)
	}

	for name, tc := range map[string]struct {
		in              *management.AutoScale
		wantFactor      float32
		wantSensitivity management.AutoScaleSensitivity
	}{
		"factor only":      {in: &management.AutoScale{MaxScaleFactor: util.Ptr[float32](1)}, wantFactor: scaleX1, wantSensitivity: management.NORMAL},
		"sensitivity only": {in: &management.AutoScale{Sensitivity: util.Ptr(management.LOW)}, wantFactor: scaleX1, wantSensitivity: management.LOW},
	} {
		got := toAutoScaleResourceModel(management.Cluster{AutoScale: tc.in})
		require.Equal(t, tc.wantFactor, got.MaxScaleFactor.ValueFloat32(), name)
		require.Equal(t, string(tc.wantSensitivity), got.Sensitivity.ValueString(), name)
	}

	require.Nil(t, int64ToIntPtr(types.Int64Null()))
	require.Nil(t, int64ToIntPtr(types.Int64Unknown()))
	require.Equal(t, 4, *int64ToIntPtr(types.Int64Value(4)))
	require.True(t, intPtrToInt64(nil).IsNull())
	require.Equal(t, int64(4), intPtrToInt64(util.Ptr(4)).ValueInt64())
}

func TestClusterPlanConverters(t *testing.T) {
	require.Nil(t, toAutoScale(validClusterPlan()).Sensitivity)

	scaled := planWith(func(p *clusterResourceModel) {
		p.AutoScale.MaxScaleFactor = types.Float32Value(2)
		p.AutoScale.Sensitivity = types.StringValue(string(management.LOW))
	})
	require.Equal(t, management.LOW, *toAutoScale(scaled).Sensitivity)
	require.Equal(t, float32(2), *toCreateAutoScale(scaled).MaxScaleFactor)

	idle := planWith(func(p *clusterResourceModel) {
		p.AutoSuspend.SuspendType = types.StringValue(string(management.AutoSuspendSuspendTypeIDLE))
		p.AutoSuspend.SuspendAfterSeconds = types.Int64Value(90)
	})
	require.Equal(t, 90, *toAutoSuspend(idle).IdleAfterSeconds)

	scheduled := planWith(func(p *clusterResourceModel) {
		p.AutoSuspend.SuspendType = types.StringValue(string(management.AutoSuspendSuspendTypeSCHEDULED))
		p.AutoSuspend.SuspendAfterSeconds = types.Int64Value(30)
	})
	require.Equal(t, 30, *toAutoSuspend(scheduled).ScheduledAfterSeconds)

	window := types.ObjectValueMust(updateWindowAttrTypes, map[string]attr.Value{
		"hour": types.Int64Value(6),
		"day":  types.Int64Value(2),
	})
	require.Equal(t, &management.UpdateWindow{Hour: 6, Day: 2}, toManagementUpdateWindow(window))
	require.Nil(t, toManagementUpdateWindow(types.ObjectNull(updateWindowAttrTypes)))
	require.Nil(t, toManagementUpdateWindow(types.ObjectUnknown(updateWindowAttrTypes)))
}

func TestResolveProjectNameByID(t *testing.T) {
	projectID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	otherID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	okClient := projectClient(t, http.StatusOK, []management.Project{{Name: "Standard Project", ProjectID: projectID}})
	got, err := resolveProjectNameByID(t.Context(), okClient, projectID)
	require.Nil(t, err)
	require.Equal(t, "Standard Project", got.ValueString())

	missing, err := resolveProjectNameByID(t.Context(), okClient, otherID)
	require.Nil(t, err)
	require.True(t, missing.IsNull())

	_, err = resolveProjectNameByID(t.Context(), projectClient(t, http.StatusInternalServerError, nil), projectID)
	require.NotNil(t, err)
}

// projectClient returns a Management API client that serves the given projects.
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
			SuspendAfterSeconds: types.Int64Null(),
			SuspendType:         types.StringValue(string(management.AutoSuspendSuspendTypeDISABLED)),
		},
		UpdateWindow: types.ObjectNull(updateWindowAttrTypes),
	}
}

// TestModifyPlanUnknownValues does not treat a value that is not known yet as a change.
func TestModifyPlanUnknownValues(t *testing.T) {
	require.False(t, changedKnown(types.StringUnknown(), types.StringValue("cluster")))
	require.False(t, changedKnown(types.StringValue("cluster"), types.StringValue("cluster")))
	require.True(t, changedKnown(types.StringValue("renamed"), types.StringValue("cluster")))

	plan := validClusterPlan()
	state := validClusterPlan()
	plan.ProjectName = types.StringUnknown()
	require.Nil(t, validateModifyProjectName(&plan, &state))

	deploymentType, known := effectiveDeploymentType(types.StringUnknown())
	require.False(t, known)
	require.Empty(t, deploymentType)
	deploymentType, known = effectiveDeploymentType(types.StringNull())
	require.True(t, known)
	require.Equal(t, string(management.ClusterDeploymentTypePRODUCTION), deploymentType)
}

// TestModifyPlanUnknownNestedBlock skips plan checks while auto_suspend is unknown
// instead of failing to decode the plan.
func TestModifyPlanUnknownNestedBlock(t *testing.T) {
	ctx := t.Context()
	r := &clusterResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	s := schemaResp.Schema

	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	model := validClusterPlan()
	model.ID = types.StringUnknown()
	model.GroupID = types.StringUnknown()
	model.OutboundAllowList = types.StringUnknown()
	model.Endpoint = types.StringUnknown()
	model.CreatedAt = types.StringUnknown()
	model.MultiAZ = types.BoolValue(false)
	model.UpdateWindow = types.ObjectUnknown(updateWindowAttrTypes)
	require.False(t, plan.Set(ctx, &model).HasError())

	unknown, diags := hasUnknownNestedBlock(ctx, plan)
	require.False(t, diags.HasError())
	require.False(t, unknown)

	autoSuspendType := s.Attributes["auto_suspend"].GetType().(types.ObjectType)
	require.False(t, plan.SetAttribute(ctx, path.Root("auto_suspend"), types.ObjectUnknown(autoSuspendType.AttrTypes)).HasError())

	unknown, diags = hasUnknownNestedBlock(ctx, plan)
	require.False(t, diags.HasError())
	require.True(t, unknown)

	// Decoding this plan into the model fails, which is why ModifyPlan checks first.
	var decoded clusterResourceModel
	require.True(t, plan.Get(ctx, &decoded).HasError())

	resp := &resource.ModifyPlanResponse{Plan: plan}
	r.ModifyPlan(ctx, resource.ModifyPlanRequest{
		Plan:   plan,
		State:  tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)},
		Config: tfsdk.Config{Schema: s, Raw: plan.Raw},
	}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
}
