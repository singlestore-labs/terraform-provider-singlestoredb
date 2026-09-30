package roles

import (
	"context"
	"fmt"
	"strings"

	openapi_types "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// RoleResourceName is the Terraform resource type name for the role resource.
const RoleResourceName = "role"

// RoleInheritModel represents a role inheritance relationship.
type RoleInheritModel struct {
	ResourceType types.String `tfsdk:"resource_type"`
	Role         types.String `tfsdk:"role"`
}

// RoleDefinitionModel represents the common fields for a role definition.
// Used by both the data source (singlestoredb_roles) and the resource (singlestoredb_role).
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

func convertPermissions(permissions []string) []types.String {
	result := make([]types.String, 0, len(permissions))
	for _, perm := range permissions {
		result = append(result, types.StringValue(perm))
	}

	return result
}

func permissionsToStrings(permissions []types.String) []string {
	result := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		result = append(result, perm.ValueString())
	}

	return result
}

func convertInherits(inherits []management.TypedRole) []RoleInheritModel {
	result := make([]RoleInheritModel, 0, len(inherits))
	for _, inherit := range inherits {
		result = append(result, RoleInheritModel{
			ResourceType: types.StringValue(inherit.ResourceType),
			Role:         types.StringValue(inherit.Role),
		})
	}

	return result
}

func inheritsToTypedRoles(inherits []RoleInheritModel) []management.TypedRole {
	result := make([]management.TypedRole, 0, len(inherits))
	for _, inherit := range inherits {
		result = append(result, management.TypedRole{
			ResourceType: inherit.ResourceType.ValueString(),
			Role:         inherit.Role.ValueString(),
		})
	}

	return result
}

func setOptionalRoleFields(role *management.RoleDefinition) (types.String, types.String, types.String) {
	description := types.StringValue("")
	createdAt := types.StringNull()
	updatedAt := types.StringNull()

	if role.Description != nil && *role.Description != "" {
		description = types.StringValue(*role.Description)
	}

	if role.CreatedAt != nil {
		createdAt = util.MaybeTimeValue(role.CreatedAt)
	}

	if role.UpdatedAt != nil {
		updatedAt = util.MaybeTimeValue(role.UpdatedAt)
	}

	return description, createdAt, updatedAt
}

