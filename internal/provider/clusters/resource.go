package clusters

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/float32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float32default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	ResourceName = "cluster"

	cacheMultiplierX1 = 1
	cacheMultiplierX2 = 2
	cacheMultiplierX4 = 4
	scaleX1           = 1
	scaleX2           = 2
	scaleX4           = 4
)

var (
	_ resource.ResourceWithConfigure   = &clusterResource{}
	_ resource.ResourceWithModifyPlan  = &clusterResource{}
	_ resource.ResourceWithImportState = &clusterResource{}
)

var updateWindowAttrTypes = map[string]attr.Type{
	"hour": types.Int64Type,
	"day":  types.Int64Type,
}

// clusterResource is the resource implementation.
type clusterResource struct {
	management.ClientWithResponsesInterface
}

type updateWindowResourceModel struct {
	Hour types.Int64 `tfsdk:"hour"`
	Day  types.Int64 `tfsdk:"day"`
}

type autoScaleResourceModel struct {
	MaxScaleFactor types.Float32 `tfsdk:"max_scale_factor"`
	Sensitivity    types.String  `tfsdk:"sensitivity"`
}

type autoSuspendResourceModel struct {
	SuspendAfterSeconds types.Int64  `tfsdk:"suspend_after_seconds"`
	SuspendType         types.String `tfsdk:"suspend_type"`
}

// clusterResourceModel maps the resource schema data.
type clusterResourceModel struct {
	ID                  types.String              `tfsdk:"id"`
	Name                types.String              `tfsdk:"name"`
	ProjectName         types.String              `tfsdk:"project_name"`
	GroupID             types.String              `tfsdk:"group_id"`
	Size                types.String              `tfsdk:"size"`
	ScaleFactor         types.Float32             `tfsdk:"scale_factor"`
	CacheConfig         types.Float32             `tfsdk:"cache_config"`
	Suspended           types.Bool                `tfsdk:"suspended"`
	Kai                 types.Bool                `tfsdk:"kai"`
	AdminPassword       types.String              `tfsdk:"admin_password"`
	FirewallRanges      []types.String            `tfsdk:"firewall_ranges"`
	ExpiresAt           types.String              `tfsdk:"expires_at"`
	CloudProvider       types.String              `tfsdk:"cloud_provider"`
	RegionName          types.String              `tfsdk:"region_name"`
	DeploymentType      types.String              `tfsdk:"deployment_type"`
	OptInPreviewFeature types.Bool                `tfsdk:"opt_in_preview_feature"`
	MultiAZ             types.Bool                `tfsdk:"multi_az"`
	OutboundAllowList   types.String              `tfsdk:"outbound_allow_list"`
	Endpoint            types.String              `tfsdk:"endpoint"`
	CreatedAt           types.String              `tfsdk:"created_at"`
	AutoScale           *autoScaleResourceModel   `tfsdk:"auto_scale"`
	AutoSuspend         *autoSuspendResourceModel `tfsdk:"auto_suspend"`
	UpdateWindow        types.Object              `tfsdk:"update_window"`
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &clusterResource{}
}

