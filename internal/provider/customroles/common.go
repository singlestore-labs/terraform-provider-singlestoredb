package customroles

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

type ResourceType string

const (
	ResourceTypeOrganization ResourceType = "Organization"
	// ResourceTypeWorkspaceGroup uses "Cluster" as the user-facing API value,
	// while the internal platform name is "WorkspaceGroup".
	ResourceTypeWorkspaceGroup ResourceType = "Cluster"
	ResourceTypeTeam           ResourceType = "Team"
	ResourceTypeSecret         ResourceType = "Secret"
)

var ResourceTypeList = []ResourceType{
	ResourceTypeOrganization,
	ResourceTypeWorkspaceGroup,
	ResourceTypeTeam,
	ResourceTypeSecret,
}

type InheritedRoleModel struct {
	ResourceType types.String `tfsdk:"resource_type"`
	Role         types.String `tfsdk:"role"`
}

func resourceTypeNames() string {
	names := make([]string, 0, len(ResourceTypeList))
	for _, rt := range ResourceTypeList {
		names = append(names, string(rt))
	}

	return strings.Join(names, ", ")
}

func convertPermissions(permissions []string) []types.String {
	result := make([]types.String, 0, len(permissions))
	for _, perm := range permissions {
		result = append(result, types.StringValue(perm))
	}

	return result
}

func convertInherits(inherits []management.TypedRole) []InheritedRoleModel {
	result := make([]InheritedRoleModel, 0, len(inherits))
	for _, inherit := range inherits {
		result = append(result, InheritedRoleModel{
			ResourceType: types.StringValue(inherit.ResourceType),
			Role:         types.StringValue(inherit.Role),
		})
	}

	return result
}

func setOptionalRoleFields(role *management.RoleDefinition) (types.String, types.String, types.String) {
	description := types.StringNull()
	createdAt := types.StringNull()
	updatedAt := types.StringNull()

	if role.Description != nil {
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

func formatResourceTypeList() string {
	return fmt.Sprintf("Valid resource types are: %s", resourceTypeNames())
}
