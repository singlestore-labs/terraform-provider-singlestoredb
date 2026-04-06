package roles

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const importIDParts = 2

var (
	_ resource.ResourceWithConfigure   = &customRoleResource{}
	_ resource.ResourceWithImportState = &customRoleResource{}
)

type CustomRoleResourceModel struct {
	ID           types.String       `tfsdk:"id"`
	Name         types.String       `tfsdk:"name"`
	ResourceType types.String       `tfsdk:"resource_type"`
	Description  types.String       `tfsdk:"description"`
	Permissions  []types.String     `tfsdk:"permissions"`
	Inherits     []RoleInheritModel `tfsdk:"inherits"`
	IsCustom     types.Bool         `tfsdk:"is_custom"`
	CreatedAt    types.String       `tfsdk:"created_at"`
	UpdatedAt    types.String       `tfsdk:"updated_at"`
}

type customRoleResource struct {
	management.ClientWithResponsesInterface
}

func NewRoleResource() resource.Resource {
	return &customRoleResource{}
}

func (r *customRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = util.ResourceTypeName(req, RoleResourceName)
}

func (r *customRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	emptyPermissionsList := types.ListValueMust(types.StringType, []attr.Value{})
	emptyInheritsList := types.ListValueMust(types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"resource_type": types.StringType,
			"role":          types.StringType,
		},
	}, []attr.Value{})

	resp.Schema = schema.Schema{
		MarkdownDescription: "This resource allows you to create and manage custom roles with fine-grained permissions for your organization. You can create roles with specific permissions and optionally inherit from other roles. Only roles with `is_custom = true` can be created, modified, or deleted through this resource.",
		Attributes: map[string]schema.Attribute{
			config.IDAttribute: schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the custom role (combination of resource_type and name).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the custom role. This must be unique within the resource type.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The type of resource this role applies to. Must be one of: Organization, Cluster, Team, or Secret. Use Cluster type for Workspace Group roles.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(ResourceTypeOrganization),
						string(ResourceTypeWorkspaceGroup),
						string(ResourceTypeTeam),
						string(ResourceTypeSecret),
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "A description of the custom role.",
			},
			"permissions": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Default:             listdefault.StaticValue(emptyPermissionsList),
				MarkdownDescription: "A list of permissions granted by this role. Available permissions depend on the resource type. Use the `singlestoredb_role_permissions` data source to discover valid permission names.",
			},
			"inherits": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "A list of roles that this custom role inherits from. The custom role will have all permissions from the inherited roles.",
				Default:             listdefault.StaticValue(emptyInheritsList),
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource_type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The resource type of the inherited role.",
							Validators: []validator.String{
								stringvalidator.OneOf(
									string(ResourceTypeOrganization),
									string(ResourceTypeWorkspaceGroup),
									string(ResourceTypeTeam),
									string(ResourceTypeSecret),
								),
							},
						},
						"role": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The name of the role to inherit from.",
						},
					},
				},
			},
			"is_custom": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Indicates whether this is a custom role (always true for resources created through this provider).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp when the custom role was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The timestamp when the custom role was last updated.",
			},
		},
	}
}