// Metadata returns the resource type name.
func (r *clusterResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

// Schema defines the schema for the resource.
func (r *clusterResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	autoScaleDefaultValue, _ := basetypes.NewObjectValue(
		map[string]attr.Type{
			"max_scale_factor": basetypes.Float32Type{},
			"sensitivity":      basetypes.StringType{},
		},
		map[string]attr.Value{
			"max_scale_factor": basetypes.NewFloat32Value(scaleX1),
			"sensitivity":      basetypes.NewStringValue(string(management.NORMAL)),
		},
	)
	autoSuspendDefaultValue, _ := basetypes.NewObjectValue(
		map[string]attr.Type{
			"suspend_after_seconds": basetypes.Int64Type{},
			"suspend_type":          basetypes.StringType{},
		},
		map[string]attr.Value{
			"suspend_after_seconds": basetypes.NewInt64Null(),
			"suspend_type":          basetypes.NewStringValue(string(management.AutoSuspendSuspendTypeDISABLED)),
		},
	)
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage a SingleStoreDB cluster. Use this resource for new deployments. A cluster is one deployment. To move an existing `singlestoredb_workspace` and `singlestoredb_workspace_group`, remove both from state and import the workspace id as this resource.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the cluster.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the cluster. Must be between 1 and 32 characters.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 32), //nolint:mnd
				},
			},
			"project_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The name of the project to which the cluster is assigned. This value cannot be changed after the cluster is created. Use the `singlestoredb_projects` data source to get the available project names.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"group_id": schema.StringAttribute{
				Computed: true,
				MarkdownDescription: "The unique identifier of the workspace group that contains the cluster. " +
					"Destroy terminates this group when the cluster is its only workspace. " +
					"If another workspace uses the group, destroy terminates only this cluster and leaves the group in place.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"size": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The size of the cluster, specified in workspace size notation (S-00, S-0, S-1, S-2).",
				Validators:          []validator.String{NewSizeValidator()},
			},
			"scale_factor": schema.Float32Attribute{
				Computed:            true,
				Optional:            true,
				Default:             float32default.StaticFloat32(scaleX1),
				Validators:          []validator.Float32{float32validator.OneOf(scaleX1, scaleX2, scaleX4)},
				MarkdownDescription: "Specifies the scale factor for the cluster. The scale factor can be 1, 2 or 4. Default is 1.",
			},
			"cache_config": schema.Float32Attribute{
				Computed:            true,
				Optional:            true,
				Default:             float32default.StaticFloat32(cacheMultiplierX1),
				Validators:          []validator.Float32{float32validator.OneOf(cacheMultiplierX1, cacheMultiplierX2, cacheMultiplierX4)},
				MarkdownDescription: "Specifies the multiplier for the persistent cache associated with the cluster. It can have one of the following values: 1, 2, or 4. Default is 1.",
			},
			"suspended": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The status of the cluster. If true, the cluster is suspended. If omitted, Terraform does not suspend or resume the cluster and reports its current status, so a cluster suspended by `auto_suspend` is not resumed on apply. Must be false or omitted on create.",
				PlanModifiers: []planmodifier.Bool{
					// An omitted value is unknown; keep the refreshed status so plans show no diff.
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"kai": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether SingleStore Kai (MongoDB compatibility) is enabled for the cluster.",
				Default:             booldefault.StaticBool(false),
			},
			"admin_password": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				MarkdownDescription: "The admin SQL user password for the cluster. Management API generates this password and returns it only on create. " +
					"Terraform stores that value. Later responses omit the password, and refresh keeps the password already in state. " +
					"Setting a password is unavailable: the API ignores a caller-supplied password. Choosing a password may be added in a later release. " +
					"Read this attribute from state or from a sensitive output.",
				PlanModifiers: []planmodifier.String{
					// Computed-only, so a plan would otherwise leave this unknown and
					// an update would store an empty password over the one already in state.
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"firewall_ranges": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "List of allowed CIDR ranges. An empty list blocks all inbound requests. For unrestricted traffic, use [\"0.0.0.0/0\"]. Note that updates to firewall ranges may take a brief moment to become effective.",
			},
			"expires_at": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The expiration timestamp applied when the cluster is created. If unset, the cluster does not expire. " +
					"Upon expiration, the cluster is terminated and all its data is lost. " +
					"Set the expiration time as an RFC3339 UTC timestamp, for example \"2221-01-02T15:04:05Z\". " +
					"Changing expires_at after creation is unavailable: PATCH /v2/clusters ignores expiresAt. " +
					"After import, set expires_at to the cluster's current expiration. Equivalent RFC3339 spellings of that time are accepted.",
				Validators: []validator.String{util.NewTimeValidator()},
			},
			"cloud_provider": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The cloud provider of the cluster. Possible values are 'AWS', 'GCP', and 'Azure'. POST /v2/clusters requires this together with region_name. It cannot be changed after the cluster is created.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.CloudProviderAWS), string(management.CloudProviderGCP), string(management.CloudProviderAzure)),
				},
			},
			"region_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The region code name of the cluster, for example us-east-1. POST /v2/clusters requires this together with cloud_provider. It cannot be changed after the cluster is created.",
			},
			"deployment_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The deployment type of the cluster. It can have one of the following values: `PRODUCTION` or `NON-PRODUCTION`. The default value is `PRODUCTION`.",
				Default:             stringdefault.StaticString(string(management.ClusterDeploymentTypePRODUCTION)),
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.ClusterDeploymentTypePRODUCTION), string(management.ClusterDeploymentTypeNONPRODUCTION)),
				},
			},
			"opt_in_preview_feature": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "If enabled, the deployment gets the latest features and updates immediately. Suitable only for `NON-PRODUCTION` deployments and cannot be changed after creation.",
			},
			"multi_az": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Enables deployment across multiple availability zones.",
			},
			"outbound_allow_list": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The account ID which must be allowed for outbound connections. This is only applicable to AWS provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The endpoint used to connect to the cluster.",
			},
			"created_at": schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The timestamp when the cluster was created.",
			},
			"auto_scale": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				Default:             objectdefault.StaticValue(autoScaleDefaultValue),
				MarkdownDescription: "Specifies the autoscale setting (scale factor) for the cluster.",
				Attributes: map[string]schema.Attribute{
					"max_scale_factor": schema.Float32Attribute{
						Optional:   true,
						Computed:   true,
						Default:    float32default.StaticFloat32(scaleX1),
						Validators: []validator.Float32{float32validator.OneOf(scaleX1, scaleX2, scaleX4)},
						MarkdownDescription: "The maximum scale factor allowed for the cluster. It can have the following values: 1, 2, or 4. To disable autoscaling, set to 1. Default is 1. " +
							"When this is 4 on create, Terraform creates the cluster without autoscale and then sets it with an update, because POST /v2/clusters does not enable the 2x step for 4. " +
							"A cluster created at 4 by another tool may be unable to scale through 2x; GET does not report this, so set max_scale_factor to another value and back to 4 to repair it.",
					},
					"sensitivity": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Specifies the sensitivity of the autoscale operation to changes in the workload. It can have the following values: `LOW`, `NORMAL`, or `HIGH`. Default is `NORMAL`.",
						Default:             stringdefault.StaticString(string(management.NORMAL)),
						Validators: []validator.String{
							stringvalidator.OneOf(string(management.LOW), string(management.NORMAL), string(management.HIGH)),
						},
					},
				},
			},
			"auto_suspend": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Auto suspend settings for the cluster.",
				Default:             objectdefault.StaticValue(autoSuspendDefaultValue),
				Attributes: map[string]schema.Attribute{
					"suspend_after_seconds": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "When to suspend the cluster, in whole seconds, according to the suspend type chosen.",
					},
					"suspend_type": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "The auto suspend mode for the cluster can have the values `IDLE`, `SCHEDULED`, or `DISABLED`. Default is `DISABLED`.",
						Default:             stringdefault.StaticString(string(management.AutoSuspendSuspendTypeDISABLED)),
						Validators: []validator.String{
							stringvalidator.OneOf(string(management.AutoSuspendSuspendTypeDISABLED), string(management.AutoSuspendSuspendTypeIDLE), string(management.AutoSuspendSuspendTypeSCHEDULED)),
						},
					},
				},
			},
			// TODO: make update_window Optional when POST and PATCH /v2/clusters apply updateWindow.
			// Create already sends a configured window, and applyClusterConfiguration already sends a changed one.
			"update_window": schema.SingleNestedAttribute{
				Computed: true,
				MarkdownDescription: "The maintenance window the Management API assigned to the cluster. " +
					"Configuring update_window is unavailable: POST and PATCH /v2/clusters ignore updateWindow. " +
					"Terraform reports the window the API returns.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"hour": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "The hour of the day, in 24-hour UTC format (0-23), when the update window starts.",
						Validators: []validator.Int64{
							int64validator.Between(0, 23), //nolint:mnd
						},
					},
					"day": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "The day of the week (0-6), where 0 is Sunday and 6 is Saturday, when the update window is scheduled.",
						Validators: []validator.Int64{
							int64validator.Between(0, 6), //nolint:mnd
						},
					},
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *clusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan clusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Suspended.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			path.Root("suspended"),
			"Cannot suspend a cluster during creation",
			"Either set value to false or omit the field.",
		)

		return
	}

	if err := validateCreateParameters(&plan); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	projectID, err := util.ResolveProjectIDByName(ctx, r.ClientWithResponsesInterface, plan.ProjectName.ValueString(), "cluster")
	if err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	updateWindow, updateWindowDiags := toManagementUpdateWindow(ctx, plan.UpdateWindow)
	resp.Diagnostics.Append(updateWindowDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	firewallRanges := util.StringFirewallRanges(plan.FirewallRanges)
	createBody := management.Cluster{
		AutoScale:           toCreateAutoScale(plan),
		AutoSuspend:         toAutoSuspend(plan),
		DeploymentType:      util.ClusterDeploymentTypeString(plan.DeploymentType),
		ExpiresAt:           util.MaybeString(plan.ExpiresAt),
		FirewallRanges:      &firewallRanges,
		Kai:                 util.MaybeBool(plan.Kai),
		MultiAZ:             util.MaybeBool(plan.MultiAZ),
		Name:                plan.Name.ValueString(),
		OptInPreviewFeature: util.MaybeBool(plan.OptInPreviewFeature),
		ProjectID:           *projectID,
		Provider:            util.WorkspaceGroupCloudProviderString(plan.CloudProvider.ValueString()),
		Region:              util.MaybeString(plan.RegionName),
		SizeConfig: &management.SizeConfig{
			Size:        util.MaybeString(plan.Size),
			ScaleFactor: util.MaybeFloat32(plan.ScaleFactor),
			CacheConfig: util.MaybeFloat32(plan.CacheConfig),
		},
		UpdateWindow: updateWindow,
	}

	clusterCreateResponse, cerr := r.PostV2ClustersWithResponse(ctx, createBody)
	if serr := util.StatusOK(clusterCreateResponse, cerr); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	clusterID := clusterCreateResponse.JSON200.ClusterID
	// The create response is the first API object that carries the generated password.
	adminPassword := util.Deref(clusterCreateResponse.JSON200.AdminPassword)
	// Record the cluster before waiting for it to become active. Terraform keeps a
	// resource only when Create stores it in state, so a wait failure can still
	// refresh or destroy this cluster instead of creating another one.
	resp.Diagnostics.Append(resp.State.Set(ctx, clusterStateAfterCreate(plan, clusterID, clusterCreateResponse.JSON200.GroupID, adminPassword))...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError(
			"Cluster created but not stored in state",
			fmt.Sprintf("Cluster %s was created and is not tracked. Import it, or delete it in the SingleStore Portal, before applying again.", clusterID),
		)

		return
	}

	cluster, werr := wait(ctx, r.ClientWithResponsesInterface, clusterID, config.WorkspaceCreationTimeout,
		waitConditionState(management.ClusterStateACTIVE),
		waitConditionFirewallRanges(plan.FirewallRanges),
	)
	if werr != nil {
		resp.Diagnostics.AddError(werr.Summary, werr.Detail)

		return
	}

	cluster, autoScaleApplied := applyAutoScaleAfterCreate(ctx, r.ClientWithResponsesInterface, clusterID, cluster, plan, &resp.Diagnostics)

	result, modelDiags := toClusterResourceModel(cluster, adminPassword, plan.FirewallRanges, plan.ProjectName, plan.ExpiresAt)
	resp.Diagnostics.Append(modelDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	keepPlannedAutoScale(&result, plan, autoScaleApplied)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// keepPlannedAutoScale stores the planned autoscale when deferred setup failed.
// A failed setup leaves the API at the default factor of 1. Storing that would
// disagree with a planned factor of 4, and Terraform would reject the apply.
// The next refresh reads the API and can retry it.
func keepPlannedAutoScale(result *clusterResourceModel, plan clusterResourceModel, autoScaleApplied bool) {
	if autoScaleApplied || plan.AutoScale == nil {
		return
	}

	autoScale := *plan.AutoScale
	result.AutoScale = &autoScale
}

// createDefersAutoScale reports whether create must leave autoScale out of
// POST /v2/clusters and set it with PATCH once the cluster is active.
// POST stores only the 4x factor for max_scale_factor 4, so the cluster cannot
// autoscale through 2x. PATCH stores both the 2x and 4x factors.
// TODO: send autoScale on create when POST /v2/clusters uses the same factors as PATCH.
func createDefersAutoScale(plan clusterResourceModel) bool {
	return plan.AutoScale != nil && plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX4))
}

