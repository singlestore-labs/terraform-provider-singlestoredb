package roles

import (
	"context"
	"fmt"

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
	UserRoleGrantResourceName = "user_role"
)

type UserRoleGrantModel struct {
	ID     types.String        `tfsdk:"id"`
	UserID types.String        `tfsdk:"user_id"`
	Role   RoleAttributesModel `tfsdk:"role"`
}

type userRoleGrantResource struct {
	management.ClientWithResponsesInterface
}

func NewUserRoleGrantResource() resource.Resource {
	return &userRoleGrantResource{}
}

func (r *userRoleGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, UserRoleGrantResourceName)
}

func (r *userRoleGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a single role grant for a user (the 'subject' in RBAC terminology). This resource allows you to assign a specific role to a user, defining what access permission the user has to a particular resource (object) in the system. In Role-Based Access Control, this resource establishes the relationship between the subject (user), the permission level (role), and the target resource that can be accessed.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the granted role.",
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the user.",
			},
			"role": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "The role to be assigned to the user.",
				Attributes:          RoleAttributesSchema(),
			},
		},
	}
}

func (r *userRoleGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserRoleGrantModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), plan.Role.ResourceType.ValueStringPointer(), nil, &[]RoleAttributesModel{plan.Role})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	_, err = modifyUserAccessControlsForResource(ctx, r.ClientWithResponsesInterface, plan.UserID, plan.Role.ResourceID, plan.Role.ResourceType, &[]RoleAttributesModel{plan.Role}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to grant user role",
			"An error occurred while attempting to grant the specified role to the user. Details: "+err.Error(),
		)

		return
	}

	roles, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), plan.Role.ResourceType.ValueStringPointer(), &[]RoleAttributesModel{plan.Role}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state := UserRoleGrantModel{
		ID:     plan.UserID,
		UserID: plan.UserID,
		Role:   roles[0],
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *userRoleGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, err := getUserRolesAndValidate(ctx, r, state.UserID.String(), state.Role.ResourceType.ValueStringPointer(), &[]RoleAttributesModel{state.Role}, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state = UserRoleGrantModel{
		ID:     state.UserID,
		UserID: state.UserID,
		Role:   roles[0],
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *userRoleGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state *UserRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *UserRoleGrantModel
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

		roles, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), plan.Role.ResourceType.ValueStringPointer(), &[]RoleAttributesModel{plan.Role}, nil)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to fetch and validate user roles",
				"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
			)

			return
		}

		result := UserRoleGrantModel{
			ID:     plan.UserID,
			UserID: plan.UserID,
			Role:   roles[0],
		}

		diags = resp.State.Set(ctx, &result)
		resp.Diagnostics.Append(diags...)
	}
}

func (r *userRoleGrantResource) handleRoleUpdate(ctx context.Context, planResourceID, stateResourceID types.String, plan, state *UserRoleGrantModel) error {
	_, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), plan.Role.ResourceType.ValueStringPointer(), nil, &[]RoleAttributesModel{plan.Role})
	if err != nil {
		return err
	}

	if planResourceID == stateResourceID && plan.Role.ResourceType == state.Role.ResourceType {
		// Grant new role and revoke old role for the same resource ID
		_, err := modifyUserAccessControlsForResource(ctx, r.ClientWithResponsesInterface, plan.UserID, planResourceID, plan.Role.ResourceType, &[]RoleAttributesModel{plan.Role}, &[]RoleAttributesModel{state.Role})
		if err != nil {
			return err
		}
	} else {
		// Revoke the old role for the old resource ID
		_, err := modifyUserAccessControlsForResource(ctx, r.ClientWithResponsesInterface, state.UserID, stateResourceID, state.Role.ResourceType, nil, &[]RoleAttributesModel{state.Role})
		if err != nil {
			return err
		}

		// Grant the new role for the new resource ID
		_, err = modifyUserAccessControlsForResource(ctx, r.ClientWithResponsesInterface, plan.UserID, planResourceID, plan.Role.ResourceType, &[]RoleAttributesModel{plan.Role}, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *userRoleGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := modifyUserAccessControlsForResource(ctx, r.ClientWithResponsesInterface, state.UserID, state.Role.ResourceID, state.Role.ResourceType, nil, &[]RoleAttributesModel{state.Role})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to revoke user role",
			"An error occurred while revoking user role: "+err.Error(),
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *userRoleGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (r *userRoleGrantResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *UserRoleGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *UserRoleGrantModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if plan.UserID != state.UserID {
		resp.Diagnostics.AddError(
			"Cannot update user ID",
			fmt.Sprintf("Updating the user_id is not permitted. Please explicitly delete(revoke) the granted role before changing the user_id: %s != %s", plan.UserID, state.UserID),
		)

		return
	}
}
