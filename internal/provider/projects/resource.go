package projects

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	ResourceName = "project"
)

var (
	_ resource.ResourceWithConfigure   = &projectResource{}
	_ resource.ResourceWithModifyPlan  = &projectResource{}
	_ resource.ResourceWithImportState = &projectResource{}
)

type projectResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Edition   types.String `tfsdk:"edition"`
	CreatedAt types.String `tfsdk:"created_at"`
}

type projectResource struct {
	management.ClientWithResponsesInterface
}

func NewResource() resource.Resource {
	return &projectResource{}
}

func (r *projectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

func (r *projectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage SingleStoreDB projects with this resource. Projects are used to organize workspace groups. The 'apply' action creates a new project or updates an existing one. The 'destroy' action deletes the project.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the project.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the project.",
			},
			"edition": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The edition of the project. Valid values are: ENTERPRISE, STANDARD, SHARED.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(management.ENTERPRISE), string(management.STANDARD), string(management.SHARED)),
				},
			},
			"created_at": schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The timestamp when the project was created.",
			},
		},
	}
}

func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createResp, err := r.PostV1ProjectsWithResponse(ctx, management.PostV1ProjectsJSONRequestBody{
		Name:    util.ToString(plan.Name),
		Edition: management.ProjectEdition(plan.Edition.ValueString()),
	})
	if serr := util.StatusOK(createResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	project, err := r.GetV1ProjectsProjectIDWithResponse(ctx, createResp.JSON200.ProjectID)
	if serr := util.StatusOK(project, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toProjectResourceModel(*project.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	project, err := r.GetV1ProjectsProjectIDWithResponse(ctx, uuid.MustParse(state.ID.ValueString()))
	if serr := util.StatusOK(project, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	if project.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)

		return
	}

	state = toProjectResourceModel(*project.JSON200)
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(plan.ID.ValueString())
	patchResp, err := r.PatchV1ProjectsProjectIDWithResponse(ctx, id, management.PatchV1ProjectsProjectIDJSONRequestBody{
		Name: util.ToString(plan.Name),
	})
	if serr := util.StatusOK(patchResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	project, err := r.GetV1ProjectsProjectIDWithResponse(ctx, id)
	if serr := util.StatusOK(project, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toProjectResourceModel(*project.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	deleteResp, err := r.DeleteV1ProjectsProjectIDWithResponse(ctx, uuid.MustParse(state.ID.ValueString()))
	if serr := util.StatusOK(deleteResp, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}
}

func (r *projectResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *projectResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *projectResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if !plan.Edition.Equal(state.Edition) {
		resp.Diagnostics.AddError(
			"Cannot update edition",
			"Updating the \"edition\" field is not allowed. Current value: \""+state.Edition.ValueString()+"\", configured value: \""+plan.Edition.ValueString()+"\". Please explicitly delete the project before changing its edition.",
		)
	}
}

func (r *projectResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	util.ImportStatePassthroughID(ctx, req, resp)
}

func toProjectResourceModel(project management.Project) projectResourceModel {
	return projectResourceModel{
		ID:        util.UUIDStringValue(project.ProjectID),
		Name:      types.StringValue(project.Name),
		Edition:   types.StringValue(string(project.Edition)),
		CreatedAt: types.StringValue(project.CreatedAt.String()),
	}
}