// applyAutoScaleAfterCreate sets the configured autoscale on a newly active cluster
// when createDefersAutoScale left it out of POST. It reports whether that setup
// succeeded. A failure is a warning, and the returned cluster keeps the autoscale
// the API reports. Create stores the planned autoscale in that case so the apply
// result matches the plan.
func applyAutoScaleAfterCreate(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, cluster management.Cluster, plan clusterResourceModel, diags *diag.Diagnostics) (management.Cluster, bool) {
	warn := func(err *util.SummaryWithDetailError) (management.Cluster, bool) {
		diags.AddWarning(
			fmt.Sprintf("Cluster %s was created without the configured auto_scale", id),
			fmt.Sprintf("Setting auto_scale after create failed. Run terraform apply again to set it. %s: %s", err.Summary, err.Detail),
		)

		return cluster, false
	}

	if !createDefersAutoScale(plan) {
		return cluster, true
	}

	patchResponse, err := c.PatchV2ClustersClusterIDWithResponse(ctx, id, management.Cluster{
		Name:      cluster.Name,
		ProjectID: cluster.ProjectID,
		AutoScale: toAutoScale(plan),
	})
	if serr := util.StatusOK(patchResponse, err); serr != nil {
		return warn(serr)
	}

	updated, werr := wait(ctx, c, id, config.ClusterUpdateTimeout,
		waitConditionState(management.ClusterStateACTIVE),
		waitConditionAutoScale(plan.AutoScale.MaxScaleFactor.ValueFloat32()),
	)
	if werr != nil {
		return warn(werr)
	}

	return updated, true
}

// Read refreshes the Terraform state with the latest data.
func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state clusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, perr := parseClusterID(state.ID.ValueString())
	if perr != nil {
		resp.Diagnostics.AddAttributeError(path.Root(config.IDAttribute), perr.Summary, perr.Detail)

		return
	}

	cluster, err := r.GetV2ClustersClusterIDWithResponse(ctx, id, &management.GetV2ClustersClusterIDParams{})
	if serr := util.StatusOK(cluster, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	if cluster.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)

		return // The resource got deleted externally, deleting it from the state file to recreate.
	}

	stateValue := util.Deref(cluster.JSON200.State)
	if isTerminatedCluster(*cluster.JSON200) {
		// A cluster terminated before its endpoint became ready is still reported as
		// FAILED or PENDING, with terminatedAt set.
		resp.State.RemoveResource(ctx)

		return
	}

	resp.Diagnostics.Append(clusterReadStateDiagnostics(state.ID.ValueString(), stateValue)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.setRefreshedClusterState(ctx, resp, state, *cluster.JSON200, stateValue)
}

