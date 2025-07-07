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
	UserRolesGrantResourceName = "user_roles"
)

type UserRolesGrantModel struct {
	ID     types.String          `tfsdk:"id"`
	UserID types.String          `tfsdk:"user_id"`
	Roles  []RoleAttributesModel `tfsdk:"roles"`
}

type userRolesGrantResource struct {
	management.ClientWithResponsesInterface
}

func NewUserRolesGrantResource() resource.Resource {
	return &userRolesGrantResource{}
}

func (r *userRolesGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, UserRolesGrantResourceName)
}

func (r *userRolesGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages role grants for a user (the 'subject' in RBAC terminology). This resource allows you to assign specific roles to a user, defining what access permissions the user has to various resources (objects) in the system. In Role-Based Access Control, this resource establishes the relationship between the subject (user), the permission level (role), and the target resources that can be accessed.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the granted roles.",
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the user.",
			},
			"roles": schema.ListNestedAttribute{
				Required:            true,
				MarkdownDescription: "A list of roles assigned to the user.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: RoleAttributesSchema(),
				},
			},
		},
	}
}

func (r *userRolesGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserRolesGrantModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), nil, nil, &plan.Roles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	_, err = grantUserRoles(ctx, r.ClientWithResponsesInterface, plan.UserID, plan.Roles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to grant user roles",
			"An error occurred while granting user roles: "+err.Error(),
		)

		return
	}

	roles, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), nil, &plan.Roles, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state := toUserRolesGrantModel(plan.UserID, roles)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *userRolesGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, err := getUserRolesAndValidate(ctx, r, state.UserID.String(), nil, &state.Roles, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	state = toUserRolesGrantModel(state.UserID, roles)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *userRolesGrantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state *UserRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *UserRolesGrantModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	grantRoles := SubtractRoles(plan.Roles, state.Roles)
	revokeRoles := SubtractRoles(state.Roles, plan.Roles)

	_, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), nil, nil, &grantRoles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	_, err = r.doUpdate(ctx, plan.UserID, grantRoles, revokeRoles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update user roles",
			"An error occurred while updating user roles: "+err.Error(),
		)

		return
	}

	roles, err := getUserRolesAndValidate(ctx, r, plan.UserID.String(), nil, &plan.Roles, &revokeRoles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	result := toUserRolesGrantModel(plan.UserID, roles)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *userRolesGrantResource) doUpdate(ctx context.Context, userID types.String, grantRoles, revokeRoles []RoleAttributesModel) (bool, error) {
	ok, err := revokeUserRoles(ctx, r.ClientWithResponsesInterface, userID, revokeRoles)
	if !ok || err != nil {
		return ok, err
	}

	return grantUserRoles(ctx, r.ClientWithResponsesInterface, userID, grantRoles)
}

func (r *userRolesGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := revokeUserRoles(ctx, r.ClientWithResponsesInterface, state.UserID, state.Roles)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to revoke user roles",
			"An error occurred while revoking user roles: "+err.Error(),
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *userRolesGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (r *userRolesGrantResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *UserRolesGrantModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *UserRolesGrantModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if plan.UserID != state.UserID {
		resp.Diagnostics.AddError(
			"Cannot update user ID",
			"Updating the user_id is not permitted. Please explicitly delete(revoke) the granted roles before changing the user_id.",
		)

		return
	}
}

func toUserRolesGrantModel(userID types.String, roles []RoleAttributesModel) UserRolesGrantModel {
	if roles == nil {
		roles = []RoleAttributesModel{}
	}

	return UserRolesGrantModel{
		ID:     userID,
		UserID: userID,
		Roles:  roles,
	}
}
