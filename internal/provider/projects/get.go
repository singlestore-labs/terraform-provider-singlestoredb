package projects

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "project"
)

type projectDataSourceGet struct {
	management.ClientWithResponsesInterface
}

type projectDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Edition   types.String `tfsdk:"edition"`
	CreatedAt types.String `tfsdk:"created_at"`
}

var _ datasource.DataSourceWithConfigure = &projectDataSourceGet{}

func NewDataSourceGet() datasource.DataSource {
	return &projectDataSourceGet{}
}

func (d *projectDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

func (d *projectDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific project using its ID with this data source.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the project.",
				Validators:          []validator.String{util.NewUUIDValidator()},
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
	}
}

func (d *projectDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data projectDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid project ID",
			"The project ID should be a valid UUID",
		)

		return
	}

	projectResp, err := d.GetV1ProjectsProjectIDWithResponse(ctx, id)
	if serr := util.StatusOK(projectResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toProjectDataSourceModel(*projectResp.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *projectDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toProjectDataSourceModel(project management.Project) projectDataSourceModel {
	return projectDataSourceModel{
		ID:        util.UUIDStringValue(project.ProjectID),
		Name:      types.StringValue(project.Name),
		Edition:   types.StringValue(string(project.Edition)),
		CreatedAt: types.StringValue(project.CreatedAt.String()),
	}
}
