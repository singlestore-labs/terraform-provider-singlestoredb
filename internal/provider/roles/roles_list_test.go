package roles_test

import (
	"net/http"
	"net/http/httptest"
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

var (
	listRolesTeamID = uuid.MustParse("24f31e2d-847f-4a62-9a93-a10e9bcd0dae")
	rolesList       = []management.ResourceRole{
		{
			Role: "Owner",
		},
		{
			Role: "Writer",
		},
		{
			Role: "Reader",
		},
		{
			Role: "Operator",
		},
	}
)

func TestReadRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		url := strings.Join([]string{"/v1beta/teams", listRolesTeamID.String(), "accessControls"}, "/")
		require.Equal(t, url, r.URL.Path)
		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(rolesList))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.RolesListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "resource_id", listRolesTeamID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "roles.#", "4"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "roles.0", rolesList[0].Role),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "roles.1", rolesList[1].Role),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "roles.2", rolesList[2].Role),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "roles.3", rolesList[3].Role),
				),
			},
		},
	})
}
