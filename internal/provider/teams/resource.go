package teams

import (
	"context"
	"fmt"

	otypes "github.com/deepmap/oapi-codegen/pkg/types"
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
	ResourceName = "team"
)

var (
	_ resource.ResourceWithConfigure   = &teamResource{}
	_ resource.ResourceWithModifyPlan  = &teamResource{}
	_ resource.ResourceWithImportState = &teamResource{}
)

type TeamResourceModel struct {
	ID          types.String   `tfsdk:"id"`
	Name        types.String   `tfsdk:"name"`
	Description types.String   `tfsdk:"description"`
	MemberUsers []types.String `tfsdk:"member_users"`
	MemberTeams []types.String `tfsdk:"member_teams"`
	CreatedAt   types.String   `tfsdk:"created_at"`
}

// teamResource is the resource implementation.
type teamResource struct {
	management.ClientWithResponsesInterface
}

// NewResource is a helper function to simplify the provider implementation.
func NewResource() resource.Resource {
	return &teamResource{}
}

// Metadata returns the resource type name.
func (r *teamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, ResourceName)
}

// Schema defines the schema for the resource.
func (r *teamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyList, _ := types.ListValue(types.StringType, []attr.Value{})
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage SingleStoreDB teams with this resource.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Computed:            true,
				MarkdownDescription: "The unique identifier of the team.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the team.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The description of the team.",
			},
			"member_users": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             listdefault.StaticValue(emptyList),
				MarkdownDescription: "List of user emails that are members of this team.",
			},
			"member_teams": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             listdefault.StaticValue(emptyList),
				MarkdownDescription: "List of team UUIDs that are members of this team.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp of when the team was created.",
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *teamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TeamResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamCreateResponse, err := r.PostV1TeamsWithResponse(ctx, management.PostV1TeamsJSONRequestBody{
		Name:        util.ToString(plan.Name),
		Description: util.MaybeString(plan.Description),
	})

	if serr := util.StatusOK(teamCreateResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	id := teamCreateResponse.JSON200.TeamID

	memberEmails, err := validateAndMapUserEmails(plan.MemberUsers)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("member_users"),
			"Invalid user email",
			err.Error(),
		)

		return
	}

	teamIDs, err := util.ParseUUIDList(plan.MemberTeams)
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("member_teams"),
			"Invalid team ID",
			err.Error(),
		)

		return
	}

	if (memberEmails != nil && len(*memberEmails) > 0) || teamIDs != nil {
		teamPatchResponse, err := r.PatchV1TeamsTeamIDWithResponse(ctx,
			id,
			management.PatchV1TeamsTeamIDJSONRequestBody{
				AddMemberUserEmails: memberEmails,
				AddMemberTeamIDs:    teamIDs,
			},
		)

		if serr := util.StatusOK(teamPatchResponse, err); serr != nil {
			resp.Diagnostics.AddError(
				serr.Summary,
				serr.Detail,
			)

			return
		}
	}

	team, err := r.GetV1TeamsTeamIDWithResponse(ctx, id)

	if serr := util.StatusOK(team, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	result := toTeamResourceModel(*team.JSON200)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *teamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TeamResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	team, err := r.GetV1TeamsTeamIDWithResponse(ctx, uuid.MustParse(state.ID.ValueString()))

	if serr := util.StatusOK(team, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	state = toTeamResourceModel(*team.JSON200)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *teamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state TeamResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan TeamResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := uuid.MustParse(state.ID.ValueString())

	addedUsers, removedUsers, addedTeams, removedTeams := r.parseUserAndTeamIds(resp, state, plan)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.shouldUpdate(state, plan, addedUsers, removedUsers, addedTeams, removedTeams) {
		teamPatchResponse, err := r.PatchV1TeamsTeamIDWithResponse(ctx, id, management.PatchV1TeamsTeamIDJSONRequestBody{
			Name:                   util.MaybeString(plan.Name),
			Description:            util.MaybeString(plan.Description),
			AddMemberUserEmails:    addedUsers,
			RemoveMemberUserEmails: removedUsers,
			AddMemberTeamIDs:       addedTeams,
			RemoveMemberTeamIDs:    removedTeams,
		})
		if serr := util.StatusOK(teamPatchResponse, err); serr != nil {
			resp.Diagnostics.AddError(
				serr.Summary,
				serr.Detail,
			)

			return
		}
	}

	team, err := r.GetV1TeamsTeamIDWithResponse(ctx, id)
	if serr := util.StatusOK(team, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toTeamResourceModel(*team.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *teamResource) parseUserAndTeamIds(resp *resource.UpdateResponse, state, plan TeamResourceModel) (*[]string, *[]string, *[]otypes.UUID, *[]otypes.UUID) {
	addedUsers, err := validateAndMapUserEmails(util.SubtractListValues(plan.MemberUsers, state.MemberUsers))
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_users"), "Invalid user email", err.Error())
	}

	removedUsers, err := validateAndMapUserEmails(util.SubtractListValues(state.MemberUsers, plan.MemberUsers))
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_users"), "Invalid user email", err.Error())
	}

	addedTeams, err := util.ParseUUIDList(util.SubtractListValues(plan.MemberTeams, state.MemberTeams))
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_teams"), "Invalid team ID", err.Error())
	}

	removedTeams, err := util.ParseUUIDList(util.SubtractListValues(state.MemberTeams, plan.MemberTeams))
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_teams"), "Invalid team ID", err.Error())
	}

	return addedUsers, removedUsers, addedTeams, removedTeams
}

