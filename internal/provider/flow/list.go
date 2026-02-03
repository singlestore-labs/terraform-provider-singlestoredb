package flow

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
	DataSourceListName = "flow_instances"
)

// flowInstancesDataSourceList is the data source implementation.
type flowInstancesDataSourceList struct {
	management.ClientWithResponsesInterface
}

// flowInstancesListDataSourceModel maps the data source schema data.
type flowInstancesListDataSourceModel struct {
	ID            types.String                  `tfsdk:"id"`
	FlowInstances []flowInstanceDataSourceModel `tfsdk:"flow_instances"`
}

var _ datasource.DataSourceWithConfigure = &flowInstancesDataSourceList{}

// NewDataSourceList is a helper function to simplify the provider implementation.
func NewDataSourceList() datasource.DataSource {
	return &flowInstancesDataSourceList{}
}

// Metadata returns the data source type name.
func (d *flowInstancesDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceListName)
}

// Schema defines the schema for the data source.
func (d *flowInstancesDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of Flow instances that the user has access to.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			DataSourceListName: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: newFlowInstanceDataSourceSchemaAttributes(flowInstanceDataSourceSchemaConfig{
						computeFlowInstanceID: true,
						computedName:          true,
					}),
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *flowInstancesDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data flowInstancesListDataSourceModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	flowInstances, err := d.GetV1FlowWithResponse(ctx, &management.GetV1FlowParams{})
	if serr := util.StatusOK(flowInstances, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	resultFlowInstances, merr := util.MapWithError(util.Deref(flowInstances.JSON200), toFlowInstanceDataSourceModel)
	if merr != nil {
		resp.Diagnostics.AddError(merr.Summary, merr.Detail)

		return
	}

	result := flowInstancesListDataSourceModel{
		ID:            types.StringValue(config.TestIDValue),
		FlowInstances: resultFlowInstances,
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *flowInstancesDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
