package roles

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
	UserRolesDataSourceListName = "user_roles"
)

// userRolesDataSourceList is the data source implementation.
type userRolesDataSourceList struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &userRolesDataSourceList{}

// NewUserRolesDataSourceList is a helper function to simplify the provider implementation.
func NewUserRolesDataSourceList() datasource.DataSource {
	return &userRolesDataSourceList{}
}

// Metadata returns the data source type name.
func (d *userRolesDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, UserRolesDataSourceListName)
}

// Schema defines the schema for the data source.
func (d *userRolesDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of user roles that the user has.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the user.",
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The unique identifier of the user.",
			},
			"roles": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "A list of roles assigned to the user.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the role.",
						},
						"resource_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The type of the resource.",
						},
						"resource_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The identifier of the resource.",
						},
					},
				},
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *userRolesDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserRolesGrantModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, err := getUserRolesAndValidate(ctx, d, data.UserID.String(), nil, nil, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch and validate user roles",
			"An error occurred during the process of fetching user roles or validating them afterward: "+err.Error(),
		)

		return
	}

	result := UserRolesGrantModel{
		ID:     types.StringValue(config.TestIDValue),
		UserID: data.UserID,
		Roles:  roles,
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

// Configure adds the provider configured client to the data source.
func (d *userRolesDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
