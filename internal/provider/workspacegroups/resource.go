package workspacegroups

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	ResourceName = "workspace_group"
)

var (
	_ resource.ResourceWithConfigure   = &workspaceGroupResource{}
	_ resource.ResourceWithModifyPlan  = &workspaceGroupResource{}
	_ resource.ResourceWithImportState = &workspaceGroupResource{}
)

// workspaceGroupResource is the resource implementation.
type workspaceGroupResource struct {
	management.ClientWithResponsesInterface
}

// workspaceGroupResourceModel maps the resource schema data.
type workspaceGroupResourceModel struct {
	ID                       types.String                 `tfsdk:"id"`
	Name                     types.String                 `tfsdk:"name"`
	FirewallRanges           []types.String               `tfsdk:"firewall_ranges"`
	CreatedAt                types.String                 `tfsdk:"created_at"`
	ExpiresAt                types.String                 `tfsdk:"expires_at"`
	RegionID                 types.String                 `tfsdk:"region_id"`
	CloudProvider            types.String                 `tfsdk:"cloud_provider"`
	RegionName               types.String                 `tfsdk:"region_name"`
	AdminPassword            types.String                 `tfsdk:"admin_password"`
	DeploymentType           types.String                 `tfsdk:"deployment_type"`
	OptInPreviewFeature      types.Bool                   `tfsdk:"opt_in_preview_feature"`
	HighAvailabilityTwoZones types.Bool                   `tfsdk:"high_availability_two_zones"`
	OutboundAllowList        types.String                 `tfsdk:"outbound_allow_list"`
	UpdateWindow             *updateWindowDataSourceModel `tfsdk:"update_window"`
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &workspaceGroupResource{}
}

