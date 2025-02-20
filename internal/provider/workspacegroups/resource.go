package workspacegroups

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

type updateWindowResourceModel struct {
	Hour types.Int64 `tfsdk:"hour"`
	Day  types.Int64 `tfsdk:"day"`
}

// workspaceGroupResourceModel maps the resource schema data.
type workspaceGroupResourceModel struct {
	ID             types.String               `tfsdk:"id"`
	Name           types.String               `tfsdk:"name"`
	FirewallRanges []types.String             `tfsdk:"firewall_ranges"`
	CreatedAt      types.String               `tfsdk:"created_at"`
	ExpiresAt      types.String               `tfsdk:"expires_at"`
	RegionID       types.String               `tfsdk:"region_id"`
	AdminPassword  types.String               `tfsdk:"admin_password"`
	UpdateWindow   *updateWindowResourceModel `tfsdk:"update_window"`
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
				Required:            true,
				MarkdownDescription: "The unique identifier of the region where the workspace group is to be created.",
				Validators:          []validator.String{util.NewUUIDValidator()},
			},
			"admin_password": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: `The admin SQL user password for the workspace group. If not provided, the server will automatically generate a secure password. Please note that updates to the admin password might take a brief moment to become effective.`,
			},
			"update_window": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Details of the scheduled update window for the workspace group. This is the time period during which any updates to the workspace group will occur.",
				Attributes: map[string]schema.Attribute{
					"hour": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "The hour of the day, in 24-hour UTC format (0-23), when the update window starts.",
					},
					"day": schema.Int64Attribute{
						Optional:            true,
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

	workspaceGroupCreateResponse, err := r.PostV1WorkspaceGroupsWithResponse(ctx, management.PostV1WorkspaceGroupsJSONRequestBody{
		AdminPassword:  util.MaybeString(plan.AdminPassword),
		ExpiresAt:      util.MaybeString(plan.ExpiresAt),
		FirewallRanges: util.StringFirewallRanges(plan.FirewallRanges),
		Name:           plan.Name.ValueString(),
		RegionID:       uuid.MustParse(plan.RegionID.ValueString()),
		UpdateWindow:   toManagementUpdateWindow(plan.UpdateWindow),
	})
	if serr := util.StatusOK(workspaceGroupCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	id := workspaceGroupCreateResponse.JSON200.WorkspaceGroupID
	wg, werr := waitStatusActive(ctx, r.ClientWithResponsesInterface, id)
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
	))

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
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

	if workspaceGroup.JSON200.State != management.WorkspaceGroupStateACTIVE {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Workspace group %s state is %s while it should be %s", state.ID.ValueString(), workspaceGroup.JSON200.State, management.WorkspaceGroupStateACTIVE),
			"An unexpected workspace group state.\n\n"+
				config.ContactSupportLaterErrorDetail,
		)

		return // A workspace group may be, e.g., PENDING during update windows when all the update activity is prohibited.
	}

	state = toWorkspaceGroupResourceModel(*workspaceGroup.JSON200, state.AdminPassword.ValueString())

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

	wg, werr := waitStatusActive(ctx, r.ClientWithResponsesInterface, id)
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	result := toWorkspaceGroupResourceModel(wg, plan.AdminPassword.ValueString())

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

	if !plan.RegionID.Equal(state.RegionID) {
		resp.Diagnostics.AddError("Cannot update workspace group region ID",
			"To prevent accidental deletion of the workspace group and loss of data, updating the region ID is not permitted. "+
				"Please explicitly delete the workspace group before changing its region ID.")

		return
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *workspaceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toWorkspaceGroupResourceModel(workspaceGroup management.WorkspaceGroup, adminPassword string) workspaceGroupResourceModel {
	return workspaceGroupResourceModel{
		ID:             util.UUIDStringValue(workspaceGroup.WorkspaceGroupID),
		Name:           types.StringValue(workspaceGroup.Name),
		FirewallRanges: util.FirewallRanges(workspaceGroup.FirewallRanges),
		CreatedAt:      types.StringValue(workspaceGroup.CreatedAt),
		ExpiresAt:      util.MaybeStringValue(workspaceGroup.ExpiresAt),
		RegionID:       util.UUIDStringValue(workspaceGroup.RegionID),
		AdminPassword:  types.StringValue(adminPassword),
		UpdateWindow:   toUpdateWindowResourceModel(workspaceGroup.UpdateWindow),
	}
}

func waitStatusActive(ctx context.Context, c management.ClientWithResponsesInterface, id management.WorkspaceGroupID) (management.WorkspaceGroup, *util.SummaryWithDetailError) {
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

		if workspaceGroup.JSON200.State == management.WorkspaceGroupStateFAILED {
			err := fmt.Errorf("workspace group %s creation failed; %s", workspaceGroup.JSON200.WorkspaceGroupID, config.ContactSupportErrorDetail)

			return retry.NonRetryableError(err)
		}

		if workspaceGroup.JSON200.State != management.WorkspaceGroupStateACTIVE {
			err = fmt.Errorf("workspace group %s state is %s", id, workspaceGroup.JSON200.State)

			return retry.RetryableError(err)
		}

		if !util.CheckLastN(workspaceGroupStateHistory, config.WorkspaceGroupConsistencyThreshold, management.WorkspaceGroupStateACTIVE) {
			err = fmt.Errorf("workspace group %s state is %s but the Management API did not return the same state for the consequent %d iterations yet",
				id, workspaceGroup.JSON200.State, config.WorkspaceGroupConsistencyThreshold,
			)

			return retry.RetryableError(err)
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

func toManagementUpdateWindow(uw *updateWindowResourceModel) *management.UpdateWindow {
	if uw == nil {
		return nil
	}

	return &management.UpdateWindow{
		Hour: float32(uw.Hour.ValueInt64()),
		Day:  float32(uw.Day.ValueInt64()),
	}
}

func toUpdateWindowResourceModel(uw *management.UpdateWindow) *updateWindowResourceModel {
	if uw == nil {
		return nil
	}

	return &updateWindowResourceModel{
		Hour: types.Int64Value(int64(uw.Hour)),
		Day:  types.Int64Value(int64(uw.Day)),
	}
}
