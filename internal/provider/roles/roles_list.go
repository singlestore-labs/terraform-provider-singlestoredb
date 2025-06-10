package roles

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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
	RolesDataSourceListName = "roles"
)

type RolesModel struct {
	ID           types.String   `tfsdk:"id"`
	ResourceType types.String   `tfsdk:"resource_type"`
	ResourceID   types.String   `tfsdk:"resource_id"`
	Roles        []types.String `tfsdk:"roles"`
}

// rolesDataSourceList is the data source implementation.
type rolesDataSourceList struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &rolesDataSourceList{}

// NewRolesDataSourceList is a helper function to simplify the provider implementation.
func NewRolesDataSourceList() datasource.DataSource {
	return &rolesDataSourceList{}
}

// Metadata returns the data source type name.
func (d *rolesDataSourceList) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, RolesDataSourceListName)
}

// Schema defines the schema for the data source.
func (d *rolesDataSourceList) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source provides a list of available roles for specific resource by resource type and resource ID.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the list roles.",
			},
			"resource_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of the resource.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(ResourceTypeOrganization), string(ResourceTypeWorkspaceGroup), string(ResourceTypeTeam), string(ResourceTypeSecret)),
				},
			},
			"resource_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the resource.",
			},
			"roles": schema.ListAttribute{
				Computed:            true,
				MarkdownDescription: "A list of roles available for the specified resource type and ID.",
				ElementType:         types.StringType,
			},
		},
	}
}

// Read refreshes the Terraform state with the latest data.
func (d *rolesDataSourceList) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RolesModel
	diags := req.Config.Get(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, err := d.getRolesByResourceTypeAndID(ctx, data.ResourceType, data.ResourceID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch roles",
			fmt.Sprintf("An error occurred while fetching roles for resource type %s and ID %s: %s", data.ResourceType.ValueString(), data.ResourceID.ValueString(), err.Error()),
		)

		return
	}

	result := toRoleModel(data.ResourceType, data.ResourceID, roles)

	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func toRoleModel(resourceType, resourceID types.String, roles *[]management.ResourceRole) RolesModel {
	result := RolesModel{
		ID:           types.StringValue(config.TestIDValue),
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
	if roles == nil {
		return result
	}
	result = RolesModel{
		ID:           types.StringValue(config.TestIDValue),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Roles:        make([]types.String, len(*roles)),
	}

	for i, role := range *roles {
		result.Roles[i] = types.StringValue(role.Role)
	}

	return result
}

func (d *rolesDataSourceList) getRolesByResourceTypeAndID(ctx context.Context, resourceTypeStr, resourceIDStr types.String) (*[]management.ResourceRole, error) {
	resourceType := ResourceTypeString(resourceTypeStr)
	resourceID, err := uuid.Parse(resourceIDStr.ValueString())
	if err != nil {
		return nil, err
	}

	switch resourceType {
	case ResourceTypeOrganization:
		return d.getOrganizationRoles(ctx, resourceID)
	case ResourceTypeWorkspaceGroup:
		return d.getWorkspaceGroupRoles(ctx, resourceID)
	case ResourceTypeTeam:
		return d.getTeamRoles(ctx, resourceID)
	case ResourceTypeSecret:
		return d.getSecretRoles(ctx, resourceID)
	case ResourceTypeUnknown:
		return nil, fmt.Errorf("resource type is unknown: %s", resourceTypeStr.ValueString())
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func (d *rolesDataSourceList) getOrganizationRoles(ctx context.Context, resourceID uuid.UUID) (*[]management.ResourceRole, error) {
	response, err := d.GetV1betaOrganizationsOrganizationIDAccessControlsWithResponse(ctx, resourceID)
	if serr := util.StatusOK(response, err); serr != nil {
		return nil, serr
	}

	return response.JSON200, nil
}

func (d *rolesDataSourceList) getWorkspaceGroupRoles(ctx context.Context, resourceID uuid.UUID) (*[]management.ResourceRole, error) {
	response, err := d.GetV1betaWorkspaceGroupsWorkspaceGroupIDAccessControlsWithResponse(ctx, resourceID)
	if serr := util.StatusOK(response, err); serr != nil {
		return nil, serr
	}

	return response.JSON200, nil
}

func (d *rolesDataSourceList) getTeamRoles(ctx context.Context, resourceID uuid.UUID) (*[]management.ResourceRole, error) {
	response, err := d.GetV1betaTeamsTeamIDAccessControlsWithResponse(ctx, resourceID)
	if serr := util.StatusOK(response, err); serr != nil {
		return nil, serr
	}

	return response.JSON200, nil
}

func (d *rolesDataSourceList) getSecretRoles(ctx context.Context, resourceID uuid.UUID) (*[]management.ResourceRole, error) {
	response, err := d.GetV1betaSecretsSecretIDAccessControlsWithResponse(ctx, resourceID)
	if serr := util.StatusOK(response, err); serr != nil {
		return nil, serr
	}

	return response.JSON200, nil
}

// Configure adds the provider configured client to the data source.
func (d *rolesDataSourceList) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}
