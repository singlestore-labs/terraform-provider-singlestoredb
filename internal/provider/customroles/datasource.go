package customroles

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
	AllRolesDataSourceName = "all_roles"
)

type RoleModel struct {
	Name         types.String         `tfsdk:"name"`
	ResourceType types.String         `tfsdk:"resource_type"`
	Description  types.String         `tfsdk:"description"`
	Permissions  []types.String       `tfsdk:"permissions"`
	Inherits     []InheritedRoleModel `tfsdk:"inherits"`
	IsCustom     types.Bool           `tfsdk:"is_custom"`
	CreatedAt    types.String         `tfsdk:"created_at"`
	UpdatedAt    types.String         `tfsdk:"updated_at"`
}

type AllRolesDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	ResourceType types.String `tfsdk:"resource_type"`
	Roles        []RoleModel  `tfsdk:"roles"`
}

type allRolesDataSource struct {
	management.ClientWithResponsesInterface
}

var _ datasource.DataSourceWithConfigure = &allRolesDataSource{}

func NewAllRolesDataSourceList() datasource.DataSource {
	return &allRolesDataSource{}
}

func (d *allRolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = util.DataSourceTypeName(req, AllRolesDataSourceName)
}

func (d *allRolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "This data source lists all roles defined for a specific resource type, including both built-in and custom roles. Use this data source to discover which roles are available to assign to users or teams and to distinguish custom roles from built-in ones via the is_custom field.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for this data source.",
			},
			"resource_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of resource to list roles for. Must be one of: Organization, Cluster, Team, or Secret.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(ResourceTypeOrganization),
						string(ResourceTypeWorkspaceGroup),
						string(ResourceTypeTeam),
						string(ResourceTypeSecret),
					),
				},
			},
			"roles": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "A list of roles for the specified resource type.",
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

func (d *allRolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data AllRolesDataSourceModel
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

	result := toAllRolesDataSourceModel(data.ResourceType, rolesResp.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (d *allRolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	d.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func toAllRolesDataSourceModel(resourceType types.String, roles *[]management.RoleDefinition) AllRolesDataSourceModel {
	result := AllRolesDataSourceModel{
		ID:           types.StringValue(config.TestIDValue),
		ResourceType: resourceType,
		Roles:        []RoleModel{},
	}

	if roles == nil {
		return result
	}

	allRoles := make([]RoleModel, 0, len(*roles))
	for _, role := range *roles {
		description, createdAt, updatedAt := setOptionalRoleFields(&role)

		allRoles = append(allRoles, RoleModel{
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

	result.Roles = allRoles

	return result
}
