package clusters

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceListName = "clusters"
)

// clustersDataSourceList is the data source implementation.
type clustersDataSourceList struct {
	management.ClientWithResponsesInterface
}

// clustersListDataSourceModel maps the data source schema data.
type clustersListDataSourceModel struct {
	ID                types.String             `tfsdk:"id"`
	ProjectID         types.String             `tfsdk:"project_id"`
	IncludeTerminated types.Bool               `tfsdk:"include_terminated"`
	Clusters          []clusterDataSourceModel `tfsdk:"clusters"`
}

var _ datasource.DataSourceWithConfigure = &clustersDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &clustersDataSourceList{}
}

// Metadata returns the data source type name.
func (d *clustersDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceListName)
}

// Schema defines the schema for the data source.
func (d *clustersDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of clusters that the user has access to.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter clusters by project ID.",
				Validators:          []validator.String{util.NewUUIDValidator()},
			},
			"include_terminated": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to include terminated clusters in the result. Defaults to false.",
			},
			DataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: newClusterDataSourceSchemaAttributes(clusterDataSourceSchemaConfig{
						computeClusterID: true,
						computeName:      true,
					}),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *clustersDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data clustersListDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	params := &management.GetV2ClustersParams{
		IncludeTerminated: util.MaybeBool(data.IncludeTerminated),
	}
	if util.IsConfiguredString(data.ProjectID) {
		params.ProjectID = util.Ptr(uuid.MustParse(data.ProjectID.ValueString()))
	}

	clusters, err := d.GetV2ClustersWithResponse(ctx, params)
	if serr := util.StatusOK(clusters, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := clustersListDataSourceModel{
		ID:                types.StringValue(config.TestIDValue),
		ProjectID:         data.ProjectID,
		IncludeTerminated: data.IncludeTerminated,
		Clusters:          util.Map(util.Deref(clusters.JSON200), toClusterDataSourceModel),
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *clustersDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
