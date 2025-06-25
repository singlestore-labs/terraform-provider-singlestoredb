package roles_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

var testTeamID = uuid.MustParse("d123ec69-936c-4e71-92bb-a45d987f9118")

func TestReadTeamRoles(t *testing.T) {
	roles := []management.IdentityRole{
		{
			Role:         "Owner",
			ResourceType: "Organization",
			ResourceID:   uuid.MustParse("c13c5dfb-5040-4c3d-9168-fed13f5082c3"),
		},
		{
			Role:         "Writer",
			ResourceType: "Team",
			ResourceID:   uuid.MustParse("37e928fd-b9f3-4f2b-b022-1593484b086c"),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := strings.Join([]string{"/v1beta/teams", testTeamID.String(), "identityRoles"}, "/")
		require.Equal(t, url, r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(roles))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.TeamRolesListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "team_id", testTeamID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "roles.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "roles.0.resource_type", roles[0].ResourceType),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "roles.0.resource_id", roles[0].ResourceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "roles.0.role_name", roles[0].Role),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "roles.1.resource_type", roles[1].ResourceType),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "roles.1.resource_id", roles[1].ResourceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_team_roles.all", "roles.1.role_name", roles[1].Role),
				),
			},
		},
	})
}

func TestReadTeamRolesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.TeamRolesListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}
