package roles

import (
	"context"
	"errors"

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
	TeamRoleGrantResourceName = "team_role"
)

type TeamRoleGrantModel struct {
	ID     types.String        `tfsdk:"id"`
	TeamID types.String        `tfsdk:"team_id"`
	Role   RoleAttributesModel `tfsdk:"role"`
}

type teamRoleGrantResource struct {
	management.ClientWithResponsesInterface
}

func NewTeamRoleGrantResource() resource.Resource {
	return &teamRoleGrantResource{}
}

func (r *teamRoleGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, TeamRoleGrantResourceName)
}

func (r *teamRoleGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single role grant for a team (the 'subject' in RBAC terminology). This resource allows you to assign a specific role to a team, defining what access permission the team has to a particular resource (object) in the system. In Role-Based Access Control, this resource establishes the relationship between the subject (team), the permission level (role), and the target resource that can be accessed. Use the `singlestoredb_roles` data source with a specific resource's type and ID to discover what roles are available for that resource object. This resource is currently in beta and may undergo changes in future releases.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the granted role.",
			},
			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the team.",
			},
			"role": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "The role to be assigned to the team.",
				Attributes:          RoleAttributesSchema(),
			},
		},
	}
}

func (r *teamRoleGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TeamRoleGrantModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), plan.Role.ResourceType.ValueStringPointer(), nil, &[]RoleAttributesModel{plan.Role})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	ok, err := modifyTeamAccessControlsForResource(ctx, r.ClientWithResponsesInterface, plan.TeamID, plan.Role.ResourceID, plan.Role.ResourceType, &[]RoleAttributesModel{plan.Role}, nil)
	if !ok || err != nil {
		resp.Diagnostics.AddError(
			"Failed to grant team role",
			"An error occurred while attempting to grant the specified role to the team. Details: "+err.Error(),
		)

		return
	}

	roles, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), plan.Role.ResourceType.ValueStringPointer(), &[]RoleAttributesModel{plan.Role}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state := TeamRoleGrantModel{
		ID:     plan.TeamID,
		TeamID: plan.TeamID,
		Role:   roles[0],
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *teamRoleGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TeamRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, err := getTeamRolesAndValidate(ctx, r, state.TeamID.String(), state.Role.ResourceType.ValueStringPointer(), &[]RoleAttributesModel{state.Role}, nil)
	if err != nil {
		var roleNotFoundErr *RoleNotFoundError
		if errors.As(err, &roleNotFoundErr) {
			// Role was deleted outside Terraform - remove from state
			resp.State.RemoveResource(ctx)

			return
		}

		// Other errors (network, permissions, etc.) should fail
		resp.Diagnostics.AddError(
			"Failed to fetch and validate team roles",
			"An error occurred during the process of fetching team roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state = TeamRoleGrantModel{
		ID:     state.TeamID,
		TeamID: state.TeamID,
		Role:   roles[0],
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *teamRoleGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state *TeamRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *TeamRoleGrantModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if IsRoleChanged(plan.Role, state.Role) {
		if err := r.handleRoleUpdate(ctx, plan.Role.ResourceID, state.Role.ResourceID, plan, state); err != nil {
			resp.Diagnostics.AddError(
				"Failed to update user role",
				"An error occurred while updating user role: "+err.Error(),
			)

			return
		}

		roles, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), plan.Role.ResourceType.ValueStringPointer(), &[]RoleAttributesModel{plan.Role}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to fetch and validate user roles",
				"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
			)

			return
		}

		result := TeamRoleGrantModel{
			ID:     plan.TeamID,
			TeamID: plan.TeamID,
			Role:   roles[0],
		}

		diags = resp.State.Set(ctx, &result)
		resp.Diagnostics.Append(diags...)
	}
}

func (r *teamRoleGrantResource) handleRoleUpdate(ctx context.Context, planResourceID, stateResourceID types.String, plan, state *TeamRoleGrantModel) error {
	_, err := getTeamRolesAndValidate(ctx, r, plan.TeamID.String(), plan.Role.ResourceType.ValueStringPointer(), nil, &[]RoleAttributesModel{plan.Role})
	if err != nil {
		return err
	}

	if planResourceID == stateResourceID && plan.Role.ResourceType == state.Role.ResourceType {
		// Grant new role and revoke old role for the same resource ID
		ok, err := modifyTeamAccessControlsForResource(ctx, r.ClientWithResponsesInterface, plan.TeamID, planResourceID, plan.Role.ResourceType, &[]RoleAttributesModel{plan.Role}, &[]RoleAttributesModel{state.Role})
		if !ok || err != nil {
			return err
		}
	} else {
		// Revoke the old role for the old resource ID
		ok, err := modifyTeamAccessControlsForResource(ctx, r.ClientWithResponsesInterface, state.TeamID, stateResourceID, state.Role.ResourceType, nil, &[]RoleAttributesModel{state.Role})
		if !ok || err != nil {
			return err
		}

		// Grant the new role for the new resource ID
		ok, err = modifyTeamAccessControlsForResource(ctx, r.ClientWithResponsesInterface, plan.TeamID, planResourceID, plan.Role.ResourceType, &[]RoleAttributesModel{plan.Role}, nil)
		if !ok || err != nil {
			return err
		}
	}

	return nil
}

func (r *teamRoleGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TeamRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ok, err := modifyTeamAccessControlsForResource(ctx, r.ClientWithResponsesInterface, state.TeamID, state.Role.ResourceID, state.Role.ResourceType, nil, &[]RoleAttributesModel{state.Role})

	if !ok || err != nil {
		resp.Diagnostics.AddError(
			"Failed to revoke user role",
			"An error occurred while revoking user role: "+err.Error(),
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *teamRoleGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (r *teamRoleGrantResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *TeamRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *TeamRoleGrantModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if plan.TeamID != state.TeamID {
		resp.Diagnostics.AddError(
			"Cannot update team ID",
			"Updating the team_id is not permitted. Please explicitly delete(revoke) the granted role before changing the team_id.",
		)

		return
	}
}
