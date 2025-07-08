package users

import (
	"context"
	"fmt"
	"reflect"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	ResourceName = "user"
)

var (
	_ resource.ResourceWithConfigure   = &userResource{}
	_ resource.ResourceWithModifyPlan  = &userResource{}
	_ resource.ResourceWithImportState = &userResource{}
)

// userResource is the resource implementation.
type userResource struct {
	management.ClientWithResponsesInterface
}

// userModel maps the resource schema data.
type UserModel struct {
	InvitationID types.String   `tfsdk:"id"`
	UserID       types.String   `tfsdk:"user_id"`
	Email        types.String   `tfsdk:"email"`
	Teams        []types.String `tfsdk:"teams"`
	State        types.String   `tfsdk:"state"`
	CreatedAt    types.String   `tfsdk:"created_at"`
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &userResource{}
}

// Metadata returns the resource type name.
func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

// Schema defines the schema for the resource.
func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyList, _ := types.ListValue(types.StringType, []attr.Value{})
	resp.Schema = schema.Schema{
		MarkdownDescription: "The 'apply' action sends a user an invitation to join the organization. The 'destroy' action removes a user from the organization and revokes their pending invitation(s). The 'update' action is not supported for this resource. This resource is currently in beta and may undergo changes in future releases.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the invitation.",
			},
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The email address of the user.",
			},
			"teams": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             listdefault.StaticValue(emptyList),
				MarkdownDescription: "A list of user teams associated with the invitation.",
			},
			"user_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the user. It is set when the user accepts the invitation.",
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The state of the invitation. Possible values are Pending, Accepted, Refused, or Revoked.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp when the invitation was created, in ISO 8601 format.",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	email := plan.Email.ValueString()

	var teamIDs *[]openapi_types.UUID
	if len(plan.Teams) > 0 {
		teams := make([]openapi_types.UUID, 0, len(plan.Teams))
		teamIDs = &teams
		for _, id := range plan.Teams {
			teamID, err := uuid.Parse(id.ValueString())
			if err != nil {
				resp.Diagnostics.AddAttributeError(
					path.Root("teams"),
					"Invalid team ID",
					fmt.Sprintf("The team ID %s must be a valid UUID", id.ValueString()),
				)

				return
			}
			*teamIDs = append(*teamIDs, teamID)
		}
	}

	invitationCreateResponse, err := r.PostV1betaInvitationsWithResponse(ctx, management.PostV1betaInvitationsJSONRequestBody{
		Email:   openapi_types.Email(email),
		TeamIDs: teamIDs,
	})

	if serr := util.StatusOK(invitationCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	userID, serr := tryToGetUserID(ctx, r, plan.Email.ValueString())
	if serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Error(),
		)

		return
	}

	result := toUserModel(invitationCreateResponse.JSON200, userID)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	invitation, err := r.GetV1betaInvitationsInvitationIDWithResponse(ctx,
		uuid.MustParse(state.InvitationID.ValueString()),
		&management.GetV1betaInvitationsInvitationIDParams{},
	)

	if serr := util.StatusOK(invitation, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if invitation.JSON200.State != nil && *invitation.JSON200.State == management.Revoked {
		resp.State.RemoveResource(ctx)

		return // The invitaton revoked externally, deleting it from the state file to recreate.
	}

	userID, serr := tryToGetUserID(ctx, r, state.Email.ValueString())
	if serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Error(),
		)

		return
	}

	state = toUserModel(invitation.JSON200, userID)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func tryToGetUserID(ctx context.Context, r *userResource, email string) (*openapi_types.UUID, *util.SummaryWithDetailError) {
	users, err := r.GetV1betaUsersWithResponse(ctx, &management.GetV1betaUsersParams{Email: &email})
	if serr := util.StatusOK(users, err, util.ReturnNilOnNotFound); serr != nil {
		return nil, serr
	}

	if users.JSON200 != nil && len(*users.JSON200) > 0 {
		return &(*users.JSON200)[0].UserID, nil
	}

	return nil, nil
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update is not supported for this resource.", "Update is not supported for this resource.")
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if state.UserID.IsNull() || state.UserID.IsUnknown() {
		invitationRevokeResponse, err := r.DeleteV1betaInvitationsInvitationIDWithResponse(ctx,
			uuid.MustParse(state.InvitationID.ValueString()),
		)
		if serr := util.StatusOK(invitationRevokeResponse, err); serr != nil {
			resp.Diagnostics.AddError(
				serr.Summary,
				serr.Detail,
			)

			return
		}
	} else {
		userDeleteResponse, err := r.DeleteV1betaUsersUserIDWithResponse(ctx,
			uuid.MustParse(state.UserID.ValueString()),
		)
		if serr := util.StatusOK(userDeleteResponse, err); serr != nil {
			resp.Diagnostics.AddError(
				serr.Summary,
				serr.Detail,
			)

			return
		}
	}
}

// Configure adds the provider configured client to the resource.
func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
func (r *userResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *UserModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *UserModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}

	if !plan.Email.Equal(state.Email) {
		resp.Diagnostics.AddError("Cannot update user email",
			"Updating the user email is not permitted. "+
				"Please explicitly delete(revoke) the user(invitation) before changing the user email.")

		return
	}

	if len(plan.Teams) != len(state.Teams) || !reflect.DeepEqual(plan.Teams, state.Teams) {
		resp.Diagnostics.AddError("Cannot update user teams",
			"Updating the user teams is not permitted. "+
				"Please explicitly delete(revoke) the user(invitation) before changing the user teams.")

		return
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toUserModel(userInvitation *management.UserInvitation, userID *openapi_types.UUID) UserModel {
	return UserModel{
		InvitationID: util.MaybeUUIDStringValue(userInvitation.InvitationID),
		UserID:       util.MaybeUUIDStringValue(userID),
		Email:        util.MaybeStringValue(userInvitation.Email),
		State:        util.StringValueOrNull(userInvitation.State),
		Teams:        util.MaybeUUIDStringListValue(userInvitation.TeamIDs),
		CreatedAt:    util.MaybeTimeValue(userInvitation.CreatedAt),
	}
}
