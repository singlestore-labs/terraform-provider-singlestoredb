package roles

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	PermissionsDataSourceListName = "role_permissions"
	ownerRoleName                 = "Owner"
)

type PermissionsModel struct {
	ID           types.String   `tfsdk:"id"`
	ResourceType types.String   `tfsdk:"resource_type"`
	Permissions  []types.String `tfsdk:"permissions"`
}

// permissionsDataSourceList is the data source implementation.
type permissionsDataSourceList struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &permissionsDataSourceList{}

// NewPermissionsDataSourceList is a helper function to simplify the provider implementation.
func NewPermissionsDataSourceList() datasource.DataSource {
	return &permissionsDataSourceList{}
}

// Metadata returns the data source type name.
func (d *permissionsDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, PermissionsDataSourceListName)
}

// Schema defines the schema for the data source.
func (d *permissionsDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides the list of all available permissions for a given resource type. It returns the permissions of the built-in Owner role, which has full access and therefore represents the complete set of permissions available for the resource type. Use this data source to discover valid permission names when creating custom roles.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the data source.",
			},
			"resource_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of the resource for which to list available permissions. " + formatResourceTypeList(),
				Validators: []validator.String{
					stringvalidator.OneOf(string(ResourceTypeOrganization), string(ResourceTypeWorkspaceGroup), string(ResourceTypeTeam), string(ResourceTypeSecret)),
				},
			},
			"permissions": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The list of all available permission names for the specified resource type.",
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *permissionsDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PermissionsModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := data.ResourceType.ValueString()

	rolesResp, err := d.GetV1RolesResourceTypeWithResponse(ctx, resourceType)
	if serr := util.StatusOK(rolesResp, err); serr != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch roles",
			fmt.Sprintf("An error occurred while fetching roles for resource type %s: %s", resourceType, serr.Detail),
		)

		return
	}

	permissions, err := extractOwnerPermissions(rolesResp.JSON200)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to find Owner role",
			fmt.Sprintf("Could not find the Owner role for resource type %s: %s", resourceType, err.Error()),
		)

		return
	}

	result := PermissionsModel{
		ID:           types.StringValue(fmt.Sprintf("permissions-%s", resourceType)),
		ResourceType: data.ResourceType,
		Permissions:  convertPermissions(permissions),
	}

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func extractOwnerPermissions(roles *[]management.RoleDefinition) ([]string, error) {
	if roles == nil {
		return nil, fmt.Errorf("no roles returned by the API")
	}

	for _, role := range *roles {
		if role.Role == ownerRoleName {
			return role.Permissions, nil
		}
	}

	return nil, fmt.Errorf("the Owner role was not found among the returned roles")
}

// Configure adds the provider configured client to the data source.
func (d *permissionsDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