// Metadata returns the resource type name.
func (r *workspaceGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

// Schema defines the schema for the resource.
func (r *workspaceGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage SingleStoreDB workspace groups with this resource.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the workspace group.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the workspace group.",
			},
			"firewall_ranges": schema.ListAttribute{
				ElementType:         types.StringType,
				Required:            true,
				MarkdownDescription: "List of allowed CIDR ranges. An empty list blocks all inbound requests. For unrestricted traffic, use [\"0.0.0.0/0\"]. Note that updates to firewall ranges may take a brief moment to become effective.",
			},
			"created_at": schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The timestamp when the workspace was created.",
			},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: `The expiration timestamp of the workspace group. If not specified, the workspace group never expires. Upon expiration, the workspace group is terminated and all its data is lost. Set the expiration time as an RFC3339 UTC timestamp, e.g., "2221-01-02T15:04:05Z".`,
				Validators:          []validator.String{util.NewTimeValidator()},
			},
			"region_id": schema.StringAttribute{
				Optional:            true,
				DeprecationMessage:  "Use 'cloud_provider' and 'region_name' instead.",
				MarkdownDescription: "The unique identifier of the region where the workspace group is to be created.",
				Validators:          []validator.String{util.NewUUIDValidator()},
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
			"admin_password": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: `The admin SQL user password for the workspace group. If not provided, the server will automatically generate a secure password. Please note that updates to the admin password might take a brief moment to become effective.`,
			},
			"deployment_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The deployment type that will be applied to all the workspaces within the workspace group. It can have one of the following values: `PRODUCTION` or `NON-PRODUCTION`. The default value is `PRODUCTION`.",
				Default:             stringdefault.StaticString(string(management.WorkspaceGroupCreateDeploymentTypePRODUCTION)),
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.WorkspaceGroupCreateDeploymentTypePRODUCTION), string(management.WorkspaceGroupCreateDeploymentTypeNONPRODUCTION)),
				},
			},
			"opt_in_preview_feature": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "If enabled, the deployment gets the latest features and updates immediately. Suitable only for `NON-PRODUCTION` deployments and cannot be changed after creation.",
			},
			"high_availability_two_zones": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Enables deployment across two Availability Zones.",
			},
			"outbound_allow_list": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The account ID which must be allowed for outbound connections. This is only applicable to AWS provider.",
			},
			"update_window": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Details of the scheduled update window for the workspace group. This is the time period during which any updates to the workspace group will occur.",
				Attributes: map[string]schema.Attribute{
					"hour": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "The hour of the day, in 24-hour UTC format (0-23), when the update window starts.",
					},
					"day": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: "The day of the week (0-6), where 0 is Sunday and 6 is Saturday, when the update window is scheduled.",
					},
				},
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *workspaceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := validateRequiredRegionParameters(&plan); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	if err := validateCreateOptInPreviewFeatureParameter(plan); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	regionIDIsSet := !plan.RegionID.IsNull() && !plan.RegionID.IsUnknown()
	var regionID *uuid.UUID
	if regionIDIsSet {
		regionID = util.Ptr(uuid.MustParse(plan.RegionID.ValueString()))
	}

	workspaceGroupCreateResponse, err := r.PostV1WorkspaceGroupsWithResponse(ctx, management.PostV1WorkspaceGroupsJSONRequestBody{
		AdminPassword:            util.MaybeString(plan.AdminPassword),
		ExpiresAt:                util.MaybeString(plan.ExpiresAt),
		FirewallRanges:           util.StringFirewallRanges(plan.FirewallRanges),
		Name:                     plan.Name.ValueString(),
		RegionID:                 regionID,
		Provider:                 util.WorkspaceGroupCloudProviderString(plan.CloudProvider.ValueString()),
		RegionName:               util.MaybeString(plan.RegionName),
		DeploymentType:           util.WorkspaceGroupCreateDeploymentTypeString(plan.DeploymentType),
		OptInPreviewFeature:      util.MaybeBool(plan.OptInPreviewFeature),
		HighAvailabilityTwoZones: util.MaybeBool(plan.HighAvailabilityTwoZones),
	})
	if serr := util.StatusOK(workspaceGroupCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	id := workspaceGroupCreateResponse.JSON200.WorkspaceGroupID
	wg, werr := verifyStatusAndGetWorkspaceGroup(ctx, r.ClientWithResponsesInterface, id, waitConditionFirewallRanges(plan.FirewallRanges))
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	result := toWorkspaceGroupResourceModel(wg, util.FirstNotEmpty(
		plan.AdminPassword.ValueString(),
		util.Deref(workspaceGroupCreateResponse.JSON200.AdminPassword), // Either from input or output.
	), regionIDIsSet)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func validateRequiredRegionParameters(plan *workspaceGroupResourceModel) *util.SummaryWithDetailError {
	providerAndRegionNameAreSet := !plan.CloudProvider.IsNull() && !plan.CloudProvider.IsUnknown() && !plan.RegionName.IsNull() && !plan.RegionName.IsUnknown()
	regionIDIsSet := !plan.RegionID.IsNull() && !plan.RegionID.IsUnknown()

	if regionIDIsSet && (providerAndRegionNameAreSet) ||
		!regionIDIsSet && (!providerAndRegionNameAreSet) {
		return &util.SummaryWithDetailError{
			Summary: "Invalid region configuration",
			Detail:  "Either 'region_id' must be set or both 'cloud_provider' and 'region_name' must be provided.",
		}
	}

	return nil
}

func validateCreateOptInPreviewFeatureParameter(plan workspaceGroupResourceModel) *util.SummaryWithDetailError {
	if plan.OptInPreviewFeature.ValueBool() && plan.DeploymentType.ValueString() != string(management.WorkspaceGroupCreateDeploymentTypeNONPRODUCTION) {
		return &util.SummaryWithDetailError{
			Summary: "Wrong configuration for opt_in_preview_feature and deployment_type",
			Detail:  "The enabled opt_in_preview_feature configuration is suitable only for the 'NON-PRODUCTION' deployment_type.",
		}
	}

	return nil
}

// Read refreshes the Terraform state with the latest data.
func (r *workspaceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceGroup, err := r.GetV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx,
		uuid.MustParse(state.ID.ValueString()),
		&management.GetV1WorkspaceGroupsWorkspaceGroupIDParams{},
	)
	if serr := util.StatusOK(workspaceGroup, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if workspaceGroup.JSON200.State == management.WorkspaceGroupStateTERMINATED {
		resp.State.RemoveResource(ctx)

		return // The resource got terminated externally, deleting it from the state file to recreate.
	}

	if isFatalWorkspaceGroupState(workspaceGroup.JSON200.State) {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Workspace group %s state is %s while it should be %s or %s", state.ID.ValueString(), workspaceGroup.JSON200.State, management.WorkspaceGroupStateACTIVE, management.WorkspaceGroupStatePENDING),
			"An unexpected workspace group state.\n\n"+
				config.ContactSupportLaterErrorDetail,
		)

		return // A workspace group may be, e.g., PENDING during update windows when all the update activity is prohibited.
	}

	regionIDIsSet := !state.RegionID.IsNull() && !state.RegionID.IsUnknown()
	state = toWorkspaceGroupResourceModel(*workspaceGroup.JSON200, state.AdminPassword.ValueString(), regionIDIsSet)
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *workspaceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workspaceGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(plan.ID.ValueString())
	workspaceGroupUpdateResponse, err := r.PatchV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx, id,
		management.WorkspaceGroupUpdate{
			AdminPassword:  util.MaybeString(plan.AdminPassword),
			ExpiresAt:      util.MaybeString(plan.ExpiresAt),
			Name:           util.MaybeString(plan.Name),
			FirewallRanges: util.Ptr(util.StringFirewallRanges(plan.FirewallRanges)),
			DeploymentType: util.WorkspaceGroupUpdateDeploymentTypeString(plan.DeploymentType),
			UpdateWindow:   toManagementUpdateWindow(plan.UpdateWindow),
		},
	)
	if serr := util.StatusOK(workspaceGroupUpdateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	wg, werr := verifyStatusAndGetWorkspaceGroup(ctx, r.ClientWithResponsesInterface, id)
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	regionIDIsSet := !plan.RegionID.IsNull() && !plan.RegionID.IsUnknown()
	result := toWorkspaceGroupResourceModel(wg, plan.AdminPassword.ValueString(), regionIDIsSet)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *workspaceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceGroupDeleteResponse, err := r.DeleteV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx,
		uuid.MustParse(state.ID.ValueString()),
		&management.DeleteV1WorkspaceGroupsWorkspaceGroupIDParams{Force: util.Ptr(true)}, // Deleting even if workspaces in the group.
	)
	if serr := util.StatusOK(workspaceGroupDeleteResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *workspaceGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
//
// `RequiresReplace` is not used because deleting a workspace group results in the data loss.
func (r *workspaceGroupResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *workspaceGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *workspaceGroupResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if err := validateModifyPlanRegionParameters(ctx, r, plan, state); err != nil {
		resp.Diagnostics.AddError(err.Summary, err.Detail)

		return
	}

	if !plan.HighAvailabilityTwoZones.Equal(state.HighAvailabilityTwoZones) {
		resp.Diagnostics.AddError("Cannot change the high_availability_two_zones configuration for the workspace group.",
			"Changing the high_availability_two_zones configuration is currently not supported.")

		return
	}

	if !plan.OptInPreviewFeature.Equal(state.OptInPreviewFeature) {
		resp.Diagnostics.AddError("Cannot change the opt_in_preview_feature configuration for the workspace group.",
			"Changing the opt_in_preview_feature configuration is currently not supported.")

		return
	}

	if state.OptInPreviewFeature.ValueBool() && plan.DeploymentType.ValueString() != string(management.WorkspaceGroupCreateDeploymentTypeNONPRODUCTION) {
		resp.Diagnostics.AddError(
			"Cannot change the deployment_type configuration to anything other than 'NON-PRODUCTION' for the workspace group when the opt_in_preview_feature is enabled.",
			"Changing the deployment_type configuration to anything other than 'NON-PRODUCTION' when the opt_in_preview_feature is enabled is not currently supported.",
		)

		return
	}
}

func validateModifyPlanRegionParameters(ctx context.Context, r *workspaceGroupResource, plan, state *workspaceGroupResourceModel) *util.SummaryWithDetailError {
	if err := validateRequiredRegionParameters(plan); err != nil {
		return err
	}

	if err := handleRegionMigrationState(ctx, r, plan, state); err != nil {
		return err
	}

	if err := validateModifyRegionID(plan, state); err != nil {
		return err
	}

	if err := validateModifyRegionNameAndProvider(plan, state); err != nil {
		return err
	}

	return nil
}

func handleRegionMigrationState(ctx context.Context, r *workspaceGroupResource, plan, state *workspaceGroupResourceModel) *util.SummaryWithDetailError {
	shouldMigrateToRegionNameAndProvider := !state.RegionID.IsNull() && plan.RegionID.IsNull()
	shouldMigrateToDepricatedRegionID := !state.RegionName.IsNull() && plan.RegionName.IsNull() && !state.CloudProvider.IsNull() && plan.CloudProvider.IsNull()

	if shouldMigrateToDepricatedRegionID {
		return &util.SummaryWithDetailError{
			Summary: "Cannot change cloud_provider and region_name to the deprecated region_id.",
			Detail:  "Changing cloud_provider and region_name to the deprecated region_id is not permitted. Use the cloud_provider and region_name parameters instead.",
		}
	}

	if shouldMigrateToRegionNameAndProvider {
		workspaceGroup, err := r.GetV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx,
			uuid.MustParse(state.ID.ValueString()),
			&management.GetV1WorkspaceGroupsWorkspaceGroupIDParams{},
		)
		if serr := util.StatusOK(workspaceGroup, err); serr != nil {
			return serr
		}
		state.CloudProvider = types.StringValue(string(workspaceGroup.JSON200.Provider))
		state.RegionName = types.StringValue(workspaceGroup.JSON200.RegionName)
	}

	return nil
}

