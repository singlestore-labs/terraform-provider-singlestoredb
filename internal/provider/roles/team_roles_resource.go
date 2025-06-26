package roles

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	TeamRolesGrantResourceName = "team_roles"
)

type TeamRolesGrantModel struct {
	ID     types.String          `tfsdk:"id"`
	TeamID types.String          `tfsdk:"team_id"`
	Roles  []RoleAttributesModel `tfsdk:"roles"`
}

type teamRolesGrantResource struct {
	management.ClientWithResponsesInterface
}

func NewTeamRolesGrantResource() resource.Resource {
	return &teamRolesGrantResource{}
}

func (r *teamRolesGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, TeamRolesGrantResourceName)
}

func (r *teamRolesGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages role grants for a team. Allows assigning roles to a team for specific resources.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the granted roles.",
			},
			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the team.",
			},
			"roles": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "A list of roles assigned to the team.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: RoleAttributesSchema(),
				},
			},
		},
	}
}

func (r *teamRolesGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TeamRolesGrantModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if len(plan.Roles) == 0 {
		resp.Diagnostics.AddError(
			"Invalid Roles",
			"The 'roles' attribute must contain at least one role to grant.",
		)

		return
	}

	_, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), nil, nil, &plan.Roles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	ok, err := grantTeamRoles(ctx, r.ClientWithResponsesInterface, plan.TeamID, plan.Roles)

	if !ok || err != nil {
		resp.Diagnostics.AddError(
			"Failed to grant team roles",
			"An error occurred while granting team roles: "+err.Error(),
		)

		return
	}

	roles, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), nil, &plan.Roles, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state := toTeamRolesGrantModel(plan.TeamID, roles)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *teamRolesGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TeamRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, err := getTeamRolesAndValidate(ctx, r, state.TeamID.String(), nil, &state.Roles, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state = toTeamRolesGrantModel(state.TeamID, roles)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *teamRolesGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state *TeamRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *TeamRolesGrantModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	grantRoles := SubtractRoles(plan.Roles, state.Roles)
	revokeRoles := SubtractRoles(state.Roles, plan.Roles)

	_, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), nil, nil, &grantRoles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	ok, err := r.doUpdate(ctx, plan.TeamID, grantRoles, revokeRoles)
	if !ok || err != nil {
		resp.Diagnostics.AddError(
			"Failed to update team roles",
			"An error occurred while updating team roles: "+err.Error(),
		)

		return
	}

	roles, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), nil, &plan.Roles, &revokeRoles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	result := toTeamRolesGrantModel(plan.TeamID, roles)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *teamRolesGrantResource) doUpdate(ctx context.Context, teamID types.String, grantRoles, revokeRoles []RoleAttributesModel) (bool, error) {
	ok, err := revokeTeamRoles(ctx, r.ClientWithResponsesInterface, teamID, revokeRoles)
	if !ok || err != nil {
		return ok, err
	}

	return grantTeamRoles(ctx, r.ClientWithResponsesInterface, teamID, grantRoles)
}

func (r *teamRolesGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TeamRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ok, err := revokeTeamRoles(ctx, r.ClientWithResponsesInterface, state.TeamID, state.Roles)
	if !ok || err != nil {
		resp.Diagnostics.AddError(
			"Failed to revoke team roles",
			"An error occurred while revoking team roles: "+err.Error(),
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *teamRolesGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (r *teamRolesGrantResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *TeamRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *TeamRolesGrantModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if plan.TeamID != state.TeamID {
		resp.Diagnostics.AddError(
			"Cannot update team ID",
			"Updating the team_id is not permitted. Please explicitly delete(revoke) the granted roles before changing the team_id.",
		)

		return
	}
}

func toTeamRolesGrantModel(teamID types.String, roles []RoleAttributesModel) TeamRolesGrantModel {
	if roles == nil {
		roles = []RoleAttributesModel{}
	}

	return TeamRolesGrantModel{
		ID:     teamID,
		TeamID: teamID,
		Roles:  roles,
	}
}
