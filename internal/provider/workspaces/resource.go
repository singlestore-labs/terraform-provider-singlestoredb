package workspaces

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	resourceName = "workspace"
)

var (
	_ resource.ResourceWithConfigure   = &workspaceResource{}
	_ resource.ResourceWithModifyPlan  = &workspaceResource{}
	_ resource.ResourceWithImportState = &workspaceResource{}
)

// workspaceResource is the resource implementation.
type workspaceResource struct {
	management.ClientWithResponsesInterface
}

// workspaceResourceModel maps the resource schema data.
type workspaceResourceModel struct {
	ID               types.String `tfsdk:"id"`
	WorkspaceGroupID types.String `tfsdk:"workspace_group_id"`
	Name             types.String `tfsdk:"name"`
	Size             types.String `tfsdk:"size"`
	CreatedAt        types.String `tfsdk:"created_at"`
	Endpoint         types.String `tfsdk:"endpoint"`
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &workspaceResource{}
}

// Metadata returns the resource type name.
func (r *workspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, resourceName)
}

// Schema defines the schema for the resource.
func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "ID of the workspace",
			},
			"workspace_group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the workspace group",
				Validators:          []validator.String{util.NewUUIDValidator()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the workspace",
			},
			"size": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Size of the workspace (in workspace size notation), such as 0 (suspended), 0.25 (S-00), 0.5 (S-0), 1 (S-1) or 2 (S-2)",
				Validators:          []validator.String{NewSizeValidator()},
			},
			"created_at": schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The timestamp of when the workspace was created",
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Endpoint to connect to the workspace",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Size.ValueString() == config.WorkspaceSizeSuspended {
		resp.Diagnostics.AddError(
			"Cannot suspend a workspace during creation",
			fmt.Sprintf("Invalid workspace size %s during creation, specify at least 0.25 (S-00)", config.WorkspaceSizeSuspended),
		)

		return
	}

	workspaceCreateResponse, err := r.PostV1WorkspacesWithResponse(ctx, management.PostV1WorkspacesJSONRequestBody{
		Name:             plan.Name.ValueString(),
		Size:             util.MaybeString(plan.Size),
		WorkspaceGroupID: uuid.MustParse(plan.WorkspaceGroupID.String()),
	})
	if serr := util.StatusOK(workspaceCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	w, werr := wait(ctx, r.ClientWithResponsesInterface, workspaceCreateResponse.JSON200.WorkspaceID, config.WorkspaceCreationTimeout,
		waitConditionState(management.WorkspaceStateACTIVE),
	)
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	result, terr := toWorkspaceResourceModel(w)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(state.ID.ValueString())

	workspace, err := r.GetV1WorkspacesWorkspaceIDWithResponse(ctx, id,
		&management.GetV1WorkspacesWorkspaceIDParams{},
	)
	if serr := util.StatusOK(workspace, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if workspace.JSON200.State == management.WorkspaceStateTERMINATED {
		resp.State.RemoveResource(ctx)

		return // The resource got terminated externally, deleting it from the state file to recreate.
	}

	if workspace.JSON200.State == management.WorkspaceStatePENDING {
		w, werr := wait(ctx, r.ClientWithResponsesInterface, id, config.WorkspaceReadTimeout,
			// A workspace may be, e.g., PENDING during update windows when all the update activity is prohibited.
			// Waiting for a bit and erroring only after the timeout.
			waitConditionState(management.WorkspaceStateACTIVE, management.WorkspaceStateSUSPENDED),
		)
		if werr == nil {
			*workspace.JSON200 = w
		}
	}

	if workspace.JSON200.State != management.WorkspaceStateACTIVE &&
		workspace.JSON200.State != management.WorkspaceStateSUSPENDED {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Workspace %s state is %s while it should be %s or %s", state.ID.ValueString(), workspace.JSON200.State, management.WorkspaceStateACTIVE, management.WorkspaceStateSUSPENDED),
			"An unexpected workspace state.\n\n"+
				"If nothing changes in a few hours, contact SingleStore support.",
		)

		return
	}

	state, terr := toWorkspaceResourceModel(*workspace.JSON200)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan workspaceResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var uerr *util.SummaryWithDetailError
	state, uerr = updateSize(ctx, r.ClientWithResponsesInterface, state, plan)
	if uerr != nil {
		resp.Diagnostics.AddError(
			uerr.Summary,
			uerr.Detail,
		)

		return
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceDeleteResponse, err := r.DeleteV1WorkspacesWorkspaceIDWithResponse(ctx, uuid.MustParse(state.ID.ValueString()))
	if serr := util.StatusOK(workspaceDeleteResponse, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
//
// `RequiresReplace` is not used because deleting a workspace result in losing database attachments.
func (r *workspaceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *workspaceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *workspaceResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if !plan.Name.Equal(state.Name) {
		resp.Diagnostics.AddError("Cannot update workspace name",
			"To prevent accidental deletion of the databases that are attached to the workspace, updating the name is not permitted. "+
				"Please explicitly delete the workspace before changing its name.")

		return
	}

	if !plan.WorkspaceGroupID.Equal(state.WorkspaceGroupID) {
		resp.Diagnostics.AddError("Cannot update workspace group ID",
			"To prevent accidental deletion of the databases that are attached to the workspace, updating the workspace group ID is not permitted. "+
				"Please explicitly delete the workspace before changing its workspace group ID.")

		return
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toWorkspaceResourceModel(workspace management.Workspace) (workspaceResourceModel, *util.SummaryWithDetailError) {
	size, perr := ParseSize(workspace.Size, workspace.State)
	if perr != nil {
		return workspaceResourceModel{}, perr
	}

	return workspaceResourceModel{
		ID:               util.UUIDStringValue(workspace.WorkspaceID),
		WorkspaceGroupID: util.UUIDStringValue(workspace.WorkspaceGroupID),
		Name:             types.StringValue(workspace.Name),
		Size:             types.StringValue(size.String()),
		CreatedAt:        types.StringValue(workspace.CreatedAt),
		Endpoint:         util.MaybeStringValue(workspace.Endpoint),
	}, nil
}