// setRefreshedClusterState copies the Management API cluster into state.
// A transitional state keeps the previous suspended value so refresh does not
// plan a suspend or resume.
func (r *clusterResource) setRefreshedClusterState(ctx context.Context, resp *resource.ReadResponse, state clusterResourceModel, cluster management.Cluster, stateValue management.ClusterState) {
	projectName := state.ProjectName
	if !util.IsConfiguredString(projectName) {
		resolved, rerr := resolveProjectNameByID(ctx, r.ClientWithResponsesInterface, cluster.ProjectID)
		if rerr != nil {
			resp.Diagnostics.AddError(rerr.Summary, rerr.Detail)

			return
		}
		projectName = resolved
	}

	configuredExpiresAt := state.ExpiresAt
	previousSuspended := state.Suspended
	state, modelDiags := toClusterResourceModel(cluster, state.AdminPassword.ValueString(), state.FirewallRanges, projectName, configuredExpiresAt)
	resp.Diagnostics.Append(modelDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !isSettledClusterState(stateValue) {
		// PENDING also covers suspend, resume, and scale. Deriving suspended from a
		// transitional state would plan a suspend or resume nobody asked for.
		state.Suspended = types.BoolValue(previousSuspended.ValueBool())
	}
	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// isSettledClusterState reports whether the cluster is in a state that accepts
// suspend, resume, and configuration changes.
func isSettledClusterState(stateValue management.ClusterState) bool {
	return stateValue == management.ClusterStateACTIVE || stateValue == management.ClusterStateSUSPENDED
}

// clusterReadStateDiagnostics keeps clusters in any state other than TERMINATED
// in state. A failed or timed-out create is already stored and tainted, and refresh
// has to succeed so the next apply can replace or destroy that cluster.
func clusterReadStateDiagnostics(clusterID string, stateValue management.ClusterState) diag.Diagnostics {
	var diags diag.Diagnostics

	switch stateValue {
	case management.ClusterStateACTIVE, management.ClusterStateSUSPENDED:
	case management.ClusterStatePENDING:
		diags.AddWarning(
			fmt.Sprintf("Cluster %s state is %s", clusterID, stateValue),
			"The cluster is changing state, or it is offline or impacted: Management API reports all of these as PENDING. "+
				"Updates wait until it is ACTIVE or SUSPENDED. terraform destroy does not wait. "+
				"If the cluster stays PENDING, check its status in the SingleStore Portal.",
		)
	case management.ClusterStateFAILED:
		diags.AddWarning(
			fmt.Sprintf("Cluster %s state is %s", clusterID, stateValue),
			"Terraform is still tracking this cluster so it can be replaced or destroyed. "+
				"A cluster that failed during create is replaced on the next apply. Otherwise, run "+
				"terraform apply -replace=ADDRESS to replace it, or terraform destroy to remove it. "+
				config.ContactSupportLaterErrorDetail,
		)
	case management.ClusterStateTERMINATED:
		// Read removes a terminated cluster from state before calling this.
	case management.ClusterStateUNKNOWN:
		fallthrough
	default:
		if stateValue == "" {
			stateValue = "missing"
		}
		diags.AddWarning(
			fmt.Sprintf("Cluster %s state is %s", clusterID, stateValue),
			fmt.Sprintf("An unexpected cluster state; it should be %s or %s. ", management.ClusterStateACTIVE, management.ClusterStateSUSPENDED)+
				"Terraform is still tracking this cluster so it can be replaced or destroyed. "+
				config.ContactSupportLaterErrorDetail,
		)
	}

	return diags
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *clusterResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state clusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan clusterResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, uerr := applyClusterConfigOrToggleSuspension(ctx, r.ClientWithResponsesInterface, state, plan)
	if uerr != nil {
		resp.Diagnostics.AddError(uerr.Summary, uerr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *clusterResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state clusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, perr := parseClusterID(state.ID.ValueString())
	if perr != nil {
		resp.Diagnostics.AddAttributeError(path.Root(config.IDAttribute), perr.Summary, perr.Detail)

		return
	}

	clusterDeleteResponse, err := r.DeleteV2ClustersClusterIDWithResponse(ctx, id)
	if serr := util.StatusOK(clusterDeleteResponse, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	// A replacement is created right after Delete returns. Wait for termination so
	// it does not overlap with the old cluster.
	if werr := waitTerminated(ctx, r.ClientWithResponsesInterface, id, config.ClusterDeletionTimeout); werr != nil {
		resp.Diagnostics.AddError(werr.Summary, werr.Detail)
	}
}

// parseClusterID parses a cluster ID from state without panicking on a bad value,
// for example after a manual state edit.
func parseClusterID(value string) (uuid.UUID, *util.SummaryWithDetailError) {
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.UUID{}, &util.SummaryWithDetailError{
			Summary: "Invalid cluster ID",
			Detail:  fmt.Sprintf("The cluster ID %q is not a valid UUID: %s", value, err),
		}
	}

	return id, nil
}

// Configure adds the provider configured client to the resource.
func (r *clusterResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
func (r *clusterResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) { //nolint:cyclop
	var state *clusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan *clusterResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	// Create has no prior state. Run the checks here so plan fails before apply.
	// ValueBool is false for null and unknown, so an omitted or not-yet-known
	// suspended value is left to Create.
	if state == nil {
		if plan.Suspended.ValueBool() {
			resp.Diagnostics.AddAttributeError(
				path.Root("suspended"),
				"Cannot suspend a cluster during creation",
				"Either set value to false or omit the field.",
			)

			return
		}

		// An omitted project_name is null in config and unknown in the plan.
		var configProjectName types.String
		diags = req.Config.GetAttribute(ctx, path.Root("project_name"), &configProjectName)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		if err := validatePlannedCreate(plan, configProjectName); err != nil {
			resp.Diagnostics.AddError(err.Summary, err.Detail)
		}

		return
	}

	if !plan.Name.Equal(state.Name) {
		resp.Diagnostics.AddError("Cannot update cluster name",
			"Updating the name is not permitted. "+
				"Current value: \""+state.Name.ValueString()+"\", configured value: \""+plan.Name.ValueString()+"\".")

		return
	}

	// UseStateForUnknown skips a null state, so an unconfigured project_name the API
	// did not return stays unknown and would look like a change. Keep it null.
	for _, a := range []struct {
		name  string
		plan  *types.String
		state types.String
	}{
		{"project_name", &plan.ProjectName, state.ProjectName},
	} {
		if a.plan.IsUnknown() && a.state.IsNull() {
			*a.plan = types.StringNull()
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(a.name), types.StringNull())...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	if err := validateModifyProjectName(plan, state); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	if err := validateModifyExpiresAt(plan, state); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("expires_at"), err.Summary, err.Detail)

		return
	}

	if !plan.CloudProvider.Equal(state.CloudProvider) {
		resp.Diagnostics.AddError("Cannot update cluster cloud_provider",
			fmt.Sprintf("Updating the cloud_provider is not permitted. Expected value is '%s'.", state.CloudProvider.ValueString()))

		return
	}

	if !plan.RegionName.Equal(state.RegionName) {
		resp.Diagnostics.AddError("Cannot update cluster region_name",
			fmt.Sprintf("Updating the region_name is not permitted. Expected value is '%s', but now is '%s'.", state.RegionName.ValueString(), plan.RegionName.ValueString()))

		return
	}

	if !plan.Kai.Equal(state.Kai) {
		resp.Diagnostics.AddError("Cannot change the kai configuration for the cluster",
			"Changing the kai configuration is currently not supported. "+
				"Current value: "+state.Kai.String()+", configured value: "+plan.Kai.String()+".")

		return
	}

	if !plan.MultiAZ.Equal(state.MultiAZ) {
		resp.Diagnostics.AddError("Cannot change the multi_az configuration for the cluster",
			"Changing the multi_az configuration is currently not supported. "+
				"Current value: "+state.MultiAZ.String()+", configured value: "+plan.MultiAZ.String()+".")

		return
	}

	if !plan.OptInPreviewFeature.Equal(state.OptInPreviewFeature) {
		resp.Diagnostics.AddError("Cannot change the opt_in_preview_feature configuration for the cluster",
			"Changing the opt_in_preview_feature configuration is currently not supported. "+
				"Current value: "+state.OptInPreviewFeature.String()+", configured value: "+plan.OptInPreviewFeature.String()+".")

		return
	}

	if state.OptInPreviewFeature.ValueBool() && plan.DeploymentType.ValueString() != string(management.ClusterDeploymentTypeNONPRODUCTION) {
		resp.Diagnostics.AddError(
			"Cannot change the deployment_type configuration to anything other than 'NON-PRODUCTION' for the cluster when the opt_in_preview_feature is enabled.",
			"Changing the deployment_type configuration to anything other than 'NON-PRODUCTION' when the opt_in_preview_feature is enabled is not currently supported. "+
				"Current value: \""+state.DeploymentType.ValueString()+"\", configured value: \""+plan.DeploymentType.ValueString()+"\".",
		)

		return
	}

	if err := validateSuspendedAndConfigChanges(state, plan); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *clusterResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	util.ImportStatePassthroughID(ctx, req, resp)
}

// validatePlannedCreate reports create-time errors whose inputs are already known.
// Unknown inputs are replaced with stand-ins that pass validateCreateParameters;
// Create repeats the checks once those values are known. The stand-ins are not
// sent to the API.
func validatePlannedCreate(plan *clusterResourceModel, configProjectName types.String) *util.SummaryWithDetailError {
	checked := deferUnknownCreateInputs(*plan, configProjectName)

	return validateCreateParameters(&checked)
}

// deferUnknownCreateInputs replaces unknown create inputs with values that pass
// validation. Create checks the real values once they are known.
func deferUnknownCreateInputs(plan clusterResourceModel, configProjectName types.String) clusterResourceModel {
	if configProjectName.IsNull() {
		plan.ProjectName = types.StringNull()
	} else if plan.ProjectName.IsUnknown() {
		plan.ProjectName = types.StringValue("pending")
	}

	if plan.CloudProvider.IsUnknown() {
		plan.CloudProvider = types.StringValue(string(management.CloudProviderAWS))
	}
	if plan.RegionName.IsUnknown() {
		plan.RegionName = types.StringValue("us-east-1")
	}
	// ValueBool is false while opt-in is unknown. A known true paired with an
	// unknown deployment type would otherwise look like a non-production mismatch.
	if plan.OptInPreviewFeature.ValueBool() && plan.DeploymentType.IsUnknown() {
		plan.OptInPreviewFeature = types.BoolValue(false)
	}

	return plan
}

// validateCreateParameters reports configuration that POST /v2/clusters would reject.
func validateCreateParameters(plan *clusterResourceModel) *util.SummaryWithDetailError {
	if !util.IsConfiguredString(plan.ProjectName) {
		return &util.SummaryWithDetailError{
			Summary: "Missing project_name",
			Detail:  "The project_name attribute is required when creating a cluster. Use the singlestoredb_projects data source or check https://docs.singlestore.com/cloud/getting-started-with-singlestore-helios/about-singlestore-helios/projects/ for more information.",
		}
	}

	if !util.IsConfiguredString(plan.CloudProvider) || !util.IsConfiguredString(plan.RegionName) {
		return &util.SummaryWithDetailError{
			Summary: "Invalid region configuration",
			Detail:  "Both cloud_provider and region_name are required. POST /v2/clusters rejects a cluster that omits either one.",
		}
	}

	if plan.OptInPreviewFeature.ValueBool() && plan.DeploymentType.ValueString() != string(management.ClusterDeploymentTypeNONPRODUCTION) {
		return &util.SummaryWithDetailError{
			Summary: "Wrong configuration for opt_in_preview_feature and deployment_type",
			Detail:  "The enabled opt_in_preview_feature configuration is suitable only for the 'NON-PRODUCTION' deployment_type.",
		}
	}

	if err := validateAutoSuspendConfig(plan); err != nil {
		return err
	}

	if err := validateAutoScaleConfig(plan); err != nil {
		return err
	}

	return nil
}

// validateModifyProjectName rejects a change to project_name. The Management API
// does not move a cluster between projects.
func validateModifyProjectName(plan, state *clusterResourceModel) *util.SummaryWithDetailError {
	if plan.ProjectName.IsNull() {
		return nil
	}

	if state.ProjectName.IsNull() {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update cluster project_name",
			Detail:  fmt.Sprintf("Updating the project_name is not permitted. Expected value is unset/null, but configured value is %s.", plan.ProjectName.String()),
		}
	}

	if !plan.ProjectName.Equal(state.ProjectName) {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update cluster project_name",
			Detail:  fmt.Sprintf("Updating the project_name is not permitted. Expected value is %s, but configured value is %s.", state.ProjectName.String(), plan.ProjectName.String()),
		}
	}

	return nil
}

// TODO: allow expires_at to change when PATCH /v2/clusters applies expiresAt.
// applyClusterConfiguration already sends a changed expiresAt.
func validateModifyExpiresAt(plan, state *clusterResourceModel) *util.SummaryWithDetailError {
	// Compare instants. Import stores the normalized API timestamp, so the config
	// may use another RFC3339 spelling of that same time.
	if plan.ExpiresAt.IsUnknown() || state.ExpiresAt.IsUnknown() || util.SameTimestamp(plan.ExpiresAt, state.ExpiresAt) {
		return nil
	}

	current := "unset"
	if util.IsConfiguredString(state.ExpiresAt) {
		current = state.ExpiresAt.ValueString()
	}
	configured := "unset"
	if util.IsConfiguredString(plan.ExpiresAt) {
		configured = plan.ExpiresAt.ValueString()
	}

	return &util.SummaryWithDetailError{
		Summary: "Cannot change cluster expires_at",
		Detail: fmt.Sprintf("The Management API applies expiresAt only when the cluster is created. "+
			"Current value is %s, configured value is %s. "+
			"Keep expires_at equal to the current value, including after import.", current, configured),
	}
}

// validateAutoSuspendConfig rejects suspend_after_seconds when suspend_type is DISABLED.
func validateAutoSuspendConfig(plan *clusterResourceModel) *util.SummaryWithDetailError {
	// A value only known at apply time may still satisfy the rule.
	if plan.AutoSuspend.SuspendType.IsUnknown() || plan.AutoSuspend.SuspendAfterSeconds.IsUnknown() {
		return nil
	}

	if plan.AutoSuspend.SuspendType.Equal(types.StringValue(string(management.AutoSuspendSuspendTypeDISABLED))) &&
		!plan.AutoSuspend.SuspendAfterSeconds.IsNull() {
		return &util.SummaryWithDetailError{
			Summary: "Invalid auto_suspend configuration.",
			Detail:  "If suspend_type is set to DISABLED, the suspend_after_seconds parameter is not allowed.",
		}
	}

	return nil
}

// validateAutoScaleConfig rejects a non-default sensitivity when max_scale_factor is 1.
func validateAutoScaleConfig(plan *clusterResourceModel) *util.SummaryWithDetailError {
	// A value only known at apply time may still satisfy the rule.
	if plan.AutoScale.Sensitivity.IsUnknown() || plan.AutoScale.MaxScaleFactor.IsUnknown() {
		return nil
	}

	if !plan.AutoScale.Sensitivity.Equal(types.StringValue(string(management.NORMAL))) &&
		plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) {
		return &util.SummaryWithDetailError{
			Summary: "Invalid auto_scale configuration.",
			Detail:  "If max_scale_factor is set to 1, the sensitivity parameter (if not set to its default value) is not allowed.",
		}
	}

	return nil
}

// validateSuspendedAndConfigChanges rejects changing suspended together with other
// configuration, and rejects configuration changes while the cluster stays suspended.
func validateSuspendedAndConfigChanges(state, plan *clusterResourceModel) *util.SummaryWithDetailError {
	if err := validateAutoScaleConfig(plan); err != nil {
		return err
	}

	if err := validateAutoSuspendConfig(plan); err != nil {
		return err
	}

	suspendedChanged := !plan.Suspended.Equal(state.Suspended)
	isSuspended := plan.Suspended.ValueBool()
	otherConfigChanged := hasGeneralConfigChanged(*state, *plan)

	if otherConfigChanged && suspendedChanged {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update both the suspension state and other configurations at the same time",
			Detail:  "To avoid an inconsistent state, either suspend the cluster or update the other configurations.",
		}
	}

	if otherConfigChanged && isSuspended {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update the configuration for a suspended cluster.",
			Detail:  "Resume the cluster by setting suspended to false before updating the configuration.",
		}
	}

	return nil
}

