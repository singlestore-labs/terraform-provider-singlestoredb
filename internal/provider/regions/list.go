package regions

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
)

const (
	dataSourceListName = "regions"
)

// regionsDataSourceList is the data source implementation.
type regionsDataSourceList struct {
	management.ClientWithResponsesInterface
}

// regionsListDataSourceModel maps the data source schema data.
type regionsListDataSourceModel struct {
	ID      types.String  `tfsdk:"id"`
	Regions []regionModel `tfsdk:"regions"`
}

// regionModel maps regions schema data.
type regionModel struct {
	ID       types.String `tfsdk:"id"`
	Provider types.String `tfsdk:"provider"`
	Region   types.String `tfsdk:"region"`
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
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			dataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						config.IDAttribute: schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "ID of the region",
						},
						"provider": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Name of the provider",
						},
						"region": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Name of the region",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *regionsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	regions, err := d.GetV1RegionsWithResponse(ctx, &management.GetV1RegionsParams{})
	if err != nil {
		resp.Diagnostics.AddError(
			"SingleStore API client failed to list regions",
			"An unexpected error occurred when calling SingleStore API regions. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client error: "+err.Error(),
		)

		return
	}

	if code := regions.StatusCode(); code != http.StatusOK {
		resp.Diagnostics.AddError(
			fmt.Sprintf("SingleStore API client returned status code %s while listing regions", http.StatusText(code)),
			"An unsuccessful status code occurred when calling SingleStore API regions. "+
				fmt.Sprintf("Make sure to set the %s value in the configuration or use the %s environment variable. ", config.APIKeyAttribute, config.EnvAPIKey)+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client response body: "+string(regions.Body),
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

func toRegionsDataSourceModel(region management.Region) regionModel {
	return regionModel{
		ID:       types.StringValue(region.RegionID.String()),
		Provider: types.StringValue(string(region.Provider)),
		Region:   types.StringValue(region.Region),
	}
}
