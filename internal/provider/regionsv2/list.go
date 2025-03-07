package regionsv2

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
	dataSourceListName = "regions_v2"
)

// regionsDataSourceList is the data source implementation.
type regionsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// regionsListDataSourceModel maps the data source schema data.
type regionsListDataSourceModel struct {
	ID      types.String    `tfsdk:"id"`
	Regions []regionModelV2 `tfsdk:"regions"`
}

// regionModelV2 maps regions schema data.
type regionModelV2 struct {
	Provider   types.String `tfsdk:"provider"`
	Region     types.String `tfsdk:"region"`
	RegionName types.String `tfsdk:"region_name"`
}

var _ datasource.DataSourceWithConfigure = &regionsDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &regionsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *regionsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, dataSourceListName)
}

// Schema defines the schema for the data source.
func (d *regionsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of regions(v2) that the user can access and that support workspaces. It includes the region code name and provider for each region.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"regions": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the cloud provider hosting the region.",
						},
						"region": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the region.",
						},
						"region_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The region code name.",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *regionsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	regions, err := d.GetV2RegionsWithResponse(ctx, &management.GetV2RegionsParams{})
	if serr := util.StatusOK(regions, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	result := regionsListDataSourceModel{
		ID:      types.StringValue(config.TestIDValue),
		Regions: util.Map(util.Deref(regions.JSON200), toRegionsDataSourceModel),
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *regionsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toRegionsDataSourceModel(region management.RegionV2) regionModelV2 {
	return regionModelV2{
		Provider:   types.StringValue(string(region.Provider)),
		Region:     types.StringValue(region.Region),
		RegionName: types.StringValue(region.RegionName),
	}
}