// toClusterResourceModel copies a Management API cluster into the resource model.
// adminPassword is the last password read from the API. The cluster object
// replaces it when that response includes adminPassword.
func toClusterResourceModel(cluster management.Cluster, adminPassword string, configuredFirewallRanges []types.String, projectName, configuredExpiresAt types.String) (clusterResourceModel, diag.Diagnostics) {
	size := types.StringNull()
	scaleFactor := types.Float32Value(scaleX1)
	cacheConfig := types.Float32Value(cacheMultiplierX1)
	if cluster.SizeConfig != nil {
		size = util.MaybeStringValue(cluster.SizeConfig.Size)
		if cluster.SizeConfig.ScaleFactor != nil {
			scaleFactor = types.Float32Value(*cluster.SizeConfig.ScaleFactor)
		}
		if cluster.SizeConfig.CacheConfig != nil {
			cacheConfig = types.Float32Value(*cluster.SizeConfig.CacheConfig)
		}
	}

	updateWindow, diags := toUpdateWindowResourceModel(cluster.UpdateWindow)
	model := clusterResourceModel{
		ID:                  util.MaybeUUIDStringValue(cluster.ClusterID),
		Name:                types.StringValue(cluster.Name),
		ProjectName:         projectName,
		GroupID:             util.MaybeUUIDStringValue(cluster.GroupID),
		Size:                size,
		ScaleFactor:         scaleFactor,
		CacheConfig:         cacheConfig,
		Suspended:           types.BoolValue(util.Deref(cluster.State) == management.ClusterStateSUSPENDED),
		Kai:                 types.BoolValue(util.Deref(cluster.Kai)),
		AdminPassword:       passwordFromAPI(cluster.AdminPassword, adminPassword),
		FirewallRanges:      firewallRangesForState(configuredFirewallRanges, cluster),
		ExpiresAt:           util.ExpiresAtStateValue(cluster.ExpiresAt, configuredExpiresAt),
		CloudProvider:       normalizeCloudProvider(cluster.Provider),
		RegionName:          util.MaybeStringValue(cluster.Region),
		DeploymentType:      deploymentTypeForState(cluster.DeploymentType),
		OptInPreviewFeature: types.BoolValue(util.Deref(cluster.OptInPreviewFeature)),
		MultiAZ:             types.BoolValue(util.Deref(cluster.MultiAZ)),
		OutboundAllowList:   util.MaybeStringValue(cluster.OutboundAllowList),
		Endpoint:            util.MaybeStringValue(cluster.Endpoint),
		CreatedAt:           util.MaybeTimeValue(cluster.CreatedAt),
		AutoScale:           toAutoScaleResourceModel(cluster),
		AutoSuspend:         toAutoSuspendResourceModel(cluster),
		UpdateWindow:        updateWindow,
	}

	return model, diags
}