func validateAndMapUserEmails(emails []types.String) (*[]string, error) {
	validEmails := make([]string, 0, len(emails))
	for _, email := range emails {
		if !util.IsValidEmail(email.ValueString()) {
			return nil, fmt.Errorf("invalid email address: %s", email.ValueString())
		}
		validEmails = append(validEmails, email.ValueString())
	}

	return &validEmails, nil
}

func (r *teamResource) shouldUpdate(state, plan TeamResourceModel, addedUsers, removedUsers *[]string, addedTeams, removedTeams *[]otypes.UUID) bool {
	return addedUsers != nil || removedUsers != nil || addedTeams != nil || removedTeams != nil || plan.Name != state.Name || plan.Description != state.Description
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *teamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TeamResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	teamDeleteResponse, err := r.DeleteV1TeamsTeamIDWithResponse(ctx,
		uuid.MustParse(state.ID.ValueString()),
	)
	if serr := util.StatusOK(teamDeleteResponse, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
}

// Configure adds the provider configured client to the resource.
func (r *teamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

// ModifyPlan emits an error if a required yet immutable field changes or if incompatible state is set.
func (r *teamResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	var state *TeamResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state == nil {
		return
	}

	var plan *TeamResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || plan == nil {
		return
	}
}

// ImportState results in Terraform managing the resource that was not previously managed.
func (r *teamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root(config.IDAttribute), req, resp)
}

func toTeamResourceModel(team management.Team) TeamResourceModel {
	return TeamResourceModel{
		ID:          util.UUIDStringValue(team.TeamID),
		Name:        types.StringValue(team.Name),
		Description: types.StringValue(team.Description),
		CreatedAt:   util.MaybeStringValue(team.CreatedAt),
		MemberUsers: toUsersEmailList(team.MemberUsers),
		MemberTeams: toTeamsUUIDList(team.MemberTeams),
	}
}

func toUsersEmailList(userList *[]management.UserInfo) []types.String {
	if userList == nil {
		return []types.String{}
	}

	users := make([]types.String, len(*userList))
	for i, user := range *userList {
		users[i] = types.StringValue(user.Email)
	}

	return users
}

func toTeamsUUIDList(userList *[]management.TeamInfo) []types.String {
	if userList == nil {
		return []types.String{}
	}

	users := make([]types.String, len(*userList))
	for i, user := range *userList {
		users[i] = util.UUIDStringValue(user.TeamID)
	}

	return users
}
