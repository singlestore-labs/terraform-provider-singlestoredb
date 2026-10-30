package roles

import (
	"context"
	"fmt"
	"net/http"
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
	ResourceTypeTeam           ResourceType = "Team"
	ResourceTypeSecret         ResourceType = "Secret"
	ResourceTypeUnknown        ResourceType = "Unknown"
)

// identityRolesClusterGroupType is returned by identity-roles APIs for workspace-group
// scoped cluster grants. Terraform continues to use resource_type = "Cluster".
const identityRolesClusterGroupType = "ClusterGroup"

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
	if strings.EqualFold(value, identityRolesClusterGroupType) {
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

// apiResourceTypeFilter returns the resourceType value to send to the identity-roles API.
// The server-side filter is an exact string match, but v2 reports a workspace-group scoped
// Cluster grant as ClusterGroup. Filtering by "Cluster" would silently drop those rows
// before they ever reach the ClusterGroup->Cluster normalization in toRoleAttributesModel,
// so a Cluster lookup fetches every resource type unfiltered and relies on client-side
// matching (via expectedRoles/unexpectedRoles in validateRoles) instead.
func apiResourceTypeFilter(resourceType *string) *string {
	if resourceType == nil {
		return nil
	}
	if ResourceTypeString(types.StringValue(*resourceType)) == ResourceTypeWorkspaceGroup {
		return nil
	}

	return resourceType
}

func getUserRolesAndValidate(ctx context.Context, r management.ClientWithResponsesInterface, userIDstr string, resourceType *string, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	return getRolesAndValidate(ctx, r, userIDstr, EntityTypeUser, resourceType, expectedRoles, unexpectedRoles)
}

func getTeamRolesAndValidate(ctx context.Context, r management.ClientWithResponsesInterface, teamIDstr string, resourceType *string, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	return getRolesAndValidate(ctx, r, teamIDstr, EntityTypeTeam, resourceType, expectedRoles, unexpectedRoles)
}

func getRolesAndValidate(ctx context.Context, r management.ClientWithResponsesInterface, entityIDstr string, entityType EntityType, resourceType *string, expectedRoles, unexpectedRoles *[]RoleAttributesModel) ([]RoleAttributesModel, error) {
	apiResourceType := apiResourceTypeFilter(resourceType)

	var jsonRoles *[]management.IdentityRole
	switch entityType {
	case EntityTypeTeam:
		rolesResponse, err := r.GetV2TeamsTeamIDIdentityRolesWithResponse(ctx, uuid.MustParse(entityIDstr), &management.GetV2TeamsTeamIDIdentityRolesParams{
			ResourceType: apiResourceType,
		})
		if serr := util.StatusOK(rolesResponse, err); serr != nil {
			return nil, serr
		}
		jsonRoles = rolesResponse.JSON200
	case EntityTypeUser:
		rolesResponse, err := r.GetV2UsersUserIDIdentityRolesWithResponse(ctx, uuid.MustParse(entityIDstr), &management.GetV2UsersUserIDIdentityRolesParams{
			ResourceType: apiResourceType,
		})

		if serr := util.StatusOK(rolesResponse, err); serr != nil {
			return nil, serr
		}
		jsonRoles = rolesResponse.JSON200
	default:
		return nil, fmt.Errorf("unsupported entity type: %s", entityType)
	}

	roles := util.Map(util.Deref(jsonRoles), toRoleAttributesModel)
	// Enumeration (the user/team roles data sources) names no cluster to align and does
	// not check the result against a config. Skip alignment — and its /v2/clusters list —
	// so a key that cannot list clusters, or a transient failure, does not fail a read
	// that never uses the list.
	if expectedRoles == nil && unexpectedRoles == nil {
		return roles, nil
	}

	// A config may name a cluster by its ID or by a parent group ID that resolves to that
	// same access-control target. Align only those identifiers. A workspace-group grant and
	// a grant on a child cluster stay distinct.
	roles, err := alignClusterRoleResourceIDs(ctx, r, roles, expectedRoles, unexpectedRoles)
	if err != nil {
		return nil, err
	}

	return validateRoles(ctx, entityIDstr, entityType, resourceType, roles, expectedRoles, unexpectedRoles)
}

// alignClusterRoleResourceIDs rewrites a Cluster grant's resource ID to the identifier
// configured in Terraform when both identifiers resolve to the same access-control target.
// A workspace-group grant and a child-cluster grant use different endpoints, so a child
// grant is never rewritten to the group ID. Grants with no matching configured identifier
// keep the API ID. Role enumeration that does not name a cluster skips the cluster list.
func alignClusterRoleResourceIDs(
	ctx context.Context,
	c management.ClientWithResponsesInterface,
	roles []RoleAttributesModel,
	expectedRoles, unexpectedRoles *[]RoleAttributesModel,
) ([]RoleAttributesModel, error) {
	configuredIDs := configuredClusterResourceIDs(expectedRoles, unexpectedRoles)
	if len(configuredIDs) == 0 {
		return roles, nil
	}

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

	clusterToGroup := clusterToGroupIDs(util.Deref(list.JSON200))

	aligned := make([]RoleAttributesModel, len(roles))
	for i, role := range roles {
		next, aerr := alignOneClusterRoleResourceID(ctx, c, role, clusterToGroup, configuredIDs)
		if aerr != nil {
			return nil, aerr
		}
		aligned[i] = next
	}

	return aligned, nil
}

// configuredClusterRoleID pairs a role name with a configured Cluster resource ID so
// alignment only considers identifiers configured for the same role.
type configuredClusterRoleID struct {
	roleName string
	id       uuid.UUID
}

func clusterToGroupIDs(clusters []management.Cluster) map[uuid.UUID]uuid.UUID {
	clusterToGroup := make(map[uuid.UUID]uuid.UUID, len(clusters))
	for _, cluster := range clusters {
		if cluster.ClusterID == nil || cluster.GroupID == nil {
			continue
		}

		clusterToGroup[*cluster.ClusterID] = *cluster.GroupID
	}

	return clusterToGroup
}

func alignOneClusterRoleResourceID(
	ctx context.Context,
	c management.ClientWithResponsesInterface,
	role RoleAttributesModel,
	clusterToGroup map[uuid.UUID]uuid.UUID,
	configured []configuredClusterRoleID,
) (RoleAttributesModel, error) {
	if ResourceTypeString(role.ResourceType) != ResourceTypeWorkspaceGroup {
		return role, nil
	}
	resourceID, ok := parseUUID(role.ResourceID.ValueString())
	if !ok {
		return role, nil
	}
	groupID, isClusterID := clusterToGroup[resourceID]
	if !isClusterID {
		return role, nil
	}
	// Scope candidate identifiers to this row's own role name. Pooling every configured
	// identifier regardless of role would let an unrelated role (e.g. Reader@clusterX)
	// make a Owner@groupG row match clusterX merely because clusterX is already the
	// resourceID, masking that Owner should instead be compared against groupG.
	configuredIDs := idsForRoleName(configured, role.RoleName.ValueString())
	configuredID, ok := pickConfiguredClusterResourceID(configuredIDs, resourceID, groupID)
	if !ok || configuredID == resourceID {
		return role, nil
	}
	same, err := sameClusterAccessTarget(ctx, c, resourceID, configuredID, clusterToGroup)
	if err != nil || !same {
		return role, err
	}
	role.ResourceID = util.UUIDStringValue(configuredID)

	return role, nil
}

func parseUUID(id string) (uuid.UUID, bool) {
	parsed, err := uuid.Parse(id)
	if err != nil {
		return uuid.UUID{}, false
	}

	return parsed, true
}

// sameClusterAccessTarget reports whether two IDs grant through the same endpoint.
// A cluster ID and its workspace group ID do not: the group stays on the v1 workspace-group
// API, and the cluster uses the v2 cluster API. A parent group ID that is not itself a
// workspace group resolves to the child cluster, and that pair does match.
func sameClusterAccessTarget(
	ctx context.Context,
	c management.ClientWithResponsesInterface,
	clusterID, configuredID uuid.UUID,
	clusterToGroup map[uuid.UUID]uuid.UUID,
) (bool, error) {
	if configuredID == clusterID {
		return true, nil
	}
	if _, isCluster := clusterToGroup[configuredID]; isCluster {
		return false, nil
	}

	isWorkspaceGroup, err := workspaceGroupExists(ctx, c, configuredID)
	if err != nil || isWorkspaceGroup {
		return false, err
	}

	for id, groupID := range clusterToGroup {
		if groupID == configuredID && id == clusterID {
			return true, nil
		}
	}

	return false, nil
}

// roleWithPlannedSpelling keeps access-control matching case-insensitive while storing
// the planned role_name and resource_type spelling. Call it only when IsRoleChanged is
// false, so a case-only change such as Owner to owner is persisted without an API call.
func roleWithPlannedSpelling(plan, state RoleAttributesModel) (RoleAttributesModel, bool) {
	if plan.RoleName.Equal(state.RoleName) && plan.ResourceType.Equal(state.ResourceType) {
		return state, false
	}

	state.RoleName = plan.RoleName
	state.ResourceType = plan.ResourceType

	return state, true
}

func configuredClusterResourceIDs(expectedRoles, unexpectedRoles *[]RoleAttributesModel) []configuredClusterRoleID {
	var ids []configuredClusterRoleID
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
			ids = append(ids, configuredClusterRoleID{roleName: role.RoleName.ValueString(), id: id})
		}
	}

	return ids
}