// passwordFromAPI stores the admin password Management API returned.
// A response that omits the password keeps the last password read from the API,
// so a later GET that leaves the field out does not clear state.
func passwordFromAPI(apiPassword *string, knownPassword string) types.String {
	if apiPassword != nil && *apiPassword != "" {
		return types.StringValue(*apiPassword)
	}
	if knownPassword == "" {
		return types.StringNull()
	}

	return types.StringValue(knownPassword)
}

// deploymentTypeForState uses the schema default when the API omits the field.
// A null value disagrees with the plan and keeps proposing updates.
func deploymentTypeForState(deploymentType *management.ClusterDeploymentType) types.String {
	if deploymentType == nil {
		return types.StringValue(string(management.ClusterDeploymentTypePRODUCTION))
	}

	return types.StringValue(string(*deploymentType))
}

// clusterStateAfterCreate copies the plan into a state value Terraform can store
// before the cluster is active. Unknown planned values cannot be stored.
func clusterStateAfterCreate(plan clusterResourceModel, clusterID, groupID uuid.UUID, adminPassword string) *clusterResourceModel {
	tracked := plan
	if plan.AutoScale != nil {
		autoScale := *plan.AutoScale
		tracked.AutoScale = &autoScale
	}
	if plan.AutoSuspend != nil {
		autoSuspend := *plan.AutoSuspend
		tracked.AutoSuspend = &autoSuspend
	}
	if plan.FirewallRanges != nil {
		tracked.FirewallRanges = append([]types.String(nil), plan.FirewallRanges...)
	}

	tracked.ID = types.StringValue(clusterID.String())
	tracked.GroupID = types.StringValue(groupID.String())
	tracked.AdminPassword = passwordFromAPI(nil, adminPassword)
	nullUnknownClusterState(&tracked)

	return &tracked
}

