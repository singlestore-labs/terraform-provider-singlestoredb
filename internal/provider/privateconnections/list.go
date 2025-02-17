package privateconnections

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
	DataSourceListName = "private_connections"
)

// privateConnectionsDataSourceList is the data source implementation.
type privateConnectionsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// privateConnectionsListDataSourceModel maps the data source schema data.
type privateConnectionsListDataSourceModel struct {
	ID                 types.String             `tfsdk:"id"`
	WorkspaceGroupID   types.String             `tfsdk:"workspace_group_id"`
	PrivateConnections []PrivateConnectionModel `tfsdk:"private_connections"`
}

var _ datasource.DataSourceWithConfigure = &privateConnectionsDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &privateConnectionsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *privateConnectionsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceListName)
}

// Schema defines the schema for the data source.
func (d *privateConnectionsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of privateConnections that the user has access to.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			config.WorkspaceGroupIDAttribute: schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the workspace group.",
			},
			DataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: newPrivateConnectionDataSourceSchemaAttributes(),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *privateConnectionsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data privateConnectionsListDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.WorkspaceGroupID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.WorkspaceGroupIDAttribute),
			"Invalid workspace group ID",
			"The workspace group ID should be a valid UUID",
		)

		return
	}

	privateConnections, err := d.GetV1WorkspaceGroupsWorkspaceGroupIDPrivateConnectionsWithResponse(ctx, id, &management.GetV1WorkspaceGroupsWorkspaceGroupIDPrivateConnectionsParams{})
	if serr := util.StatusOK(privateConnections, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}
	resultPrivateConnections, merr := util.MapWithError(util.Deref(privateConnections.JSON200), toPrivateConnectionModel)
	if merr != nil {
		resp.Diagnostics.AddError(merr.Summary, merr.Detail)

		return
	}

	result := privateConnectionsListDataSourceModel{
		ID:                 types.StringValue(config.TestIDValue),
		WorkspaceGroupID:   data.WorkspaceGroupID,
		PrivateConnections: resultPrivateConnections,
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *privateConnectionsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
