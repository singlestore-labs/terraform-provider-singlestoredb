package regions

import (
	"context"

	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	dataSourceName = "regions"
)

// regionsDataSource is the data source implementation.
type regionsDataSource struct{}

// regionsDataSourceModel maps the data source schema data.
type regionsDataSourceModel struct {
	Regions []regionsModel `tfsdk:"regions"`
}

// regionsModel maps regions schema data.
type regionsModel struct {
	ID types.Int64 `tfsdk:"id"`
}

var _ datasource.DataSource = &regionsDataSource{}

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
			dataSourceName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *regionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	result := regionsDataSourceModel{
		Regions: []regionsModel{
			{ID: types.Int64Value(int64(1))},
			{ID: types.Int64Value(int64(2))},
			{ID: types.Int64Value(int64(3))},
		},
	}
	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}