// nullUnknownClusterState replaces unknown attributes with null. Terraform cannot
// store unknown values in state.
func nullUnknownClusterState(model *clusterResourceModel) {
	model.ID = nullUnknownString(model.ID)
	model.Name = nullUnknownString(model.Name)
	model.ProjectName = nullUnknownString(model.ProjectName)
	model.GroupID = nullUnknownString(model.GroupID)
	model.Size = nullUnknownString(model.Size)
	model.AdminPassword = nullUnknownString(model.AdminPassword)
	model.ExpiresAt = nullUnknownString(model.ExpiresAt)
	model.CloudProvider = nullUnknownString(model.CloudProvider)
	model.RegionName = nullUnknownString(model.RegionName)
	model.DeploymentType = nullUnknownString(model.DeploymentType)
	model.OutboundAllowList = nullUnknownString(model.OutboundAllowList)
	model.Endpoint = nullUnknownString(model.Endpoint)
	model.CreatedAt = nullUnknownString(model.CreatedAt)
	model.ScaleFactor = nullUnknownFloat32(model.ScaleFactor)
	model.CacheConfig = nullUnknownFloat32(model.CacheConfig)
	model.Suspended = nullUnknownBool(model.Suspended)
	model.Kai = nullUnknownBool(model.Kai)
	model.OptInPreviewFeature = nullUnknownBool(model.OptInPreviewFeature)
	model.MultiAZ = nullUnknownBool(model.MultiAZ)
	for i, firewallRange := range model.FirewallRanges {
		model.FirewallRanges[i] = nullUnknownString(firewallRange)
	}
	if model.AutoScale != nil {
		model.AutoScale.MaxScaleFactor = nullUnknownFloat32(model.AutoScale.MaxScaleFactor)
		model.AutoScale.Sensitivity = nullUnknownString(model.AutoScale.Sensitivity)
	}
	if model.AutoSuspend != nil {
		model.AutoSuspend.SuspendAfterSeconds = nullUnknownInt64(model.AutoSuspend.SuspendAfterSeconds)
		model.AutoSuspend.SuspendType = nullUnknownString(model.AutoSuspend.SuspendType)
	}
	if model.UpdateWindow.IsNull() || model.UpdateWindow.IsUnknown() {
		model.UpdateWindow = types.ObjectNull(updateWindowAttrTypes)
	}
}

// nullUnknownString returns null when value is unknown.
func nullUnknownString(value types.String) types.String {
	if value.IsUnknown() {
		return types.StringNull()
	}

	return value
}

// nullUnknownBool returns null when value is unknown.
func nullUnknownBool(value types.Bool) types.Bool {
	if value.IsUnknown() {
		return types.BoolNull()
	}

	return value
}

// nullUnknownFloat32 returns null when value is unknown.
func nullUnknownFloat32(value types.Float32) types.Float32 {
	if value.IsUnknown() {
		return types.Float32Null()
	}

	return value
}

// nullUnknownInt64 returns null when value is unknown.
func nullUnknownInt64(value types.Int64) types.Int64 {
	if value.IsUnknown() {
		return types.Int64Null()
	}

	return value
}

// normalizeCloudProvider maps the Management API provider onto the schema's values.
func normalizeCloudProvider(provider *management.CloudProvider) types.String {
	if provider == nil {
		return types.StringNull()
	}

	result := util.WorkspaceGroupCloudProviderString(string(*provider))
	if result == nil {
		return types.StringValue(string(*provider))
	}

	return types.StringValue(string(*result))
}

// toCreateAutoScale is the autoscale sent to POST /v2/clusters. Factor 1 is omitted,
// and factor 4 is deferred to PATCH because POST stores only the 4x step.
func toCreateAutoScale(plan clusterResourceModel) *management.AutoScale {
	if plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) || createDefersAutoScale(plan) {
		return nil
	}

	return &management.AutoScale{
		MaxScaleFactor: util.MaybeFloat32(plan.AutoScale.MaxScaleFactor),
		Sensitivity:    util.WorkspaceAutoScaleSensitivityString(plan.AutoScale.Sensitivity),
	}
}

