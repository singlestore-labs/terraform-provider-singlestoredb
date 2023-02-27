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
	dataSourceName = "regions"
)

// regionsDataSource is the data source implementation.
type regionsDataSource struct {
	management.ClientWithResponsesInterface
}

// regionsDataSourceModel maps the data source schema data.
type regionsDataSourceModel struct {
	Regions []regionsModel `tfsdk:"regions"`
	ID      types.String   `tfsdk:"id"`
}

// regionsModel maps regions schema data.
type regionsModel struct {
	Provider types.String `tfsdk:"provider"`
	Region   types.String `tfsdk:"region"`
	RegionID types.String `tfsdk:"region_id"`
}

var _ datasource.DataSourceWithConfigure = &regionsDataSource{}

// NewDataSource is a helper function to simplify the provider implementation.
func NewDataSource() datasource.DataSource {
	return &regionsDataSource{}
}

// Metadata returns the data source type name.
func (d *regionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, dataSourceName)
}

// Schema defines the schema for the data source.
func (d *regionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			config.TestIDAttribute: schema.StringAttribute{
				Computed: true,
			},
			dataSourceName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Name of the provider",
						},
						"region": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Name of the region",
						},
						"region_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "ID of the region",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *regionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
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

	code := regions.StatusCode()
	if code != http.StatusOK {
		resp.Diagnostics.AddError(
			fmt.Sprintf("SingleStore API client returned status code %s while listing regions", http.StatusText(code)),
			"An unsucessfull status code occurred when calling SingleStore API regions. "+
				fmt.Sprintf("Make sure to set the %s value in the configuration or use the %s environment variable. ", config.APIKeyAttribute, config.EnvAPIKey)+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"SingleStore client response body: "+string(regions.Body),
		)

		return

	}

	result := regionsDataSourceModel{
		ID: types.StringValue(config.TestIDValue),
	}

	for _, region := range util.Deref(regions.JSON200) {
		result.Regions = append(result.Regions, regionsModel{
			Provider: types.StringValue(string(region.Provider)),
			Region:   types.StringValue(region.Region),
			RegionID: types.StringValue(region.RegionID.String()),
		})
	}

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *regionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
