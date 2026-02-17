package roles_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

var (
	testTeamRoleTeamID      = uuid.MustParse("93e40aef-df01-467b-944c-3b091afd304e")
	testTeamRoleGrantedTeam = management.IdentityRole{
		Role:         "Owner",
		ResourceID:   uuid.MustParse("f1e01e30-440c-4633-a165-77589419eb42"),
		ResourceType: "Team",
	}

	testTeamRoleGrantedOrganization = management.IdentityRole{
		Role:         "Operator",
		ResourceID:   uuid.MustParse("24f31e2d-847f-4a62-9a93-a10e9bcd0dae"),
		ResourceType: "Organization",
	}

	grantSecretRoleToTeamOnUpdate = management.IdentityRole{
		Role:         "Owner",
		ResourceID:   uuid.MustParse("1f7e629e-cb3c-4e73-bba7-dae1e9277e96"),
		ResourceType: "Secret",
	}
	grantWorkspaceGroupRoleToTeamOnUpdate = management.IdentityRole{
		Role:         "Reader",
		ResourceID:   uuid.MustParse("2f61efb9-2a61-4883-bfd9-2508b748c1d0"),
		ResourceType: "Cluster",
	}
)

func TestGrantRevokeTeamRoles(t *testing.T) {
	grantedRoles := []management.IdentityRole{}
	identityRolesHandler := func(w http.ResponseWriter, r *http.Request) bool {
		url := strings.Join([]string{"/v1/teams", testTeamRoleTeamID.String(), "identityRoles"}, "/")
		if r.URL.Path != url || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(grantedRoles))
		require.NoError(t, err)

		return true
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		identityRolesHandler,
		identityRolesHandler,
	}

	teamsAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Team", testTeamRoleGrantedTeam.ResourceID)
	}

	organizationAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Organization", testTeamRoleGrantedOrganization.ResourceID)
	}

	secretsUpdateAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Secret", grantSecretRoleToTeamOnUpdate.ResourceID)
	}

	workspaceGroupsUpdateAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Cluster", grantWorkspaceGroupRoleToTeamOnUpdate.ResourceID)
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		organizationAccessControlsPatchHandler, // grant org
		teamsAccessControlsPatchHandler,        // grant team

		organizationAccessControlsPatchHandler, // revoke org on update
		teamsAccessControlsPatchHandler,        // revoke team on update

		workspaceGroupsUpdateAccessControlsPatchHandler, // grant workspace group on update
		secretsUpdateAccessControlsPatchHandler,         // grant secret on update

		workspaceGroupsUpdateAccessControlsPatchHandler, // revoke workspace group on update
		secretsUpdateAccessControlsPatchHandler,         // revoke secret on update
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}

		h := writeHandlers[0]

		h(w, r)

		writeHandlers = writeHandlers[1:]
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.TeamRolesResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "team_id", testTeamRoleTeamID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.resource_type", testTeamRoleGrantedTeam.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.resource_id", testTeamRoleGrantedTeam.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.role_name", testTeamRoleGrantedTeam.Role),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.resource_type", testTeamRoleGrantedOrganization.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.resource_id", testTeamRoleGrantedOrganization.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.role_name", testTeamRoleGrantedOrganization.Role),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.TeamRolesResource).
					WithTeamRolesResource("this")("roles", cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"resource_type": cty.StringVal(grantSecretRoleToTeamOnUpdate.ResourceType),
					"role_name":     cty.StringVal(grantSecretRoleToTeamOnUpdate.Role),
					"resource_id":   cty.StringVal(grantSecretRoleToTeamOnUpdate.ResourceID.String()),
				}), cty.ObjectVal(map[string]cty.Value{
					"resource_type": cty.StringVal(grantWorkspaceGroupRoleToTeamOnUpdate.ResourceType),
					"role_name":     cty.StringVal(grantWorkspaceGroupRoleToTeamOnUpdate.Role),
					"resource_id":   cty.StringVal(grantWorkspaceGroupRoleToTeamOnUpdate.ResourceID.String()),
				})})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "team_id", testTeamRoleTeamID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.resource_type", grantSecretRoleToTeamOnUpdate.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.resource_id", grantSecretRoleToTeamOnUpdate.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.role_name", grantSecretRoleToTeamOnUpdate.Role),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.resource_type", grantWorkspaceGroupRoleToTeamOnUpdate.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.resource_id", grantWorkspaceGroupRoleToTeamOnUpdate.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.role_name", grantWorkspaceGroupRoleToTeamOnUpdate.Role),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestGrantRevokeTeamRolesIntegration(t *testing.T) {
	t1Name := testutil.GenerateUniqueResourceName("t1")
	t2Name := testutil.GenerateUniqueResourceName("t2")

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.TeamRolesResourceIntegration).
					WithTeamResource("t1")("name", cty.StringVal(t1Name)).
					WithTeamResource("t2")("name", cty.StringVal(t2Name)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.resource_type", "Team"),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.0.role_name", "Owner"),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.resource_type", "Cluster"),
					resource.TestCheckResourceAttr("singlestoredb_team_roles.this", "roles.1.role_name", "Owner"),
				),
			},
		},
	})
}