// toAutoScale is the autoscale sent to PATCH /v2/clusters. Factor 1 omits sensitivity.
func toAutoScale(plan clusterResourceModel) *management.AutoScale {
	var sensitivity *management.AutoScaleSensitivity
	if !plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) {
		sensitivity = util.WorkspaceAutoScaleSensitivityString(plan.AutoScale.Sensitivity)
	}

	return &management.AutoScale{
		MaxScaleFactor: util.MaybeFloat32(plan.AutoScale.MaxScaleFactor),
		Sensitivity:    sensitivity,
	}
}

// toAutoScaleResourceModel copies the Management API autoscale into state. Omitted
// fields use the schema defaults so a scale-down to 1 does not stay a perpetual diff.
func toAutoScaleResourceModel(cluster management.Cluster) *autoScaleResourceModel {
	// Omitted API fields use the schema defaults. A null or unknown value here
	// disagrees with the plan and keeps proposing updates after a scale-down to 1,
	// which sends auto_scale without sensitivity.
	model := &autoScaleResourceModel{
		MaxScaleFactor: types.Float32Value(scaleX1),
		Sensitivity:    types.StringValue(string(management.NORMAL)),
	}
	if cluster.AutoScale == nil {
		return model
	}
	if cluster.AutoScale.MaxScaleFactor != nil {
		model.MaxScaleFactor = types.Float32Value(*cluster.AutoScale.MaxScaleFactor)
	}
	if cluster.AutoScale.Sensitivity != nil {
		model.Sensitivity = types.StringValue(string(*cluster.AutoScale.Sensitivity))
	}

	return model
}

// toAutoSuspend maps the plan onto the Management API autoSuspend object, placing
// the delay on the field the suspend type uses.
func toAutoSuspend(plan clusterResourceModel) *management.AutoSuspend {
	suspendType := util.AutoSuspendSuspendTypeString(plan.AutoSuspend.SuspendType)
	result := &management.AutoSuspend{
		SuspendType: suspendType,
	}

	seconds := int64ToIntPtr(plan.AutoSuspend.SuspendAfterSeconds)
	if suspendType != nil {
		switch *suspendType {
		case management.AutoSuspendSuspendTypeIDLE:
			result.IdleAfterSeconds = seconds
		case management.AutoSuspendSuspendTypeSCHEDULED:
			result.ScheduledAfterSeconds = seconds
		case management.AutoSuspendSuspendTypeDISABLED:
		}
	}

	return result
}

// toAutoSuspendResourceModel copies the Management API auto suspend settings into state.
func toAutoSuspendResourceModel(cluster management.Cluster) *autoSuspendResourceModel {
	if cluster.AutoSuspend == nil || cluster.AutoSuspend.SuspendType == nil {
		// A zero Int64 is unknown. The plan default for suspend_after_seconds is null.
		return &autoSuspendResourceModel{
			SuspendAfterSeconds: types.Int64Null(),
			SuspendType:         types.StringValue(string(management.AutoSuspendSuspendTypeDISABLED)),
		}
	}

	var suspendAfterSeconds *int
	switch *cluster.AutoSuspend.SuspendType {
	case management.AutoSuspendSuspendTypeIDLE:
		suspendAfterSeconds = cluster.AutoSuspend.IdleAfterSeconds
	case management.AutoSuspendSuspendTypeSCHEDULED:
		suspendAfterSeconds = cluster.AutoSuspend.ScheduledAfterSeconds
	case management.AutoSuspendSuspendTypeDISABLED:
	}

	return &autoSuspendResourceModel{
		SuspendAfterSeconds: intPtrToInt64(suspendAfterSeconds),
		SuspendType:         util.StringValueOrNull(cluster.AutoSuspend.SuspendType),
	}
}

// int64ToIntPtr converts the Terraform value into the integer the Management API stores.
func int64ToIntPtr(v types.Int64) *int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}

	n := int(v.ValueInt64())

	return &n
}

// intPtrToInt64 converts an API integer into the Terraform int64 attribute.
func intPtrToInt64(i *int) types.Int64 {
	if i == nil {
		return types.Int64Null()
	}

	return types.Int64Value(int64(*i))
}

// toManagementUpdateWindow converts the planned update window into the Management API object.
func toManagementUpdateWindow(ctx context.Context, uw types.Object) (*management.UpdateWindow, diag.Diagnostics) {
	if uw.IsNull() || uw.IsUnknown() {
		return nil, nil
	}

	var model updateWindowResourceModel
	diags := uw.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &management.UpdateWindow{
		Hour: float32(model.Hour.ValueInt64()),
		Day:  float32(model.Day.ValueInt64()),
	}, diags
}

// toUpdateWindowResourceModel converts the Management API update window into state.
func toUpdateWindowResourceModel(uw *management.UpdateWindow) (types.Object, diag.Diagnostics) {
	if uw == nil {
		return types.ObjectNull(updateWindowAttrTypes), nil
	}

	obj, diags := types.ObjectValue(
		updateWindowAttrTypes,
		map[string]attr.Value{
			"hour": types.Int64Value(int64(uw.Hour)),
			"day":  types.Int64Value(int64(uw.Day)),
		},
	)
	if diags.HasError() {
		return types.ObjectNull(updateWindowAttrTypes), diags
	}

	return obj, diags
}

// errorFromDiagnostics returns the first diagnostic error, with every error detail joined.
func errorFromDiagnostics(diags diag.Diagnostics) *util.SummaryWithDetailError {
	if !diags.HasError() {
		return nil
	}

	errs := diags.Errors()
	details := make([]string, len(errs))
	for i, d := range errs {
		details[i] = d.Detail()
		if details[i] == "" {
			details[i] = d.Summary()
		}
	}

	return &util.SummaryWithDetailError{
		Summary: errs[0].Summary(),
		Detail:  strings.Join(details, "\n"),
	}
}

// resolveProjectNameByID finds the project name for a Management API project ID.
func resolveProjectNameByID(ctx context.Context, c management.ClientWithResponsesInterface, projectID uuid.UUID) (types.String, *util.SummaryWithDetailError) {
	projectsResponse, err := c.GetV2ProjectsWithResponse(ctx)
	if serr := util.StatusOK(projectsResponse, err); serr != nil {
		return types.StringNull(), serr
	}

	for _, project := range util.Deref(projectsResponse.JSON200) {
		if project.ProjectID == projectID {
			return types.StringValue(project.Name), nil
		}
	}

	return types.StringNull(), nil
}
