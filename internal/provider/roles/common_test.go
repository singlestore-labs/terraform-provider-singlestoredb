package roles_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/roles"
	"github.com/stretchr/testify/assert"
)

func TestGroupRolesByResourceID(t *testing.T) {
	resourceID1 := types.StringValue(uuid.New().String())
	resourceID2 := types.StringValue(uuid.New().String())

	testRoles := []roles.RoleAttributesModel{
		{
			RoleName:     types.StringValue("role1"),
			ResourceType: types.StringValue("Organization"),
			ResourceID:   resourceID1,
		},
		{
			RoleName:     types.StringValue("role2"),
			ResourceType: types.StringValue("WorkspaceGroup"),
			ResourceID:   resourceID2,
		},
		{
			RoleName:     types.StringValue("role3"),
			ResourceType: types.StringValue("Team"),
			ResourceID:   resourceID1,
		},
	}

	groupedRoles := roles.GroupRolesByResourceID(testRoles)

	assert.Len(t, groupedRoles, 2)
	assert.Len(t, groupedRoles[resourceID1], 2)
	assert.Len(t, groupedRoles[resourceID2], 1)
}

func TestGroupRolesByResourceType(t *testing.T) {
	testRoles := []roles.RoleAttributesModel{
		{
			RoleName:     types.StringValue("role1"),
			ResourceType: types.StringValue("Organization"),
			ResourceID:   types.StringValue(uuid.New().String()),
		},
		{
			RoleName:     types.StringValue("role2"),
			ResourceType: types.StringValue("WorkspaceGroup"),
			ResourceID:   types.StringValue(uuid.New().String()),
		},
		{
			RoleName:     types.StringValue("role3"),
			ResourceType: types.StringValue("Organization"),
			ResourceID:   types.StringValue(uuid.New().String()),
		},
	}

	groupedRoles := roles.GroupRolesByResourceType(testRoles)

	assert.Len(t, groupedRoles, 2)
	assert.Len(t, groupedRoles[roles.ResourceTypeString(types.StringValue("Organization"))], 2)
	assert.Len(t, groupedRoles[roles.ResourceTypeString(types.StringValue("WorkspaceGroup"))], 1)
}

func TestSubtractRoles(t *testing.T) {
	rolesA := []roles.RoleAttributesModel{
		{
			RoleName:     types.StringValue("role1"),
			ResourceType: types.StringValue("Organization"),
			ResourceID:   types.StringValue("resource1"),
		},
		{
			RoleName:     types.StringValue("role2"),
			ResourceType: types.StringValue("WorkspaceGroup"),
			ResourceID:   types.StringValue("resource2"),
		},
	}

	rolesB := []roles.RoleAttributesModel{
		{
			RoleName:     types.StringValue("role1"),
			ResourceType: types.StringValue("Organization"),
			ResourceID:   types.StringValue("resource1"),
		},
	}

	result := roles.SubtractRoles(rolesA, rolesB)

	assert.Len(t, result, 1)
	assert.Equal(t, types.StringValue("role2"), result[0].RoleName)

	result = roles.SubtractRoles(rolesB, rolesA)

	assert.Len(t, result, 0)
}

func TestIsRoleChanged(t *testing.T) {
	roleA := roles.RoleAttributesModel{
		RoleName:     types.StringValue("role1"),
		ResourceType: types.StringValue("Organization"),
		ResourceID:   types.StringValue("resource1"),
	}

	roleB := roles.RoleAttributesModel{
		RoleName:     types.StringValue("role1"),
		ResourceType: types.StringValue("Organization"),
		ResourceID:   types.StringValue("resource1"),
	}

	roleC := roles.RoleAttributesModel{
		RoleName:     types.StringValue("role2"),
		ResourceType: types.StringValue("WorkspaceGroup"),
		ResourceID:   types.StringValue("resource2"),
	}

	assert.False(t, roles.IsRoleChanged(roleA, roleB))
	assert.True(t, roles.IsRoleChanged(roleA, roleC))
}

func TestMatchedRoles(t *testing.T) {
	rolesA := []roles.RoleAttributesModel{
		{
			RoleName:     types.StringValue("role1"),
			ResourceType: types.StringValue("Organization"),
			ResourceID:   types.StringValue("resource1"),
		},
		{
			RoleName:     types.StringValue("role2"),
			ResourceType: types.StringValue("WorkspaceGroup"),
			ResourceID:   types.StringValue("resource2"),
		},
		{
			RoleName:     types.StringValue("role3"),
			ResourceType: types.StringValue("Team"),
			ResourceID:   types.StringValue("resource3"),
		},
	}

	rolesB := []roles.RoleAttributesModel{
		{
			RoleName:     types.StringValue("role1"),
			ResourceType: types.StringValue("Organization"),
			ResourceID:   types.StringValue("resource1"),
		},
		{
			RoleName:     types.StringValue("role3"),
			ResourceType: types.StringValue("Team"),
			ResourceID:   types.StringValue("resource3"),
		},
	}

	result := roles.MatchedRoles(rolesA, rolesB)

	assert.Len(t, result, 2)
	assert.Equal(t, types.StringValue("role1"), result[0].RoleName)
	assert.Equal(t, types.StringValue("role3"), result[1].RoleName)

	result = roles.MatchedRoles(rolesB, rolesA)

	assert.Len(t, result, 2)
	assert.Equal(t, types.StringValue("role1"), result[0].RoleName)
	assert.Equal(t, types.StringValue("role3"), result[1].RoleName)

	rolesC := []roles.RoleAttributesModel{
		{
			RoleName:     types.StringValue("role4"),
			ResourceType: types.StringValue("Secret"),
			ResourceID:   types.StringValue("resource4"),
		},
	}

	result = roles.MatchedRoles(rolesA, rolesC)

	assert.Len(t, result, 0)
}

type MockClientWithResponses struct {
	management.ClientWithResponsesInterface
}

func (m *MockClientWithResponses) GetV1OrganizationsCurrentWithResponse(ctx context.Context, _ ...management.RequestEditorFn) (*management.GetV1OrganizationsCurrentResponse, error) {
	return &management.GetV1OrganizationsCurrentResponse{
		JSON200: &management.Organization{
			OrgID: uuid.New(),
		},
	}, nil
}
