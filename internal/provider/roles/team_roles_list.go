package roles

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	TeamRolesDataSourceListName = "team_roles"
)

// teamRolesDataSourceList is the data source implementation.
type teamRolesDataSourceList struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &teamRolesDataSourceList{}

// NewTeamRolesDataSourceList is a helper function to simplify the provider implementation.
func NewTeamRolesDataSourceList() datasource.DataSource {
	return &teamRolesDataSourceList{}
}

// Metadata returns the data source type name.
func (d *teamRolesDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, TeamRolesDataSourceListName)
}

// Schema defines the schema for the data source.
func (d *teamRolesDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of team roles that the team has.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the team roles.",
			},
			"team_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the team.",
			},
			"roles": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "A list of roles assigned to the team.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the role.",
						},
						"resource_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of the resource.",
						},
						"resource_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The identifier of the resource.",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *teamRolesDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TeamRolesGrantModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, err := getTeamRolesAndValidate(ctx, d, data.TeamID.String(), nil, nil, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch team roles",
			"An error occurred during the process of fetching team roles: "+err.Error(),
		)

		return
	}

	result := TeamRolesGrantModel{
		ID:     types.StringValue(config.TestIDValue),
		TeamID: data.TeamID,
		Roles:  roles,
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *teamRolesDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
