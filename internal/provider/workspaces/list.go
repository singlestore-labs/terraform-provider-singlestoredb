package workspaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
)

const (
	dataSourceListName = "workspaces"
)

// workspacesDataSourceList is the data source implementation.
type workspacesDataSourceList struct {
	management.ClientWithResponsesInterface
}

// workspacesListDataSourceModel maps the data source schema data.
type workspacesListDataSourceModel struct {
	ID               types.String               `tfsdk:"id"`
	WorkspaceGroupID types.String               `tfsdk:"workspace_group_id"`
	Workspaces       []workspaceDataSourceModel `tfsdk:"workspaces"`
}

var _ datasource.DataSourceWithConfigure = &workspacesDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &workspacesDataSourceList{}
}

// Metadata returns the data source type name.
func (d *workspacesDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, dataSourceListName)
}

// Schema defines the schema for the data source.
func (d *workspacesDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"workspace_group_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the workspace group",
			},
			dataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: newWorkspaceDataSourceSchemaAttributes(workspaceDataSourceSchemaConfig{
						computeWorkspaceID: true,
					}),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *workspacesDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workspacesListDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.WorkspaceGroupID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root("workspace_group_id"),
			"Invalid workspace group ID",
			"The workspace group ID should be a valid UUID",
		)

		return
	}

	workspaces, err := d.GetV1WorkspacesWithResponse(ctx, &management.GetV1WorkspacesParams{
		WorkspaceGroupID: id,
	})
	if serr := util.StatusOK(workspaces, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	resultWorkspaces, merr := util.MapWithError(util.Deref(workspaces.JSON200), toWorkspaceDataSourceModel)
	if merr != nil {
		resp.Diagnostics.AddError(merr.Summary, merr.Detail)

		return
	}

	result := workspacesListDataSourceModel{
		ID:               types.StringValue(config.TestIDValue),
		WorkspaceGroupID: data.WorkspaceGroupID,
		Workspaces:       resultWorkspaces,
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *workspacesDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