func validateModifyRegionID(plan, state *workspaceGroupResourceModel) *util.SummaryWithDetailError {
	if !plan.RegionID.IsNull() && !state.RegionID.IsNull() && !plan.RegionID.Equal(state.RegionID) {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update workspace group region_id",
			Detail:  "Updating the region_id is not permitted. Warning: this field is deprecated. Use cloud_provider and region_name instead.",
		}
	}

	return nil
}

func validateModifyRegionNameAndProvider(plan, state *workspaceGroupResourceModel) *util.SummaryWithDetailError {
	if !plan.RegionName.Equal(state.RegionName) {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update workspace group region_name",
			Detail:  fmt.Sprintf("Updating the region_name is not permitted. Expected value is '%s', but now is '%s'.", state.RegionName.ValueString(), plan.RegionName.ValueString()),
		}
	}
	if !plan.CloudProvider.Equal(state.CloudProvider) {
		return &util.SummaryWithDetailError{
			Summary: "Cannot update workspace group cloud_provider",
			Detail:  fmt.Sprintf("Updating the cloud_provider is not permitted. Expected value is '%s'.", state.CloudProvider.ValueString()),
		}
	}

	return nil
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *workspaceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toWorkspaceGroupResourceModel(workspaceGroup management.WorkspaceGroup, adminPassword string, regionIDIsSet bool) workspaceGroupResourceModel {
	result := workspaceGroupResourceModel{
		ID:                       util.UUIDStringValue(workspaceGroup.WorkspaceGroupID),
		Name:                     types.StringValue(workspaceGroup.Name),
		FirewallRanges:           util.FirewallRanges(workspaceGroup.FirewallRanges),
		CreatedAt:                types.StringValue(workspaceGroup.CreatedAt),
		ExpiresAt:                util.MaybeStringValue(workspaceGroup.ExpiresAt),
		AdminPassword:            types.StringValue(adminPassword),
		DeploymentType:           util.StringValueOrNull(workspaceGroup.DeploymentType),
		OptInPreviewFeature:      types.BoolValue(workspaceGroup.OptInPreviewFeature != nil && *workspaceGroup.OptInPreviewFeature),
		HighAvailabilityTwoZones: types.BoolValue(workspaceGroup.HighAvailabilityTwoZones != nil && *workspaceGroup.HighAvailabilityTwoZones),
		OutboundAllowList:        util.MaybeStringValue(workspaceGroup.OutboundAllowList),
		UpdateWindow:             toUpdateWindowDataSourceModel(workspaceGroup.UpdateWindow),
	}
	if regionIDIsSet {
		result.RegionID = util.UUIDStringValue(workspaceGroup.RegionID)
	} else {
		result.CloudProvider = normalizeCloudProvider(workspaceGroup.Provider)
		result.RegionName = types.StringValue(workspaceGroup.RegionName)
	}

	return result
}

