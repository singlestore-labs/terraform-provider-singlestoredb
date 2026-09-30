package clusters

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/float32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
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
	SuspendAfterSeconds types.Float32 `tfsdk:"suspend_after_seconds"`
	SuspendType         types.String  `tfsdk:"suspend_type"`
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
			"suspend_after_seconds": basetypes.Float32Type{},
			"suspend_type":          basetypes.StringType{},
		},
		map[string]attr.Value{
			"suspend_after_seconds": basetypes.NewFloat32Null(),
			"suspend_type":          basetypes.NewStringValue(string(management.DISABLED)),
		},
	)
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage a SingleStoreDB cluster (workspace + workspace group created in one call) with this resource.",
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
				Computed:            true,
				MarkdownDescription: "The unique identifier of the workspace group that contains the cluster.",
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
				MarkdownDescription: "The status of the cluster. If true, the cluster is suspended.",
				Default:             booldefault.StaticBool(false),
			},
			"kai": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether SingleStore Kai (MongoDB compatibility) is enabled for the cluster.",
				Default:             booldefault.StaticBool(false),
			},
			"admin_password": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: `The admin SQL user password for the cluster. If not provided, the server will automatically generate a secure password. Must be at least 14 characters long.`,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(config.AdminPasswordMinLength),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"firewall_ranges": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "List of allowed CIDR ranges. An empty list blocks all inbound requests. For unrestricted traffic, use [\"0.0.0.0/0\"]. Note that updates to firewall ranges may take a brief moment to become effective.",
			},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: `The expiration timestamp of the cluster. If not specified, the cluster never expires. Upon expiration, the cluster is terminated and all its data is lost. Set the expiration time as an RFC3339 UTC timestamp, e.g., "2221-01-02T15:04:05Z".`,
				Validators:          []validator.String{util.NewTimeValidator()},
			},
			"cloud_provider": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The name of the cloud provider used to resolve region. Possible values are 'AWS', 'GCP', and 'Azure'.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.CloudProviderAWS), string(management.CloudProviderGCP), string(management.CloudProviderAzure)),
				},
			},
			"region_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The region code name used to resolve region.",
			},
			"deployment_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The deployment type of the cluster. It can have one of the following values: `PRODUCTION` or `NON-PRODUCTION`. The default value is `PRODUCTION`.",
				Default:             stringdefault.StaticString(string(management.PRODUCTION)),
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.PRODUCTION), string(management.NONPRODUCTION)),
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
						Optional:            true,
						Computed:            true,
						Default:             float32default.StaticFloat32(scaleX1),
						Validators:          []validator.Float32{float32validator.OneOf(scaleX1, scaleX2, scaleX4)},
						MarkdownDescription: "The maximum scale factor allowed for the cluster. It can have the following values: 1, 2, or 4. To disable autoscaling, set to 1. Default is 1.",
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
					"suspend_after_seconds": schema.Float32Attribute{
						Optional:            true,
						MarkdownDescription: "When to suspend the cluster, according to the suspend type chosen.",
					},
					"suspend_type": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "The auto suspend mode for the cluster can have the values `IDLE`, `SCHEDULED`, or `DISABLED`. Default is `DISABLED`.",
						Default:             stringdefault.StaticString(string(management.DISABLED)),
						Validators: []validator.String{
							stringvalidator.OneOf(string(management.DISABLED), string(management.IDLE), string(management.SCHEDULED)),
						},
					},
				},
			},
			"update_window": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Details of the scheduled update window for the cluster. This is the time period during which any updates to the cluster will occur.",
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

	projectID, err := resolveProjectIDByName(ctx, r.ClientWithResponsesInterface, plan.ProjectName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	firewallRanges := util.StringFirewallRanges(plan.FirewallRanges)
	createBody := management.Cluster{
		AdminPassword:       util.MaybeNonEmptyString(plan.AdminPassword),
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
		UpdateWindow: toManagementUpdateWindow(ctx, plan.UpdateWindow),
	}

	clusterCreateResponse, cerr := r.PostV2ClustersWithResponse(ctx, createBody)
	if serr := util.StatusOK(clusterCreateResponse, cerr); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	clusterID := clusterCreateResponse.JSON200.ClusterID
	cluster, werr := wait(ctx, r.ClientWithResponsesInterface, clusterID, config.WorkspaceCreationTimeout,
		waitConditionState(management.ClusterStateACTIVE),
		waitConditionFirewallRanges(plan.FirewallRanges),
	)
	if werr != nil {
		resp.Diagnostics.AddError(werr.Summary, werr.Detail)

		return
	}

	result := toClusterResourceModel(cluster, util.AdminPasswordForState(
		plan.AdminPassword.ValueString(),
		util.Deref(clusterCreateResponse.JSON200.AdminPassword),
	), plan.FirewallRanges, plan.ProjectName)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *clusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state clusterResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(state.ID.ValueString())

	cluster, err := r.GetV2ClustersClusterIDWithResponse(ctx, id, &management.GetV2ClustersClusterIDParams{})
	if serr := util.StatusOK(cluster, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	stateValue := util.Deref(cluster.JSON200.State)
	if stateValue == management.ClusterStateTERMINATED {
		resp.State.RemoveResource(ctx)

		return
	}

	if stateValue != management.ClusterStateACTIVE && stateValue != management.ClusterStateSUSPENDED {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Cluster %s state is %s while it should be %s or %s", state.ID.ValueString(), stateValue, management.ClusterStateACTIVE, management.ClusterStateSUSPENDED),
			"An unexpected cluster state.\n\n"+
				config.ContactSupportLaterErrorDetail,
		)

		return
	}

	projectName := state.ProjectName
	if !util.IsConfiguredString(projectName) {
		resolved, rerr := resolveProjectNameByID(ctx, r.ClientWithResponsesInterface, cluster.JSON200.ProjectID)
		if rerr != nil {
			resp.Diagnostics.AddError(rerr.Summary, rerr.Detail)

			return
		}
		projectName = resolved
	}

	state = toClusterResourceModel(*cluster.JSON200, state.AdminPassword.ValueString(), state.FirewallRanges, projectName)
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
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

	clusterDeleteResponse, err := r.DeleteV2ClustersClusterIDWithResponse(ctx, uuid.MustParse(state.ID.ValueString()))
	if serr := util.StatusOK(clusterDeleteResponse, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}
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
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *clusterResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if !plan.Name.Equal(state.Name) {
		resp.Diagnostics.AddError("Cannot update cluster name",
			"Updating the name is not permitted. "+
				"Current value: \""+state.Name.ValueString()+"\", configured value: \""+plan.Name.ValueString()+"\".")

		return
	}

	if err := validateModifyProjectName(plan, state); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

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

	if state.OptInPreviewFeature.ValueBool() && plan.DeploymentType.ValueString() != string(management.NONPRODUCTION) {
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

func validateCreateParameters(plan *clusterResourceModel) *util.SummaryWithDetailError {
	if !util.IsConfiguredString(plan.ProjectName) {
		return &util.SummaryWithDetailError{
			Summary: "Missing project_name",
			Detail:  "The project_name attribute is required when creating a cluster because the Management API requires a project ID.",
		}
	}

	providerSet := util.IsConfiguredString(plan.CloudProvider)
	regionSet := util.IsConfiguredString(plan.RegionName)
	if providerSet != regionSet {
		return &util.SummaryWithDetailError{
			Summary: "Invalid region configuration",
			Detail:  "Both 'cloud_provider' and 'region_name' must be provided together, or both omitted.",
		}
	}

	if plan.OptInPreviewFeature.ValueBool() && plan.DeploymentType.ValueString() != string(management.NONPRODUCTION) {
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

func validateAutoSuspendConfig(plan *clusterResourceModel) *util.SummaryWithDetailError {
	if plan.AutoSuspend.SuspendType.Equal(types.StringValue(string(management.DISABLED))) &&
		!plan.AutoSuspend.SuspendAfterSeconds.IsNull() {
		return &util.SummaryWithDetailError{
			Summary: "Invalid auto_suspend configuration.",
			Detail:  "If suspend_type is set to DISABLED, the suspend_after_seconds parameter is not allowed.",
		}
	}

	return nil
}

func validateAutoScaleConfig(plan *clusterResourceModel) *util.SummaryWithDetailError {
	if !plan.AutoScale.Sensitivity.Equal(types.StringValue(string(management.NORMAL))) &&
		plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) {
		return &util.SummaryWithDetailError{
			Summary: "Invalid auto_scale configuration.",
			Detail:  "If max_scale_factor is set to 1, the sensitivity parameter (if not set to its default value) is not allowed.",
		}
	}

	return nil
}

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

func toClusterResourceModel(cluster management.Cluster, adminPassword string, configuredFirewallRanges []types.String, projectName types.String) clusterResourceModel {
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
		AdminPassword:       types.StringValue(adminPassword),
		FirewallRanges:      firewallRangesForState(configuredFirewallRanges, cluster),
		ExpiresAt:           util.MaybeExpiresAtStringValue(cluster.ExpiresAt),
		CloudProvider:       normalizeCloudProvider(cluster.Provider),
		RegionName:          util.MaybeStringValue(cluster.Region),
		DeploymentType:      util.StringValueOrNull(cluster.DeploymentType),
		OptInPreviewFeature: types.BoolValue(util.Deref(cluster.OptInPreviewFeature)),
		MultiAZ:             types.BoolValue(util.Deref(cluster.MultiAZ)),
		OutboundAllowList:   util.MaybeStringValue(cluster.OutboundAllowList),
		Endpoint:            util.MaybeStringValue(cluster.Endpoint),
		CreatedAt:           util.MaybeTimeValue(cluster.CreatedAt),
		AutoScale:           toAutoScaleResourceModel(cluster),
		AutoSuspend:         toAutoSuspendResourceModel(cluster),
		UpdateWindow:        toUpdateWindowResourceModel(cluster.UpdateWindow),
	}

	return model
}

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

func toCreateAutoScale(plan clusterResourceModel) *management.AutoScale {
	if plan.AutoScale.MaxScaleFactor.Equal(types.Float32Value(scaleX1)) {
		return nil
	}

	return &management.AutoScale{
		MaxScaleFactor: util.MaybeFloat32(plan.AutoScale.MaxScaleFactor),
		Sensitivity:    util.WorkspaceAutoScaleSensitivityString(plan.AutoScale.Sensitivity),
	}
}

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

func toAutoScaleResourceModel(cluster management.Cluster) *autoScaleResourceModel {
	if cluster.AutoScale == nil {
		return &autoScaleResourceModel{
			MaxScaleFactor: types.Float32Value(scaleX1),
			Sensitivity:    types.StringValue(string(management.NORMAL)),
		}
	}

	return &autoScaleResourceModel{
		MaxScaleFactor: types.Float32PointerValue(cluster.AutoScale.MaxScaleFactor),
		Sensitivity:    util.StringValueOrNull(cluster.AutoScale.Sensitivity),
	}
}

func toAutoSuspend(plan clusterResourceModel) *management.AutoSuspend {
	suspendType := util.AutoSuspendSuspendTypeString(plan.AutoSuspend.SuspendType)
	result := &management.AutoSuspend{
		SuspendType: suspendType,
	}

	seconds := float32ToIntPtr(plan.AutoSuspend.SuspendAfterSeconds)
	if suspendType != nil {
		switch *suspendType {
		case management.IDLE:
			result.IdleAfterSeconds = seconds
		case management.SCHEDULED:
			result.ScheduledAfterSeconds = seconds
		case management.DISABLED:
		}
	}

	return result
}

func toAutoSuspendResourceModel(cluster management.Cluster) *autoSuspendResourceModel {
	if cluster.AutoSuspend == nil || cluster.AutoSuspend.SuspendType == nil {
		return &autoSuspendResourceModel{
			SuspendType: types.StringValue(string(management.DISABLED)),
		}
	}

	var suspendAfterSeconds *int
	switch *cluster.AutoSuspend.SuspendType {
	case management.IDLE:
		suspendAfterSeconds = cluster.AutoSuspend.IdleAfterSeconds
	case management.SCHEDULED:
		suspendAfterSeconds = cluster.AutoSuspend.ScheduledAfterSeconds
	case management.DISABLED:
	}

	return &autoSuspendResourceModel{
		SuspendAfterSeconds: intPtrToFloat32(suspendAfterSeconds),
		SuspendType:         util.StringValueOrNull(cluster.AutoSuspend.SuspendType),
	}
}

func float32ToIntPtr(f types.Float32) *int {
	if f.IsNull() || f.IsUnknown() {
		return nil
	}

	v := int(f.ValueFloat32())

	return &v
}

func intPtrToFloat32(i *int) types.Float32 {
	if i == nil {
		return types.Float32Null()
	}

	return types.Float32Value(float32(*i))
}

func toManagementUpdateWindow(ctx context.Context, uw types.Object) *management.UpdateWindow {
	if uw.IsNull() || uw.IsUnknown() {
		return nil
	}

	var model updateWindowResourceModel
	uw.As(ctx, &model, basetypes.ObjectAsOptions{})

	return &management.UpdateWindow{
		Hour: float32(model.Hour.ValueInt64()),
		Day:  float32(model.Day.ValueInt64()),
	}
}

func toUpdateWindowResourceModel(uw *management.UpdateWindow) types.Object {
	if uw == nil {
		return types.ObjectNull(map[string]attr.Type{
			"hour": types.Int64Type,
			"day":  types.Int64Type,
		})
	}

	obj, diags := types.ObjectValue(
		map[string]attr.Type{
			"hour": types.Int64Type,
			"day":  types.Int64Type,
		},
		map[string]attr.Value{
			"hour": types.Int64Value(int64(uw.Hour)),
			"day":  types.Int64Value(int64(uw.Day)),
		},
	)
	if diags.HasError() {
		panic(fmt.Sprintf("failed to create update_window object: %v", diags.Errors()))
	}

	return obj
}

func resolveProjectIDByName(ctx context.Context, c management.ClientWithResponsesInterface, projectName string) (*uuid.UUID, *util.SummaryWithDetailError) {
	projectsResponse, err := c.GetV2ProjectsWithResponse(ctx)
	if serr := util.StatusOK(projectsResponse, err); serr != nil {
		return nil, serr
	}

	var sameNameProjectIDs []uuid.UUID
	availableProjectNamesSet := make(map[string]struct{})
	for _, project := range util.Deref(projectsResponse.JSON200) {
		if project.Name == projectName {
			sameNameProjectIDs = append(sameNameProjectIDs, project.ProjectID)
		}

		availableProjectNamesSet[project.Name] = struct{}{}
	}

	availableProjectNames := make([]string, 0, len(availableProjectNamesSet))
	for name := range availableProjectNamesSet {
		availableProjectNames = append(availableProjectNames, fmt.Sprintf("'%s'", name))
	}

	if len(sameNameProjectIDs) == 0 {
		availableProjectsDetail := ""
		if len(availableProjectNames) > 0 {
			sort.Strings(availableProjectNames)
			availableProjectsDetail = fmt.Sprintf("Available projects: %s.", strings.Join(availableProjectNames, ", "))
		}

		return nil, &util.SummaryWithDetailError{
			Summary: "Project not found",
			Detail:  fmt.Sprintf("No project with the name '%s' was found. Set project_name to a valid project name. %s", projectName, availableProjectsDetail),
		}
	}

	if len(sameNameProjectIDs) > 1 {
		return nil, &util.SummaryWithDetailError{
			Summary: "Multiple projects found",
			Detail:  fmt.Sprintf("Multiple projects named '%s' were found. Please rename projects to use a unique project name for cluster assignment.", projectName),
		}
	}

	projectID := sameNameProjectIDs[0]

	return &projectID, nil
}

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
