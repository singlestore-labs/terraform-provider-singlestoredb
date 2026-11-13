package privateconnectionsv2

import (
	"context"
	"fmt"

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
	DataSourceListName        = "private_connections_v2"
	privateConnectionsAttrKey = "private_connections"
)

type privateConnectionsDataSourceList struct {
	management.ClientWithResponsesInterface
}

type privateConnectionsListModel struct {
	ID                 types.String             `tfsdk:"id"`
	ClusterID          types.String             `tfsdk:"cluster_id"`
	PrivateConnections []privateConnectionModel `tfsdk:"private_connections"`
}

var _ datasource.DataSourceWithConfigure = &privateConnectionsDataSourceList{}

func NewDataSourceList() datasource.DataSource {
	return &privateConnectionsDataSourceList{}
}

func (d *privateConnectionsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceListName)
}

func (d *privateConnectionsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "List the private connections on a cluster from the v2 Management API.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed: true,
			},
			"cluster_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The ID of the cluster whose private connections are listed.",
				Validators:          []validator.String{util.NewUUIDValidator()},
			},
			privateConnectionsAttrKey: schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: privateConnectionDataSourceAttributes(false),
				},
			},
		},
	}
}

func (d *privateConnectionsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data privateConnectionsListModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := parseUUID(data.ClusterID.ValueString(), "cluster ID")
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("cluster_id"), err.Summary, err.Detail)

		return
	}

	privateConnections, gerr := d.GetV2ClustersClusterIDPrivateConnectionsWithResponse(ctx, id, &management.GetV2ClustersClusterIDPrivateConnectionsParams{})
	if serr := util.StatusOK(privateConnections, gerr); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}
	if privateConnections.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Private connections list returned an unreadable response",
			fmt.Sprintf("GET /v2/clusters/%s/privateConnections returned status code OK without a JSON list of private connections.", id),
		)

		return
	}

	result := privateConnectionsListModel{
		ID:        types.StringValue(config.TestIDValue),
		ClusterID: data.ClusterID,
		PrivateConnections: util.Map(*privateConnections.JSON200, func(conn management.ClusterPrivateConnection) privateConnectionModel {
			return toPrivateConnectionModel(ctx, conn)
		}),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func (d *privateConnectionsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
