package privateconnectionsv2

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const DataSourceGetName = "private_connection_v2"

type privateConnectionDataSourceGet struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &privateConnectionDataSourceGet{}

func NewDataSourceGet() datasource.DataSource {
	return &privateConnectionDataSourceGet{}
}

func (d *privateConnectionDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

func (d *privateConnectionDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a private connection for a cluster by ID from the v2 Management API.",
		Attributes:          privateConnectionDataSourceAttributes(true),
	}
}

func (d *privateConnectionDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data privateConnectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := parseUUID(data.ID.ValueString(), "private connection ID")
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root(config.IDAttribute), err.Summary, err.Detail)

		return
	}

	// A data source lookup fails immediately. The resource refresh is what tolerates a brief 404.
	privateConnection, gerr := d.GetV2PrivateConnectionsConnectionIDWithResponse(ctx, id, &management.GetV2PrivateConnectionsConnectionIDParams{})
	if serr := util.StatusOK(privateConnection, gerr); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}
	if privateConnection.JSON200 == nil {
		resp.Diagnostics.AddError(
			"Private connection read returned an unreadable response",
			"GET /v2/privateConnections returned status code OK without a JSON private connection.",
		)

		return
	}

	result := toPrivateConnectionModel(ctx, *privateConnection.JSON200)
	resp.Diagnostics.Append(resp.State.Set(ctx, &result)...)
}

func (d *privateConnectionDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func privateConnectionDataSourceAttributes(idRequired bool) map[string]schema.Attribute {
	id := schema.StringAttribute{
		MarkdownDescription: "The unique identifier of the private connection.",
		Validators:          []validator.String{util.NewUUIDValidator()},
	}
	if idRequired {
		id.Required = true
	} else {
		id.Computed = true
	}

	return map[string]schema.Attribute{
		config.IDAttribute: id,
		"cluster_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the cluster connected with this private connection.",
		},
		"active_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp of when the private connection became active.",
		},
		"allow_list": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The private connection allow list. This is the account ID for AWS, the subscription ID for Azure, and the project name for GCP.",
		},
		"kai_endpoint_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "VPC Endpoint ID for AWS.",
		},
		"created_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp of when the private connection was created.",
		},
		"deleted_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp of when the private connection was deleted.",
		},
		"updated_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp of when the private connection was updated.",
		},
		"endpoint": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The service endpoint.",
		},
		"outbound_allow_list": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The account ID which must be allowed for outbound connections.",
		},
		"service_name": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The name of the private connection service.",
		},
		"sql_port": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "The SQL port.",
		},
		"web_socket_port": schema.Float32Attribute{
			Computed:            true,
			MarkdownDescription: "The websockets port.",
		},
		"supported_cross_regions": schema.ListAttribute{
			ElementType:         types.StringType,
			Computed:            true,
			MarkdownDescription: "Additional regions allowed to reach this private connection.",
		},
		"type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The private connection type.",
		},
		"status": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The status of the private connection.",
		},
	}
}
