package teams

import (
	"context"
	"net/http"

	otypes "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
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
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	MemberUsers types.Set    `tfsdk:"member_users"`
	MemberTeams types.Set    `tfsdk:"member_teams"`
	CreatedAt   types.String `tfsdk:"created_at"`
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
	emptySet, _ := types.SetValue(types.StringType, []attr.Value{})
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage SingleStoreDB teams with this resource. The 'apply' action creates a new team or updates an existing one. You can add/remove users to/from the team by specifying their email addresses in the 'member_users' set. You can also add/remove other teams to/from this team by specifying their IDs in the 'member_teams' set. The 'destroy' action deletes the team. Updating the 'member_users' or 'member_teams' sets will add or remove the corresponding users or teams from the team.",
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
			"member_users": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(emptySet),
				MarkdownDescription: "Set of user emails that are members of this team.",
			},
			"member_teams": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             setdefault.StaticValue(emptySet),
				MarkdownDescription: "Set of team UUIDs that are members of this team.",
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

	if !r.addInitialMembers(ctx, &resp.Diagnostics, id, plan) {
		return
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

// addInitialMembers adds the members specified in the plan to a newly created team.
// Returns false if the caller should return due to an error.
func (r *teamResource) addInitialMembers(ctx context.Context, diags *diag.Diagnostics, id otypes.UUID, plan TeamResourceModel) bool {
	emptyStrings := types.SetNull(types.StringType)

	memberEmails, err := util.ValidateUserEmailDiff(ctx, plan.MemberUsers, emptyStrings, diags)
	if err != nil {
		diags.AddAttributeError(path.Root("member_users"), "Invalid user email", err.Error())

		return false
	}

	teamIDs, err := util.ValidateUUIDDiff(ctx, plan.MemberTeams, emptyStrings, diags)
	if err != nil {
		diags.AddAttributeError(path.Root("member_teams"), "Invalid team ID", err.Error())

		return false
	}

	if diags.HasError() {
		return false
	}

	if len(memberEmails) == 0 && len(teamIDs) == 0 {
		return true
	}

	teamPatchResponse, err := r.PatchV1TeamsTeamIDWithResponse(ctx, id, management.PatchV1TeamsTeamIDJSONRequestBody{
		AddMemberUserEmails: &memberEmails,
		AddMemberTeamIDs:    &teamIDs,
	})
	if serr := util.StatusOK(teamPatchResponse, err); serr != nil {
		diags.AddError(serr.Summary, serr.Detail)

		return false
	}

	return true
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

	if serr := util.StatusOK(team, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if team.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)

		return // The resource got terminated externally, deleting it from the state file to recreate.
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

	addedUsers, removedUsers, addedTeams, removedTeams := parseUserAndTeamIds(ctx, resp, state, plan)
	if resp.Diagnostics.HasError() {
		return
	}

	if shouldUpdate(state, plan, addedUsers, removedUsers, addedTeams, removedTeams) {
		teamPatchResponse, err := r.PatchV1TeamsTeamIDWithResponse(ctx, id, management.PatchV1TeamsTeamIDJSONRequestBody{
			Name:                   util.MaybeString(plan.Name),
			Description:            util.MaybeString(plan.Description),
			AddMemberUserEmails:    &addedUsers,
			RemoveMemberUserEmails: &removedUsers,
			AddMemberTeamIDs:       &addedTeams,
			RemoveMemberTeamIDs:    &removedTeams,
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

func parseUserAndTeamIds(ctx context.Context, resp *resource.UpdateResponse, state, plan TeamResourceModel) ([]string, []string, []otypes.UUID, []otypes.UUID) {
	addedUsers, err := util.ValidateUserEmailDiff(ctx, plan.MemberUsers, state.MemberUsers, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_users"), "Invalid user email", err.Error())
	}

	removedUsers, err := util.ValidateUserEmailDiff(ctx, state.MemberUsers, plan.MemberUsers, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_users"), "Invalid user email", err.Error())
	}

	addedTeams, err := util.ValidateUUIDDiff(ctx, plan.MemberTeams, state.MemberTeams, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_teams"), "Invalid team ID", err.Error())
	}

	removedTeams, err := util.ValidateUUIDDiff(ctx, state.MemberTeams, plan.MemberTeams, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("member_teams"), "Invalid team ID", err.Error())
	}

	return addedUsers, removedUsers, addedTeams, removedTeams
}

func shouldUpdate(state, plan TeamResourceModel, addedUsers, removedUsers []string, addedTeams, removedTeams []otypes.UUID) bool {
	return len(addedUsers) > 0 || len(removedUsers) > 0 || len(addedTeams) > 0 || len(removedTeams) > 0 || plan.Name != state.Name || plan.Description != state.Description
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
	if serr := util.StatusOK(teamDeleteResponse, err, util.ReturnNilOnNotFound); serr != nil {
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
	util.ImportStatePassthroughID(ctx, req, resp)
}

func toTeamResourceModel(team management.Team) TeamResourceModel {
	return TeamResourceModel{
		ID:          util.UUIDStringValue(team.TeamID),
		Name:        types.StringValue(team.Name),
		Description: types.StringValue(team.Description),
		CreatedAt:   util.MaybeStringValue(team.CreatedAt),
		MemberUsers: toUsersEmailSet(team.MemberUsers),
		MemberTeams: toTeamsUUIDSet(team.MemberTeams),
	}
}

func toUsersEmailSet(userList *[]management.UserInfo) types.Set {
	if userList == nil {
		return types.SetValueMust(types.StringType, []attr.Value{})
	}

	seen := make(map[string]struct{}, len(*userList))
	elems := make([]attr.Value, 0, len(*userList))
	for _, user := range *userList {
		if _, ok := seen[user.Email]; ok {
			continue
		}
		seen[user.Email] = struct{}{}
		elems = append(elems, types.StringValue(user.Email))
	}

	return types.SetValueMust(types.StringType, elems)
}

func toTeamsUUIDSet(teamList *[]management.TeamInfo) types.Set {
	if teamList == nil {
		return types.SetValueMust(types.StringType, []attr.Value{})
	}

	seen := make(map[otypes.UUID]struct{}, len(*teamList))
	elems := make([]attr.Value, 0, len(*teamList))
	for _, t := range *teamList {
		if _, ok := seen[t.TeamID]; ok {
			continue
		}
		seen[t.TeamID] = struct{}{}
		elems = append(elems, util.UUIDStringValue(t.TeamID))
	}

	return types.SetValueMust(types.StringType, elems)
}
