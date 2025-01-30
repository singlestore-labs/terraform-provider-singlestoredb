package privateconnections

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	DataSourceGetName = "private_connection"
)

// privateConnectionDataSourceGet is the data source implementation.
type privateConnectionDataSourceGet struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &privateConnectionDataSourceGet{}

// NewDataSourceGet is a helper function to simplify the provider implementation.
func NewDataSourceGet() datasource.DataSource {
	return &privateConnectionDataSourceGet{}
}

// Metadata returns the data source type name.
func (d *privateConnectionDataSourceGet) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, DataSourceGetName)
}

// Schema defines the schema for the data source.
func (d *privateConnectionDataSourceGet) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieve a specific private connection using its ID with this data source.",
		Attributes:          newPrivateConnectionDataSourceSchemaAttributes(),
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *privateConnectionDataSourceGet) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data privateConnectionModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := uuid.Parse(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(
			path.Root(config.IDAttribute),
			"Invalid private connection ID",
			"The private connection ID should be a valid UUID",
		)

		return
	}

	privateConnection, err := d.GetV1PrivateConnectionsConnectionIDWithResponse(ctx, id, &management.GetV1PrivateConnectionsConnectionIDParams{})
	if serr := util.StatusOK(privateConnection, err); serr != nil {
		resp.Diagnostics.AddError(
			serr.Summary,
			serr.Detail,
		)

		return
	}

	if privateConnection.JSON200.Status == nil || *privateConnection.JSON200.Status != management.PrivateConnectionStatusACTIVE {
		resp.Diagnostics.AddError(
			fmt.Sprintf("private connection with the specified ID exists, but is at the %s status", *privateConnection.JSON200.Status),
			config.ContactSupportErrorDetail,
		)

		return
	}

	result, terr := toPrivateConnectionModel(*privateConnection.JSON200)
	if terr != nil {
		resp.Diagnostics.AddError(terr.Summary, terr.Detail)

		return
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *privateConnectionDataSourceGet) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func newPrivateConnectionDataSourceSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		config.IDAttribute: schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The unique identifier of the private connection.",
			Validators:          []validator.String{util.NewUUIDValidator()},
		},
		"active_at": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The timestamp of when the private connection became active.",
		},
		"allow_list": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The private connection allow list. This is the account ID for AWS,  subscription ID for Azure, and the project name GCP.",
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
		"type": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The private connection type.",
		},
		"status": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The status of the private connection.",
		},
		"workspace_group_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the workspace group containing the private connection.",
		},
		"workspace_id": schema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "The ID of the workspace to connect with.",
		},
	}
}