func (r *customRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CustomRoleResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := plan.ResourceType.ValueString()

	createReq := management.RoleCreate{
		Role:        plan.Name.ValueString(),
		Description: util.MaybeString(plan.Description),
		Permissions: permissionsToStrings(plan.Permissions),
		Inherits:    inheritsToTypedRoles(plan.Inherits),
	}

	createResp, err := r.PostV1RolesResourceTypeWithResponse(ctx, resourceType, createReq)
	if serr := util.StatusOK(createResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toCustomRoleResourceModel(createResp.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *customRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CustomRoleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := state.ResourceType.ValueString()
	roleName := state.Name.ValueString()

	getResp, err := r.GetV1RolesResourceTypeRoleWithResponse(ctx, resourceType, roleName)
	if serr := util.StatusOK(getResp, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	if getResp.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)

		return
	}

	if serr := validateRoleIsCustom(getResp.JSON200); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toCustomRoleResourceModel(getResp.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *customRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CustomRoleResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := plan.ResourceType.ValueString()
	roleName := plan.Name.ValueString()

	updateReq := management.RoleUpdate{
		Description: util.MaybeString(plan.Description),
		Permissions: permissionsToStrings(plan.Permissions),
		Inherits:    inheritsToTypedRoles(plan.Inherits),
	}

	updateResp, err := r.PutV1RolesResourceTypeRoleWithResponse(ctx, resourceType, roleName, updateReq)
	if serr := util.StatusOK(updateResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toCustomRoleResourceModel(updateResp.JSON200)
	diags = resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func (r *customRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CustomRoleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resourceType := state.ResourceType.ValueString()
	roleName := state.Name.ValueString()

	deleteResp, err := r.DeleteV1RolesResourceTypeRoleWithResponse(ctx, resourceType, roleName)
	if serr := util.StatusOK(deleteResp, err, util.ReturnNilOnNotFound); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}
}

func (r *customRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return // Should not return an error for unknown reasons.
	}

	r.ClientWithResponsesInterface = req.ProviderData.(management.ClientWithResponsesInterface)
}

func (r *customRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := parseCustomRoleID(req.ID)
	if idParts == nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("The import ID must be in the format 'resource_type/role_name', got: %s. %s", req.ID, formatResourceTypeList()),
		)

		return
	}

	getResp, err := r.GetV1RolesResourceTypeRoleWithResponse(ctx, idParts.ResourceType, idParts.RoleName)
	if serr := util.StatusOK(getResp, err); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	if serr := validateRoleIsCustom(getResp.JSON200); serr != nil {
		resp.Diagnostics.AddError(serr.Summary, serr.Detail)

		return
	}

	result := toCustomRoleResourceModel(getResp.JSON200)
	diags := resp.State.Set(ctx, &result)
	resp.Diagnostics.Append(diags...)
}

func toCustomRoleResourceModel(role *management.RoleDefinition) CustomRoleResourceModel {
	if role == nil {
		return CustomRoleResourceModel{}
	}

	base := toRoleDefinitionModel(role)

	return CustomRoleResourceModel{
		ID:           types.StringValue(fmt.Sprintf("%s/%s", role.ResourceType, role.Role)),
		Name:         base.Name,
		ResourceType: base.ResourceType,
		Description:  base.Description,
		Permissions:  base.Permissions,
		Inherits:     base.Inherits,
		IsCustom:     base.IsCustom,
		CreatedAt:    base.CreatedAt,
		UpdatedAt:    base.UpdatedAt,
	}
}

func validateRoleIsCustom(role *management.RoleDefinition) *util.SummaryWithDetailError {
	if role == nil {
		return &util.SummaryWithDetailError{
			Summary: "Role not found",
			Detail:  "The API returned an empty role definition.",
		}
	}

	if role.IsCustom {
		return nil
	}

	return &util.SummaryWithDetailError{
		Summary: "Role is not a custom role",
		Detail: fmt.Sprintf(
			"The role %q for resource type %q is a built-in role. Only roles with is_custom = true can be managed through this resource.",
			role.Role,
			role.ResourceType,
		),
	}
}

type customRoleIDParts struct {
	ResourceType string
	RoleName     string
}

func parseCustomRoleID(id string) *customRoleIDParts {
	parts := strings.SplitN(id, "/", importIDParts)
	if len(parts) != importIDParts || parts[1] == "" {
		return nil
	}

	resourceType := parts[0]
	for _, rt := range ResourceTypeList {
		if string(rt) == resourceType {
			return &customRoleIDParts{
				ResourceType: resourceType,
				RoleName:     parts[1],
			}
		}
	}

	return nil
}
