package flow

import (
	"context"

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
	ResourceName = "flow"
)

var (
	_ resource.ResourceWithConfigure   = &flowInstanceResource{}
	_ resource.ResourceWithModifyPlan  = &flowInstanceResource{}
	_ resource.ResourceWithImportState = &flowInstanceResource{}
)

// flowInstanceResource is the resource implementation.
type flowInstanceResource struct {
	management.ClientWithResponsesInterface
}

// flowInstanceResourceModel maps the resource schema data.
type flowInstanceResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	UserName     types.String `tfsdk:"user_name"`
	DatabaseName types.String `tfsdk:"database_name"`
	Size         types.String `tfsdk:"size"`
	CreatedAt    types.String `tfsdk:"created_at"`
	Endpoint     types.String `tfsdk:"endpoint"`
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &flowInstanceResource{}
}

// Metadata returns the resource type name.
func (r *flowInstanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

// Schema defines the schema for the resource.
func (r *flowInstanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This resource enables the management of SingleStore Flow instances.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the Flow instance.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the Flow instance.",
			},
			"workspace_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the workspace to associate the Flow instance with.",
				Validators:          []validator.String{util.NewUUIDValidator()},
			},
			"user_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The username of the SingleStore database user to connect with.",
			},
			"database_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the SingleStore database to connect to.",
			},
			"size": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The size of the Flow instance (in Flow size notation), such as \"F1\", \"F2\", or \"F3\".",
				Validators:          []validator.String{NewSizeValidator()},
			},
			"created_at": schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The timestamp when the Flow instance was created.",
			},
			"endpoint": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The endpoint used to connect to the Flow instance.",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *flowInstanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan flowInstanceResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	workspaceID := uuid.MustParse(plan.WorkspaceID.ValueString())

	createBody := management.FlowCreate{
		Name:         plan.Name.ValueString(),
		WorkspaceID:  workspaceID,
		UserName:     plan.UserName.ValueString(),
		DatabaseName: plan.DatabaseName.ValueString(),
		Size:         util.Ptr(plan.Size.ValueString()),
	}

	flowCreateResponse, err := r.PostV1FlowWithResponse(ctx, createBody)
	if serr := util.StatusOK(flowCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	flowID := flowCreateResponse.JSON200.FlowID

	flow, werr := wait(ctx, r.ClientWithResponsesInterface, flowID, config.FlowInstanceCreationTimeout,
		waitConditionEndpointReady(),
	)
	if werr != nil {
		resp.Diagnostics.AddError(
			werr.Summary,
			werr.Detail,
		)

		return
	}

	result := toFlowInstanceResourceModel(flow, plan)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *flowInstanceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state flowInstanceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(state.ID.ValueString())

	flow, err := r.GetV1FlowFlowIDWithResponse(ctx, id)
	if serr := util.StatusOK(flow, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if flow.JSON200.DeletedAt != nil {
		resp.State.RemoveResource(ctx)

		return // The resource got terminated externally, deleting it from the state file to recreate.
	}

	result := toFlowInstanceResourceModel(*flow.JSON200, state)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
// Since Flow instances are immutable, all changes require replacement.
// This method should not be called due to RequiresReplace plan modifiers.
func (r *flowInstanceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"Flow instances are immutable. Any changes require resource replacement.",
	)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *flowInstanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state flowInstanceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	flowDeleteResponse, err := r.DeleteV1FlowFlowIDWithResponse(ctx, uuid.MustParse(state.ID.ValueString()))
	if serr := util.StatusOK(flowDeleteResponse, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *flowInstanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
func (r *flowInstanceResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *flowInstanceResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *flowInstanceResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if !plan.Name.Equal(state.Name) ||
		!plan.WorkspaceID.Equal(state.WorkspaceID) ||
		!plan.UserName.Equal(state.UserName) ||
		!plan.DatabaseName.Equal(state.DatabaseName) ||
		!plan.Size.Equal(state.Size) {
		resp.Diagnostics.AddError("Cannot update fields",
			"To prevent accidental deletion of data, updating fields for Flow instances is not allowed. "+
				"Please explicitly delete the Flow instance before updating fields.")
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *flowInstanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toFlowInstanceResourceModel(flow management.Flow, state flowInstanceResourceModel) flowInstanceResourceModel {
	model := flowInstanceResourceModel{
		ID:           util.UUIDStringValue(flow.FlowID),
		Name:         types.StringValue(flow.Name),
		WorkspaceID:  util.MaybeUUIDStringValue(flow.WorkspaceID),
		CreatedAt:    types.StringValue(flow.CreatedAt.String()),
		Endpoint:     util.MaybeStringValue(flow.Endpoint),
		Size:         util.MaybeStringValue(flow.Size),
		UserName:     state.UserName,
		DatabaseName: state.DatabaseName,
	}

	return model
}
