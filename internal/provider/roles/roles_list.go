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

type RoleDefinitionModel struct {
	Name         types.String       `tfsdk:"name"`
	ResourceType types.String       `tfsdk:"resource_type"`
	Description  types.String       `tfsdk:"description"`
	Permissions  []types.String     `tfsdk:"permissions"`
	Inherits     []RoleInheritModel `tfsdk:"inherits"`
	IsCustom     types.Bool         `tfsdk:"is_custom"`
	CreatedAt    types.String       `tfsdk:"created_at"`
	UpdatedAt    types.String       `tfsdk:"updated_at"`
}

type RolesModel struct {
	ID              types.String          `tfsdk:"id"`
	ResourceType    types.String          `tfsdk:"resource_type"`
	ResourceID      types.String          `tfsdk:"resource_id"`
	Roles           []types.String        `tfsdk:"roles"`
	RoleDefinitions []RoleDefinitionModel `tfsdk:"role_definitions"`
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
		MarkdownDescription: "This data source provides information about roles available in a given organization. When `resource_id` is specified, it returns the list of role names available for that specific resource object. When only `resource_type` is specified, it returns detailed role definitions including both built-in and custom roles with their permissions, inheritance, and metadata.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the data source.",
			},
			"resource_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of the resource for which to list roles.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(ResourceTypeOrganization), string(ResourceTypeWorkspaceGroup), string(ResourceTypeTeam), string(ResourceTypeSecret)),
				},
			},
			"resource_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The unique identifier of a specific resource object. When provided, the data source returns role names available for that resource via the `roles` attribute. When omitted, the data source returns detailed role definitions via the `role_definitions` attribute.",
				Validators: []validator.String{
					util.NewUUIDValidator(),
				},
			},
			"roles": schema.ListAttribute{
				Computed:            true,
				MarkdownDescription: "A list of role names available for the specified resource object. Populated when `resource_id` is provided.",
				ElementType:         types.StringType,
			},
			"role_definitions": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "A list of detailed role definitions for the specified resource type, including both built-in and custom roles. Populated when `resource_id` is not provided.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the role.",
						},
						"resource_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The resource type this role applies to.",
						},
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "A description of the role.",
						},
						"permissions": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "The permissions granted by this role.",
						},
						"inherits": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "The roles that this custom role inherits from.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"resource_type": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The resource type of the inherited role.",
									},
									"role": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "The name of the inherited role.",
									},
								},
							},
						},
						"is_custom": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Indicates whether this role is custom or built-in.",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The timestamp when the role was created.",
						},
						"updated_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The timestamp when the role was last updated.",
						},
					},
				},
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

	if !data.ResourceID.IsNull() && !data.ResourceID.IsUnknown() {
		d.readByResourceID(ctx, &data, resp)
	} else {
		d.readByResourceType(ctx, &data, resp)
	}
}

func (d *rolesDataSourceList) readByResourceID(ctx context.Context, data *RolesModel, resp *datasource.ReadResponse) {
	roles, err := d.getRolesByResourceTypeAndID(ctx, data.ResourceType, data.ResourceID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch roles",
			fmt.Sprintf("An error occurred while fetching roles for resource type %s and ID %s: %s", data.ResourceType.ValueString(), data.ResourceID.ValueString(), err.Error()),
		)

		return
	}

	result := toRoleModel(data.ResourceType, data.ResourceID, roles)

	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *rolesDataSourceList) readByResourceType(ctx context.Context, data *RolesModel, resp *datasource.ReadResponse) {
	resourceType := data.ResourceType.ValueString()

	rolesResp, err := d.GetV1RolesResourceTypeWithResponse(ctx, resourceType)
	if serr := util.StatusOK(rolesResp, err); serr != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch roles",
			fmt.Sprintf("An error occurred while fetching roles for resource type %s: %s", resourceType, serr.Detail),
		)

		return
	}

	result := toRoleDefinitionsModel(data.ResourceType, rolesResp.JSON200)

	diags := resp.State.Set(ctx, &result)
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

func toRoleDefinitionsModel(resourceType types.String, roles *[]management.RoleDefinition) RolesModel {
	result := RolesModel{
		ID:              types.StringValue(config.TestIDValue),
		ResourceType:    resourceType,
		RoleDefinitions: []RoleDefinitionModel{},
	}

	if roles == nil {
		return result
	}

	definitions := make([]RoleDefinitionModel, 0, len(*roles))
	for _, role := range *roles {
		description, createdAt, updatedAt := setOptionalRoleFields(&role)

		definitions = append(definitions, RoleDefinitionModel{
			Name:         types.StringValue(role.Role),
			ResourceType: types.StringValue(role.ResourceType),
			Description:  description,
			Permissions:  convertPermissions(role.Permissions),
			Inherits:     convertInherits(role.Inherits),
			IsCustom:     types.BoolValue(role.IsCustom),
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
		})
	}

	result.RoleDefinitions = definitions

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
	response, err := d.GetV1OrganizationsOrganizationIDAccessControlsWithResponse(ctx, resourceID)
	if serr := util.StatusOK(response, err); serr != nil {
		return nil, serr
	}

	return response.JSON200, nil
}

func (d *rolesDataSourceList) getWorkspaceGroupRoles(ctx context.Context, resourceID uuid.UUID) (*[]management.ResourceRole, error) {
	response, err := d.GetV1WorkspaceGroupsWorkspaceGroupIDAccessControlsWithResponse(ctx, resourceID)
	if serr := util.StatusOK(response, err); serr != nil {
		return nil, serr
	}

	return response.JSON200, nil
}

func (d *rolesDataSourceList) getTeamRoles(ctx context.Context, resourceID uuid.UUID) (*[]management.ResourceRole, error) {
	response, err := d.GetV1TeamsTeamIDAccessControlsWithResponse(ctx, resourceID)
	if serr := util.StatusOK(response, err); serr != nil {
		return nil, serr
	}

	return response.JSON200, nil
}

func (d *rolesDataSourceList) getSecretRoles(ctx context.Context, resourceID uuid.UUID) (*[]management.ResourceRole, error) {
	response, err := d.GetV1SecretsSecretIDAccessControlsWithResponse(ctx, resourceID)
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
