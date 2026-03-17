package projects

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
	DataSourceListName = "projects"
)

// projectsDataSourceList is the data source implementation.
type projectsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// projectsListDataSourceModel maps the data source schema data.
type projectsListDataSourceModel struct {
	ID       types.String      `tfsdk:"id"`
	Projects []projectListItem `tfsdk:"projects"`
}

type projectListItem struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Edition   types.String `tfsdk:"edition"`
	CreatedAt types.String `tfsdk:"created_at"`
}

var _ datasource.DataSourceWithConfigure = &projectsDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &projectsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *projectsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceListName)
}

// Schema defines the schema for the data source.
func (d *projectsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of projects available to the authenticated user.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"projects": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of projects available to the user.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						config.IDAttribute: schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier of the project.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the project.",
						},
						"edition": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The edition of the project.",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The timestamp when the project was created.",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *projectsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	projectsResponse, err := d.GetV1ProjectsWithResponse(ctx)
	if serr := util.StatusOK(projectsResponse, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	result := projectsListDataSourceModel{
		ID:       types.StringValue(config.TestIDValue),
		Projects: util.Map(util.Deref(projectsResponse.JSON200), toProjectListItem),
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *projectsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toProjectListItem(project management.Project) projectListItem {
	return projectListItem{
		ID:        util.UUIDStringValue(project.ProjectID),
		Name:      types.StringValue(project.Name),
		Edition:   types.StringValue(string(project.Edition)),
		CreatedAt: types.StringValue(project.CreatedAt.UTC().Format("2006-01-02T15:04:05Z")),
	}
}