func normalizeCloudProvider(provider management.CloudProvider) basetypes.StringValue {
	result := util.WorkspaceGroupCloudProviderString(string(provider))
	if result == nil {
		return types.StringValue(string(provider)) // We should never get here but keeping the old behavior as the default.
	}

	return types.StringValue(string(*result))
}

// waitCondition return nil if it is satisfied.
type waitCondition func(management.WorkspaceGroup) error

func verifyStatusAndGetWorkspaceGroup(ctx context.Context, c management.ClientWithResponsesInterface, id management.WorkspaceGroupID, conditions ...waitCondition) (management.WorkspaceGroup, *util.SummaryWithDetailError) {
	result := management.WorkspaceGroup{}

	workspaceGroupStateHistory := make([]management.WorkspaceGroupState, 0, config.WorkspaceGroupConsistencyThreshold)

	if err := retry.RetryContext(ctx, config.WorkspaceGroupCreationTimeout, func() *retry.RetryError {
		workspaceGroup, err := c.GetV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx, id, &management.GetV1WorkspaceGroupsWorkspaceGroupIDParams{})
		if err != nil { // Not status code OK does not get here, not retrying for that reason.
			ferr := fmt.Errorf("failed to get workspace group %s: %w", id, err)

			return retry.NonRetryableError(ferr)
		}

		if code := workspaceGroup.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get workspace group %s: status code %s", id, http.StatusText(code))

			return retry.RetryableError(err)
		}

		workspaceGroupStateHistory = append(workspaceGroupStateHistory, workspaceGroup.JSON200.State)

		if isFatalWorkspaceGroupState(workspaceGroup.JSON200.State) {
			err := fmt.Errorf("workspace group %s creation failed; %s", workspaceGroup.JSON200.WorkspaceGroupID, config.ContactSupportErrorDetail)

			return retry.NonRetryableError(err)
		}

		if !util.CheckLastN(workspaceGroupStateHistory, config.WorkspaceGroupConsistencyThreshold, management.WorkspaceGroupStateACTIVE, management.WorkspaceGroupStatePENDING) {
			err = fmt.Errorf("workspace group %s state is %s but the Management API did not return the same state for the consequent %d iterations yet",
				id, workspaceGroup.JSON200.State, config.WorkspaceGroupConsistencyThreshold,
			)

			return retry.RetryableError(err)
		}

		for _, c := range conditions {
			if err := c(*workspaceGroup.JSON200); err != nil {
				return retry.RetryableError(err)
			}
		}

		result = *workspaceGroup.JSON200

		return nil
	}); err != nil {
		return management.WorkspaceGroup{}, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to wait for a workspace group %s creation", id),
			Detail:  fmt.Sprintf("Workspace group is not ready: %s", err),
		}
	}

	return result, nil
}