// idsForRoleName returns the configured Cluster resource IDs for roles sharing roleName,
// matched case-insensitively like IsRoleChanged.
func idsForRoleName(configured []configuredClusterRoleID, roleName string) []uuid.UUID {
	var ids []uuid.UUID
	for _, c := range configured {
		if strings.EqualFold(c.roleName, roleName) {
			ids = append(ids, c.id)
		}
	}

	return ids
}

func pickConfiguredClusterResourceID(configured []uuid.UUID, clusterID, groupID uuid.UUID) (uuid.UUID, bool) {
	// An exact cluster ID match wins over its group ID.
	for _, want := range []uuid.UUID{clusterID, groupID} {
		for _, id := range configured {
			if id == want {
				return id, true
			}
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
	target, err := resolveClusterAccessTarget(ctx, r, resourceID)
	if err != nil {
		return false, err
	}

	body := management.ControlAccessAction{
		Grants:  grants,
		Revokes: revokes,
	}
	var response util.StatusCoder
	switch target.endpoint {
	case clusterAccessEndpointV1WorkspaceGroup:
		response, err = r.PatchV1WorkspaceGroupsWorkspaceGroupIDAccessControlsWithResponse(ctx, target.id, body)
	case clusterAccessEndpointV2Cluster:
		response, err = r.PatchV2ClustersClusterIDAccessControlsWithResponse(ctx, target.id, body)
	default:
		return false, fmt.Errorf("unsupported cluster access endpoint")
	}
	if serr := util.StatusOK(response, err); serr != nil {
		return false, serr
	}

	return true, nil
}

// clusterAccessEndpoint selects the access-control API for a Cluster role.
// A workspace group ID keeps using the v1 workspace-group API. A cluster ID,
// including one resolved from a parent group that is not itself a workspace group,
// uses the v2 cluster API.
type clusterAccessEndpoint int

const (
	clusterAccessEndpointV2Cluster clusterAccessEndpoint = iota
	clusterAccessEndpointV1WorkspaceGroup
)

type clusterAccessTarget struct {
	endpoint clusterAccessEndpoint
	id       uuid.UUID
}

// resolveClusterAccessTarget accepts a cluster ID or a workspace group ID.
// Terraform configs pass singlestoredb_workspace_group.id for resource_type = "Cluster".
// That ID is the group, not a v2 cluster, so access controls stay on the workspace group.
func resolveClusterAccessTarget(ctx context.Context, c management.ClientWithResponsesInterface, resourceID uuid.UUID) (clusterAccessTarget, error) {
	isCluster, err := clusterExists(ctx, c, resourceID)
	if err != nil {
		return clusterAccessTarget{}, err
	}
	if isCluster {
		return clusterAccessTarget{endpoint: clusterAccessEndpointV2Cluster, id: resourceID}, nil
	}

	isWorkspaceGroup, err := workspaceGroupExists(ctx, c, resourceID)
	if err != nil {
		return clusterAccessTarget{}, err
	}
	if isWorkspaceGroup {
		return clusterAccessTarget{endpoint: clusterAccessEndpointV1WorkspaceGroup, id: resourceID}, nil
	}

	return resolveSoleClusterInGroup(ctx, c, resourceID)
}

// resolveSoleClusterInGroup looks up every cluster whose GroupID matches resourceID and
// resolves to it only when exactly one such cluster exists. Picking an arbitrary child
// when there are several would grant/revoke access on the wrong cluster if the list order
// ever changes, silently leaking or losing access, so the caller must name the specific
// cluster instead.
func resolveSoleClusterInGroup(ctx context.Context, c management.ClientWithResponsesInterface, resourceID uuid.UUID) (clusterAccessTarget, error) {
	list, err := c.GetV2ClustersWithResponse(ctx, &management.GetV2ClustersParams{})
	if serr := util.StatusOK(list, err); serr != nil {
		return clusterAccessTarget{}, serr
	}

	var children []uuid.UUID
	for _, cluster := range util.Deref(list.JSON200) {
		if cluster.GroupID != nil && *cluster.GroupID == resourceID && cluster.ClusterID != nil {
			children = append(children, *cluster.ClusterID)
		}
	}

	switch len(children) {
	case 0:
		return clusterAccessTarget{}, &util.SummaryWithDetailError{
			Summary: "Failed to resolve cluster ID",
			Detail:  fmt.Sprintf("No cluster or workspace group was found for resource ID %s (tried as cluster ID, as workspace group ID, and as the group that contains a cluster).", resourceID),
		}
	case 1:
		return clusterAccessTarget{endpoint: clusterAccessEndpointV2Cluster, id: children[0]}, nil
	default:
		return clusterAccessTarget{}, &util.SummaryWithDetailError{
			Summary: "Ambiguous cluster group",
			Detail:  fmt.Sprintf("Resource ID %s is not a cluster or a workspace group, and it contains %d clusters. Set resource_id to the specific cluster ID instead.", resourceID, len(children)),
		}
	}
}

// clusterExists reports false only on 404. Other failures are returned so they
// are not mistaken for a missing cluster.
func clusterExists(ctx context.Context, c management.ClientWithResponsesInterface, resourceID uuid.UUID) (bool, error) {
	direct, err := c.GetV2ClustersClusterIDWithResponse(ctx, resourceID, &management.GetV2ClustersClusterIDParams{})
	if serr := util.StatusOK(direct, err, util.ReturnNilOnNotFound); serr != nil {
		return false, serr
	}

	return direct.StatusCode() == http.StatusOK && direct.JSON200 != nil, nil
}

// workspaceGroupExists reports false only on 404. Other failures are returned so
// they are not mistaken for a missing workspace group.
func workspaceGroupExists(ctx context.Context, c management.ClientWithResponsesInterface, resourceID uuid.UUID) (bool, error) {
	group, err := c.GetV1WorkspaceGroupsWorkspaceGroupIDWithResponse(ctx, resourceID, &management.GetV1WorkspaceGroupsWorkspaceGroupIDParams{})
	if serr := util.StatusOK(group, err, util.ReturnNilOnNotFound); serr != nil {
		return false, serr
	}

	return group.StatusCode() == http.StatusOK && group.JSON200 != nil, nil
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