// toRoleDefinitionModel converts a management.RoleDefinition to RoleDefinitionModel.
// This is the single source of truth for mapping API role definitions to Terraform models.
func toRoleDefinitionModel(role *management.RoleDefinition) RoleDefinitionModel {
	if role == nil {
		return RoleDefinitionModel{}
	}

	description, createdAt, updatedAt := setOptionalRoleFields(role)

	return RoleDefinitionModel{
		Name:         types.StringValue(role.Role),
		ResourceType: types.StringValue(role.ResourceType),
		Description:  description,
		Permissions:  convertPermissions(role.Permissions),
		Inherits:     convertInherits(role.Inherits),
		IsCustom:     types.BoolValue(role.IsCustom),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func resourceTypeNames() string {
	names := make([]string, 0, len(ResourceTypeList))
	for _, rt := range ResourceTypeList {
		names = append(names, string(rt))
	}

	return strings.Join(names, ", ")
}

func formatResourceTypeList() string {
	return fmt.Sprintf("Valid resource types are: %s", resourceTypeNames())
}

// RoleNotFoundError indicates that expected roles were not found.
type RoleNotFoundError struct {
	MissedRoles  []RoleAttributesModel
	EntityType   EntityType
	EntityID     string
	ResourceType *string
}

func (e *RoleNotFoundError) Error() string {
	if e.ResourceType == nil {
		return fmt.Sprintf("the expected roles %v are not granted to the %s %s",
			e.MissedRoles, e.EntityType, e.EntityID)
	}

	return fmt.Sprintf("the expected roles %v are not granted to the %s %s for the resource type '%s'",
		e.MissedRoles, e.EntityType, e.EntityID, *e.ResourceType)
}

type EntityType string

const (
	EntityTypeUser EntityType = "user"
	EntityTypeTeam EntityType = "team"
)

type ResourceType string

const (
	ResourceTypeOrganization   ResourceType = "Organization"
	ResourceTypeWorkspaceGroup ResourceType = "Cluster"
	// resourceTypeClusterGroup is returned by identity-roles APIs for workspace-group
	// scoped cluster grants. Terraform continues to use resource_type = "Cluster".
	resourceTypeClusterGroup ResourceType = "ClusterGroup"
	ResourceTypeTeam         ResourceType = "Team"
	ResourceTypeSecret       ResourceType = "Secret"
	ResourceTypeUnknown      ResourceType = "Unknown"
)

var ResourceTypeList = []ResourceType{
	ResourceTypeOrganization,
	ResourceTypeWorkspaceGroup,
	ResourceTypeTeam,
	ResourceTypeSecret,
}

func ResourceTypeString(provider types.String) ResourceType {
	value := provider.ValueString()
	for _, s := range []ResourceType{
		ResourceTypeOrganization,
		ResourceTypeWorkspaceGroup,
		ResourceTypeTeam,
		ResourceTypeSecret,
	} {
		if strings.EqualFold(value, string(s)) {
			return s
		}
	}
	// Identity-roles APIs report workspace-group grants as ClusterGroup.
	if strings.EqualFold(value, string(resourceTypeClusterGroup)) {
		return ResourceTypeWorkspaceGroup
	}

	return ResourceTypeUnknown
}

func RoleAttributesSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"role_name": schema.StringAttribute{
			Required:            true,
			MarkdownDescription: "The name of the role.",
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
	}
}

type RoleAttributesModel struct {
	RoleName     types.String `tfsdk:"role_name"`
	ResourceType types.String `tfsdk:"resource_type"`
	ResourceID   types.String `tfsdk:"resource_id"`
}

func toRoleAttributesModel(role management.IdentityRole) RoleAttributesModel {
	resourceType := role.ResourceType
	// Normalize API ClusterGroup → Terraform Cluster so configs using
	// resource_type = "Cluster" with singlestoredb_workspace_group.id keep working.
	if ResourceTypeString(types.StringValue(resourceType)) == ResourceTypeWorkspaceGroup {
		resourceType = string(ResourceTypeWorkspaceGroup)
	}

	return RoleAttributesModel{
		RoleName:     types.StringValue(role.Role),
		ResourceType: types.StringValue(resourceType),
		ResourceID:   util.UUIDStringValue(role.ResourceID),
	}
}

func getUserRolesAndValidate(ctx context.Context, r management.ClientWithResponsesInterface, userIDstr string, resourceType *string, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	return getRolesAndValidate(ctx, r, userIDstr, EntityTypeUser, resourceType, expectedRoles, unexpectedRoles)
}

func getTeamRolesAndValidate(ctx context.Context, r management.ClientWithResponsesInterface, teamIDstr string, resourceType *string, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	return getRolesAndValidate(ctx, r, teamIDstr, EntityTypeTeam, resourceType, expectedRoles, unexpectedRoles)
}

func getRolesAndValidate(ctx context.Context, r management.ClientWithResponsesInterface, entityIDstr string, entityType EntityType, resourceType *string, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	var jsonRoles *[]management.IdentityRole
	switch entityType {
	case EntityTypeTeam:
		rolesResponse, err := r.GetV2TeamsTeamIDIdentityRolesWithResponse(ctx, uuid.MustParse(entityIDstr), &management.GetV2TeamsTeamIDIdentityRolesParams{
			ResourceType: resourceType,
		})
		if serr := util.StatusOK(rolesResponse, err); serr != nil {
			return nil, serr
		}
		jsonRoles = rolesResponse.JSON200
	case EntityTypeUser:
		rolesResponse, err := r.GetV2UsersUserIDIdentityRolesWithResponse(ctx, uuid.MustParse(entityIDstr), &management.GetV2UsersUserIDIdentityRolesParams{
			ResourceType: resourceType,
		})

		if serr := util.StatusOK(rolesResponse, err); serr != nil {
			return nil, serr
		}
		jsonRoles = rolesResponse.JSON200
	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	roles := util.Map(util.Deref(jsonRoles), toRoleAttributesModel)
	// Identity roles for resource_type=Cluster return cluster IDs, while Terraform configs
	// historically use singlestoredb_workspace_group.id (group ID). Align IDs so validation
	// and state match the configured resource_id.
	roles, err := alignClusterRoleResourceIDs(ctx, r, roles, expectedRoles, unexpectedRoles)
	if err != nil {
		return nil, err
	}

	if expectedRoles == nil && unexpectedRoles == nil {
		return roles, nil
	}

	return validateRoles(ctx, entityIDstr, entityType, resourceType, roles, expectedRoles, unexpectedRoles)
}

// alignClusterRoleResourceIDs rewrites Cluster role resource IDs from cluster IDs to the
// identifier form used in Terraform (typically workspace group ID). When expected/unexpected
// roles are provided, prefer that configured identifier if it refers to the same cluster/group.
func alignClusterRoleResourceIDs(
	ctx context.Context,
	c management.ClientWithResponsesInterface,
	roles []RoleAttributesModel,
	expectedRoles, unexpectedRoles *[]RoleAttributesModel,
) ([]RoleAttributesModel, error) {
	needsAlign := false
	for _, role := range roles {
		if ResourceTypeString(role.ResourceType) == ResourceTypeWorkspaceGroup {
			needsAlign = true

			break
		}
	}
	if !needsAlign {
		return roles, nil
	}

	list, err := c.GetV2ClustersWithResponse(ctx, &management.GetV2ClustersParams{})
	if serr := util.StatusOK(list, err); serr != nil {
		return nil, serr
	}

	clusterToGroup := make(map[uuid.UUID]uuid.UUID)
	for _, cluster := range util.Deref(list.JSON200) {
		if cluster.ClusterID != nil && cluster.GroupID != nil {
			clusterToGroup[*cluster.ClusterID] = *cluster.GroupID
		}
	}

	configuredIDs := configuredClusterResourceIDs(expectedRoles, unexpectedRoles)
	aligned := make([]RoleAttributesModel, len(roles))
	for i, role := range roles {
		aligned[i] = role
		if ResourceTypeString(role.ResourceType) != ResourceTypeWorkspaceGroup {
			continue
		}
		resourceID, perr := uuid.Parse(role.ResourceID.ValueString())
		if perr != nil {
			continue
		}
		groupID, isClusterID := clusterToGroup[resourceID]
		if !isClusterID {
			continue
		}
		if id, ok := pickConfiguredClusterResourceID(configuredIDs, resourceID, groupID); ok {
			aligned[i].ResourceID = util.UUIDStringValue(id)

			continue
		}
		aligned[i].ResourceID = util.UUIDStringValue(groupID)
	}

	return aligned, nil
}

func configuredClusterResourceIDs(expectedRoles, unexpectedRoles *[]RoleAttributesModel) []uuid.UUID {
	var ids []uuid.UUID
	for _, list := range []*[]RoleAttributesModel{expectedRoles, unexpectedRoles} {
		if list == nil {
			continue
		}
		for _, role := range *list {
			if ResourceTypeString(role.ResourceType) != ResourceTypeWorkspaceGroup {
				continue
			}
			id, err := uuid.Parse(role.ResourceID.ValueString())
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}
	}

	return ids
}

func pickConfiguredClusterResourceID(configured []uuid.UUID, clusterID, groupID uuid.UUID) (uuid.UUID, bool) {
	for _, id := range configured {
		if id == clusterID || id == groupID {
			return id, true
		}
	}

	return uuid.UUID{}, false
}

func validateRoles(ctx context.Context, entityIDstr string, entityType EntityType, resourceType *string, roles []RoleAttributesModel, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	var resultRoles []RoleAttributesModel
	if expectedRoles != nil {
		missedRoles := SubtractRoles(*expectedRoles, roles)
		if len(missedRoles) > 0 {
			return nil, &RoleNotFoundError{
				MissedRoles:  missedRoles,
				EntityType:   entityType,
				EntityID:     entityIDstr,
				ResourceType: resourceType,
			}
		}
		resultRoles = MatchedRoles(*expectedRoles, roles)
	}

	if unexpectedRoles != nil {
		foundRoles := MatchedRoles(roles, *unexpectedRoles)
		if len(foundRoles) > 0 {
			if resourceType == nil {
				tflog.Warn(ctx, fmt.Sprintf("the roles %v are already granted to the %s %s", foundRoles, entityType, entityIDstr))
			} else {
				tflog.Warn(ctx, fmt.Sprintf("the roles %v are already granted to the %s %s for the resource type '%s'", foundRoles, entityType, entityIDstr, *resourceType))
			}
		}
	}

	return resultRoles, nil
}

func grantUserRoles(ctx context.Context, r management.ClientWithResponsesInterface, userIDstr types.String, roles []RoleAttributesModel) (bool, error) {
	return handleRoles(ctx, r, userIDstr, EntityTypeUser, roles, true)
}

func revokeUserRoles(ctx context.Context, r management.ClientWithResponsesInterface, userIDstr types.String, roles []RoleAttributesModel) (bool, error) {
	return handleRoles(ctx, r, userIDstr, EntityTypeUser, roles, false)
}

func grantTeamRoles(ctx context.Context, r management.ClientWithResponsesInterface, teamIDstr types.String, roles []RoleAttributesModel) (bool, error) {
	return handleRoles(ctx, r, teamIDstr, EntityTypeTeam, roles, true)
}

func revokeTeamRoles(ctx context.Context, r management.ClientWithResponsesInterface, teamIDstr types.String, roles []RoleAttributesModel) (bool, error) {
	return handleRoles(ctx, r, teamIDstr, EntityTypeTeam, roles, false)
}

func handleRoles(ctx context.Context, r management.ClientWithResponsesInterface, entityIDstr types.String, entityType EntityType, roles []RoleAttributesModel, isGrant bool) (bool, error) {
	rolesByResourceType := GroupRolesByResourceType(roles)
	for _, resourceType := range ResourceTypeList {
		roles, exists := rolesByResourceType[resourceType]
		if !exists {
			continue
		}
		groupedRoles := GroupRolesByResourceID(roles)
		for resourceID, rolesByResourceID := range groupedRoles {
			var grantRoles, revokeRoles *[]RoleAttributesModel
			if isGrant {
				grantRoles = &rolesByResourceID
			} else {
				revokeRoles = &rolesByResourceID
			}

			ok, err := modifyAccessControlsForResource(ctx, r, entityIDstr, entityType, resourceID, resourceType, grantRoles, revokeRoles)
			if !ok || err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func modifyUserAccessControlsForResource(ctx context.Context, r management.ClientWithResponsesInterface, entityIDstr, resourceIDstr, resourceType types.String, grantRoles, revokeRoles *[]RoleAttributesModel) (bool, error) {
	return modifyAccessControlsForResource(ctx, r, entityIDstr, EntityTypeUser, resourceIDstr, ResourceTypeString(resourceType), grantRoles, revokeRoles)
}

func modifyTeamAccessControlsForResource(ctx context.Context, r management.ClientWithResponsesInterface, entityIDstr, resourceIDstr, resourceType types.String, grantRoles, revokeRoles *[]RoleAttributesModel) (bool, error) {
	return modifyAccessControlsForResource(ctx, r, entityIDstr, EntityTypeTeam, resourceIDstr, ResourceTypeString(resourceType), grantRoles, revokeRoles)
}

func modifyAccessControlsForResource(ctx context.Context, r management.ClientWithResponsesInterface, entityIDstr types.String, entityType EntityType, resourceIDstr types.String, resourceType ResourceType, grantRoles, revokeRoles *[]RoleAttributesModel) (bool, error) {
	entityID, err := uuid.Parse(entityIDstr.ValueString())
	if err != nil {
		return false, fmt.Errorf("invalid entity ID: %w", err)
	}

	resourceID, err := uuid.Parse(resourceIDstr.ValueString())
	if err != nil {
		return false, fmt.Errorf("invalid resource ID: %w", err)
	}

	grants, revokes := mapRoles(entityType, entityID, grantRoles, revokeRoles)

	switch resourceType {
	case ResourceTypeOrganization:
		return applyOrganizationAccessControls(ctx, r, resourceID, grants, revokes)
	case ResourceTypeWorkspaceGroup:
		return applyWorkspaceGroupAccessControls(ctx, r, resourceID, grants, revokes)
	case ResourceTypeTeam:
		return applyTeamAccessControls(ctx, r, resourceID, grants, revokes)
	case ResourceTypeSecret:
		return applySecretAccessControls(ctx, r, resourceID, grants, revokes)
	case ResourceTypeUnknown:
		return false, fmt.Errorf("wrong resource type marked as: %s", resourceType)
	default:
		return false, fmt.Errorf("unsupported resource type: %s", resourceType)
	}
}

func applyOrganizationAccessControls(ctx context.Context, r management.ClientWithResponsesInterface, resourceID uuid.UUID, grants, revokes []management.ControlAccessRole) (bool, error) {
	response, err := r.PatchV2OrganizationsOrganizationIDAccessControlsWithResponse(ctx, resourceID, management.PatchV2OrganizationsOrganizationIDAccessControlsJSONRequestBody{
		Grants:  grants,
		Revokes: revokes,
	})
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

func applyWorkspaceGroupAccessControls(ctx context.Context, r management.ClientWithResponsesInterface, resourceID uuid.UUID, grants, revokes []management.ControlAccessRole) (bool, error) {
	clusterID, err := resolveClusterIDForAccessControls(ctx, r, resourceID)
	if err != nil {
		return false, err
	}

	response, err := r.PatchV2ClustersClusterIDAccessControlsWithResponse(ctx, clusterID, management.PatchV2ClustersClusterIDAccessControlsJSONRequestBody{
		Grants:  grants,
		Revokes: revokes,
	})
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

// resolveClusterIDForAccessControls accepts either a cluster ID or a workspace group ID.
// Terraform configs historically pass singlestoredb_workspace_group.id (group ID) for
// resource_type = "Cluster"; /v2/clusters/{id}/accessControls expects a cluster ID.
func resolveClusterIDForAccessControls(ctx context.Context, c management.ClientWithResponsesInterface, resourceID uuid.UUID) (uuid.UUID, error) {
	direct, err := c.GetV2ClustersClusterIDWithResponse(ctx, resourceID, &management.GetV2ClustersClusterIDParams{})
	if err == nil && direct != nil && direct.StatusCode() == 200 && direct.JSON200 != nil {
		return resourceID, nil
	}

	list, err := c.GetV2ClustersWithResponse(ctx, &management.GetV2ClustersParams{})
	if serr := util.StatusOK(list, err); serr != nil {
		return uuid.UUID{}, serr
	}

	for _, cluster := range util.Deref(list.JSON200) {
		if cluster.GroupID != nil && *cluster.GroupID == resourceID && cluster.ClusterID != nil {
			return *cluster.ClusterID, nil
		}
	}

	return uuid.UUID{}, &util.SummaryWithDetailError{
		Summary: "Failed to resolve cluster ID",
		Detail:  fmt.Sprintf("No cluster was found for resource ID %s (tried as cluster ID and as workspace group ID).", resourceID),
	}
}

func applyTeamAccessControls(ctx context.Context, r management.ClientWithResponsesInterface, resourceID uuid.UUID, grants, revokes []management.ControlAccessRole) (bool, error) {
	response, err := r.PatchV2TeamsTeamIDAccessControlsWithResponse(ctx, resourceID, management.PatchV2TeamsTeamIDAccessControlsJSONRequestBody{
		Grants:  grants,
		Revokes: revokes,
	})
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

func applySecretAccessControls(ctx context.Context, r management.ClientWithResponsesInterface, resourceID uuid.UUID, grants, revokes []management.ControlAccessRole) (bool, error) {
	response, err := r.PatchV2SecretsSecretIDAccessControlsWithResponse(ctx, resourceID, management.PatchV2SecretsSecretIDAccessControlsJSONRequestBody{
		Grants:  grants,
		Revokes: revokes,
	})
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

func mapRoles(entityType EntityType, entityID uuid.UUID, grantRoles, revokeRoles *[]RoleAttributesModel) ([]management.ControlAccessRole, []management.ControlAccessRole) {
	var grants, revokes []management.ControlAccessRole

	if grantRoles != nil {
		grants = mapRoleAttributes(entityType, entityID, grantRoles)
	}

	if revokeRoles != nil {
		revokes = mapRoleAttributes(entityType, entityID, revokeRoles)
	}

	return grants, revokes
}

func mapRoleAttributes(entityType EntityType, entityID uuid.UUID, roles *[]RoleAttributesModel) []management.ControlAccessRole {
	accessRoles := make([]management.ControlAccessRole, 0, len(*roles))

	for _, role := range *roles {
		controlAccessRole := management.ControlAccessRole{
			Role: role.RoleName.ValueString(),
		}

		switch entityType {
		case EntityTypeTeam:
			controlAccessRole.Teams = []openapi_types.UUID{entityID}
		case EntityTypeUser:
			controlAccessRole.Users = []openapi_types.UUID{entityID}
		default:
			panic(fmt.Sprintf("unsupported entity type: %s", entityType))
		}

		accessRoles = append(accessRoles, controlAccessRole)
	}

	return accessRoles
}

func IsRoleChanged(plan, state RoleAttributesModel) bool {
	if !strings.EqualFold(plan.RoleName.ValueString(), state.RoleName.ValueString()) {
		return true
	}
	if !strings.EqualFold(plan.ResourceType.ValueString(), state.ResourceType.ValueString()) {
		return true
	}

	return plan.ResourceID.ValueString() != state.ResourceID.ValueString()
}

func SubtractRoles(a, b []RoleAttributesModel) []RoleAttributesModel {
	var result []RoleAttributesModel
	for _, role := range a {
		notFound := true
		for _, stateRole := range b {
			if !IsRoleChanged(role, stateRole) {
				notFound = false

				break
			}
		}
		if notFound {
			result = append(result, role)
		}
	}

	return result
}

func MatchedRoles(a, b []RoleAttributesModel) []RoleAttributesModel {
	var result []RoleAttributesModel
	for _, role := range a {
		for _, mappedRole := range b {
			if !IsRoleChanged(role, mappedRole) {
				// Prefer the configured role (a) so Cluster resource_id stays as the
				// workspace group ID from Terraform config when IDs were aligned.
				result = append(result, role)

				break
			}
		}
	}

	return result
}

func GroupRolesByResourceID(roles []RoleAttributesModel) map[types.String][]RoleAttributesModel {
	groupedRoles := make(map[types.String][]RoleAttributesModel)
	for _, role := range roles {
		groupedRoles[role.ResourceID] = append(groupedRoles[role.ResourceID], role)
	}

	return groupedRoles
}

func GroupRolesByResourceType(roles []RoleAttributesModel) map[ResourceType][]RoleAttributesModel {
	groupedRoles := make(map[ResourceType][]RoleAttributesModel)
	for _, role := range roles {
		resourceType := ResourceTypeString(role.ResourceType)
		groupedRoles[resourceType] = append(groupedRoles[resourceType], role)
	}

	return groupedRoles
}
