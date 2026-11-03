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
	testTeamEntityID = uuid.MustParse("f820a472-ab16-4fdd-ac09-79ea5321844f")
	testRoleTeamID   = uuid.MustParse("c2757c25-26d2-434a-91ee-f47683e6cdb3")
)

func TestGrantRevokeTeamRole(t *testing.T) {
	grantedRoles := []management.IdentityRole{}
	identityRolesHandler := func(w http.ResponseWriter, r *http.Request) bool {
		url := strings.Join([]string{"/v1/teams", testTeamEntityID.String(), "identityRoles"}, "/")
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
	}

	teamsAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Team", testRoleTeamID)
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		teamsAccessControlsPatchHandler, // grant team
		teamsAccessControlsPatchHandler, // revoke team
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
				Config: examples.TeamRoleResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team_role.this", "team_id", testTeamEntityID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_role.this", "role.resource_type", testRole.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_team_role.this", "role.resource_id", testRole.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_team_role.this", "role.role_name", testRole.Role),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestGrantRevokeTeamRoleIntegration(t *testing.T) {
	t1Name := testutil.GenerateUniqueResourceName("t1")
	t2Name := testutil.GenerateUniqueResourceName("t2")

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.TeamRoleResourceIntegration).
					WithTeamResource("t1")("name", cty.StringVal(t1Name)).
					WithTeamResource("t2")("name", cty.StringVal(t2Name)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_team_role.this", "role.resource_type", "Team"),
					resource.TestCheckResourceAttr("singlestoredb_team_role.this", "role.role_name", "Owner"),
				),
			},
		},
	})
}
