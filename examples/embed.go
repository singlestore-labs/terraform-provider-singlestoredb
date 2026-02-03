package examples

import (
	"embed"
	"fmt"
)

//go:embed data-sources/*/data-source.tf resources/*/resource.tf
var f embed.FS

var (
	Regions                          = mustRead("data-sources/singlestoredb_regions/data-source.tf")
	RegionsV2                        = mustRead("data-sources/singlestoredb_regions_v2/data-source.tf")
	PrivateConnectionsGetDataSource  = mustRead("data-sources/singlestoredb_private_connection/data-source.tf")
	PrivateConnectionsResource       = mustRead("resources/singlestoredb_private_connection/resource.tf")
	PrivateConnectionsListDataSource = mustRead("data-sources/singlestoredb_private_connections/data-source.tf")
	WorkspaceGroupsListDataSource    = mustRead("data-sources/singlestoredb_workspace_groups/data-source.tf")
	WorkspaceGroupsGetDataSource     = mustRead("data-sources/singlestoredb_workspace_group/data-source.tf")
	WorkspacesListDataSource         = mustRead("data-sources/singlestoredb_workspaces/data-source.tf")
	WorkspacesGetDataSource          = mustRead("data-sources/singlestoredb_workspace/data-source.tf")
	WorkspaceGroupsResource          = mustRead("resources/singlestoredb_workspace_group/resource.tf")
	WorkspacesResource               = mustRead("resources/singlestoredb_workspace/resource.tf")
	UserGetDataSource                = mustRead("data-sources/singlestoredb_user/data-source.tf")
	UserResource                     = mustRead("resources/singlestoredb_user/resource.tf")
	UserListDataSource               = mustRead("data-sources/singlestoredb_users/data-source.tf")
	InvitationsGetDataSource         = mustRead("data-sources/singlestoredb_invitation/data-source.tf")
	InvitationsListDataSource        = mustRead("data-sources/singlestoredb_invitations/data-source.tf")
	TeamsGetDataSource               = mustRead("data-sources/singlestoredb_team/data-source.tf")
	TeamsResource                    = mustRead("resources/singlestoredb_team/resource.tf")
	TeamsListDataSource              = mustRead("data-sources/singlestoredb_teams/data-source.tf")
	UserRoleResource                 = mustRead("resources/singlestoredb_user_role/resource.tf")
	UserRolesResource                = mustRead("resources/singlestoredb_user_roles/resource.tf")
	UserRoleResourceIntegration      = mustRead("resources/singlestoredb_user_role_integration/resource.tf")
	UserRolesListDataSource          = mustRead("data-sources/singlestoredb_user_roles/data-source.tf")
	UserRolesResourceIntegration     = mustRead("resources/singlestoredb_user_roles_integration/resource.tf")
	RolesListDataSource              = mustRead("data-sources/singlestoredb_roles/data-source.tf")
	TeamRolesListDataSource          = mustRead("data-sources/singlestoredb_team_roles/data-source.tf")
	TeamRoleResource                 = mustRead("resources/singlestoredb_team_role/resource.tf")
	TeamRoleResourceIntegration      = mustRead("resources/singlestoredb_team_role_integration/resource.tf")
	TeamRolesResource                = mustRead("resources/singlestoredb_team_roles/resource.tf")
	TeamRolesResourceIntegration     = mustRead("resources/singlestoredb_team_roles_integration/resource.tf")
	FlowGetDataSource        = mustRead("data-sources/singlestoredb_flow_instance/data-source.tf")
	FlowListDataSource       = mustRead("data-sources/singlestoredb_flow_instances/data-source.tf")
	FlowResource             = mustRead("resources/singlestoredb_flow_instance/resource.tf")
)

func mustRead(path string) string {
	result, err := f.ReadFile(path)
	if err != nil {
		panic(err)
	}

	if string(result) == "" {
		panic(fmt.Sprintf("path '%s' should have content but is empty", path))
	}

	return string(result)
}
