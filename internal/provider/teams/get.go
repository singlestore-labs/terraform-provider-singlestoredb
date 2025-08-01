package teams

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "team"
)

// teamsDataSourceGet is the data source implementation.
type teamsDataSourceGet struct {
	management.ClientWithResponsesInterface
}

// TeamDataSourceModel maps workspace groups schema data.
type TeamDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	MemberUsers []MemberUser `tfsdk:"member_users"`
	MemberTeams []MemberTeam `tfsdk:"member_teams"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

type MemberUser struct {
	UserID    types.String `tfsdk:"user_id"`
	Email     types.String `tfsdk:"email"`
	FirstName types.String `tfsdk:"first_name"`
	LastName  types.String `tfsdk:"last_name"`
}

type MemberTeam struct {
	TeamID      types.String `tfsdk:"team_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

var _ datasource.DataSourceWithConfigure = &teamsDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &teamsDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *teamsDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *teamsDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific team using its ID or name with this data source.",
		Attributes:          teamDataSourceSchemaAttributes(),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *teamsDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TeamDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Validate that exactly one of id or name is provided
	idProvided := !data.ID.IsNull() && !data.ID.IsUnknown()
	nameProvided := !data.Name.IsNull() && !data.Name.IsUnknown()

	if !idProvided && !nameProvided {
		resp.Diagnostics.AddError(
			"Missing team identifier",
			"Either 'id' or 'name' must be specified to identify the team.",
		)

		return
	}

	if idProvided && nameProvided {
		resp.Diagnostics.AddError(
			"Conflicting team identifiers",
			"Only one of 'id' or 'name' can be specified, not both.",
		)

		return
	}

	var team *management.Team

	if idProvided {
		team = d.teamByID(ctx, data, resp)
	} else {
		team = d.teamByName(ctx, data, resp)
	}

	if team == nil {
		return // An error was reported by helper functions.
	}

	result := toTeamDataSourceModel(*team)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// teamByID is a helper function for getting a team by ID.
// It reports the error and returns nil in case of an error.
func (d *teamsDataSourceGet) teamByID(ctx context.Context, data TeamDataSourceModel, resp *datasource.ReadResponse) *management.Team {
	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid team ID",
			"The team ID should be a valid UUID",
		)

		return nil
	}

	result, err := d.GetV1TeamsTeamIDWithResponse(ctx, id)
	if serr := util.StatusOK(result, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return nil
	}

	if result.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Team not found",
			"No team found with the specified ID: "+data.ID.ValueString(),
		)

		return nil
	}

	return result.JSON200
}

func (d *teamsDataSourceGet) teamByName(ctx context.Context, data TeamDataSourceModel, resp *datasource.ReadResponse) *management.Team {
	teams, err := d.GetV1TeamsWithResponse(ctx, &management.GetV1TeamsParams{Name: util.Ptr(data.Name.ValueString())})
	if serr := util.StatusOK(teams, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return nil
	}

	if teams.JSON200 == nil || len(*teams.JSON200) == 0 {
		resp.Diagnostics.AddError(
			"Team not found",
			"No team found with the specified name: "+data.Name.ValueString(),
		)

		return nil
	}

	if len(*teams.JSON200) > 1 {
		resp.Diagnostics.AddError(
			"Multiple teams found",
			"Multiple teams found with the specified name: "+data.Name.ValueString()+". Use ID instead for exact identification.",
		)

		return nil
	}

	result := *teams.JSON200

	return util.Ptr(result[0])
}

// Configure adds the provider configured client to the data source.
func (d *teamsDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toTeamDataSourceModel(team management.Team) TeamDataSourceModel {
	return TeamDataSourceModel{
		ID:          util.UUIDStringValue(team.TeamID),
		Name:        types.StringValue(team.Name),
		Description: types.StringValue(team.Description),
		MemberUsers: util.Map(util.Deref(team.MemberUsers), toMemberUser),
		MemberTeams: util.Map(util.Deref(team.MemberTeams), toMemberTeam),
		CreatedAt:   util.MaybeStringValue(team.CreatedAt),
	}
}

func toMemberUser(user management.UserInfo) MemberUser {
	return MemberUser{
		UserID:    util.UUIDStringValue(user.UserID),
		Email:     types.StringValue(user.Email),
		FirstName: types.StringValue(user.FirstName),
		LastName:  types.StringValue(user.LastName),
	}
}

func toMemberTeam(team management.TeamInfo) MemberTeam {
	return MemberTeam{
		TeamID:      util.UUIDStringValue(team.TeamID),
		Name:        types.StringValue(team.Name),
		Description: types.StringValue(team.Description),
	}
}

func teamDataSourceSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "The unique identifier of the team. Either `id` or `name` must be specified.",
		},
		"name": schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			MarkdownDescription: "The name of the team. Either `id` or `name` must be specified.",
		},
		"description": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The description of the team.",
		},
		"member_users": schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "List of users that are members of this team.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"user_id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The unique identifier of the user.",
					},
					"email": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The email of the user.",
					},
					"first_name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The first name of the user.",
					},
					"last_name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The last name of the user.",
					},
				},
			},
		},
		"member_teams": schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: "List of teams that are members of this team.",
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"team_id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The unique identifier of the team.",
					},
					"name": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The name of the team.",
					},
					"description": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "The description of the team.",
					},
				},
			},
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp of when the team was created.",
		},
	}
}
