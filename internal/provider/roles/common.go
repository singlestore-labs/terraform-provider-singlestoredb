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

type EntityType string

const (
	EntityTypeUser EntityType = "user"
	EntityTypeTeam EntityType = "team"
)

type ResourceType string

const (
	ResourceTypeOrganization   ResourceType = "Organization"
	ResourceTypeWorkspaceGroup ResourceType = "Cluster"
	ResourceTypeTeam           ResourceType = "Team"
	ResourceTypeSecret         ResourceType = "Secret"
	ResourceTypeUnknown        ResourceType = "Unknown"
)

var ResourceTypeList = []ResourceType{
	ResourceTypeOrganization,
	ResourceTypeWorkspaceGroup,
	ResourceTypeTeam,
	ResourceTypeSecret,
}

func ResourceTypeString(provider types.String) ResourceType {
	for _, s := range []ResourceType{
		ResourceTypeOrganization,
		ResourceTypeWorkspaceGroup,
		ResourceTypeTeam,
		ResourceTypeSecret,
	} {
		if strings.EqualFold(provider.ValueString(), string(s)) {
			return s
		}
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
	return RoleAttributesModel{
		RoleName:     types.StringValue(role.Role),
		ResourceType: types.StringValue(role.ResourceType),
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
		rolesResponse, err := r.GetV1betaTeamsTeamIDIdentityRolesWithResponse(ctx, uuid.MustParse(entityIDstr), &management.GetV1betaTeamsTeamIDIdentityRolesParams{
			ResourceType: resourceType,
		})
		if serr := util.StatusOK(rolesResponse, err); serr != nil {
			return nil, serr
		}
		jsonRoles = rolesResponse.JSON200
	case EntityTypeUser:
		rolesResponse, err := r.GetV1betaUsersUserIDIdentityRolesWithResponse(ctx, uuid.MustParse(entityIDstr), &management.GetV1betaUsersUserIDIdentityRolesParams{
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
	if expectedRoles == nil && unexpectedRoles == nil {
		return roles, nil
	}

	return validateRoles(ctx, entityIDstr, entityType, resourceType, roles, expectedRoles, unexpectedRoles)
}

func validateRoles(ctx context.Context, entityIDstr string, entityType EntityType, resourceType *string, roles []RoleAttributesModel, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	var resultRoles []RoleAttributesModel
	if expectedRoles != nil {
		missedRoles := SubtractRoles(*expectedRoles, roles)
		if len(missedRoles) > 0 {
			if resourceType == nil {
				return nil, fmt.Errorf("the expected roles %v are not granted to the %s %s", missedRoles, entityType, entityIDstr)
			}

			return nil, fmt.Errorf("the expected roles %v are not granted to the %s %s for the resource type '%s'", missedRoles, entityType, entityIDstr, *resourceType)
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
	response, err := r.PatchV1betaOrganizationsOrganizationIDAccessControlsWithResponse(ctx, resourceID, management.PatchV1betaOrganizationsOrganizationIDAccessControlsJSONRequestBody{
		Grants:  grants,
		Revokes: revokes,
	})
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

func applyWorkspaceGroupAccessControls(ctx context.Context, r management.ClientWithResponsesInterface, resourceID uuid.UUID, grants, revokes []management.ControlAccessRole) (bool, error) {
	response, err := r.PatchV1betaWorkspaceGroupsWorkspaceGroupIDAccessControlsWithResponse(ctx, resourceID, management.PatchV1betaWorkspaceGroupsWorkspaceGroupIDAccessControlsJSONRequestBody{
		Grants:  grants,
		Revokes: revokes,
	})
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

func applyTeamAccessControls(ctx context.Context, r management.ClientWithResponsesInterface, resourceID uuid.UUID, grants, revokes []management.ControlAccessRole) (bool, error) {
	response, err := r.PatchV1betaTeamsTeamIDAccessControlsWithResponse(ctx, resourceID, management.PatchV1betaTeamsTeamIDAccessControlsJSONRequestBody{
		Grants:  grants,
		Revokes: revokes,
	})
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

func applySecretAccessControls(ctx context.Context, r management.ClientWithResponsesInterface, resourceID uuid.UUID, grants, revokes []management.ControlAccessRole) (bool, error) {
	response, err := r.PatchV1betaSecretsSecretIDAccessControlsWithResponse(ctx, resourceID, management.PatchV1betaSecretsSecretIDAccessControlsJSONRequestBody{
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
	return plan.ResourceID != state.ResourceID ||
		plan.ResourceType != state.ResourceType ||
		plan.RoleName != state.RoleName
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
				result = append(result, mappedRole)

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