func isFatalWorkspaceGroupState(state management.WorkspaceGroupState) bool {
	return state == management.WorkspaceGroupStateFAILED || state == management.WorkspaceGroupStateTERMINATED
}

func waitConditionFirewallRanges(firewallRanges []types.String) func(management.WorkspaceGroup) error {
	return func(w management.WorkspaceGroup) error {
		switch {
		case len(firewallRanges) == 0:
			return nil
		case w.FirewallRanges == nil || len(*w.FirewallRanges) != len(firewallRanges):
			return fmt.Errorf("workspace group %s firewallRanges length should be %d", w.WorkspaceGroupID, len(firewallRanges))
		default:
			return nil
		}
	}
}

func toManagementUpdateWindow(uw *updateWindowDataSourceModel) *management.UpdateWindow {
	if uw == nil {
		return nil
	}

	day, err := strconv.Atoi(fmt.Sprint(uw.Day))
	if err != nil {
		return nil
	}
	hour, err := strconv.Atoi(fmt.Sprint(uw.Hour))
	if err != nil {
		return nil
	}

	return &management.UpdateWindow{
		Hour: float32(hour),
		Day:  float32(day),
	}
}

func toUpdateWindowDataSourceModel(uw *management.UpdateWindow) *updateWindowDataSourceModel {
	if uw == nil {
		return nil
	}

	return &updateWindowDataSourceModel{
		Hour: types.Int64Value(int64(uw.Hour)),
		Day:  types.Int64Value(int64(uw.Day)),
	}
}
