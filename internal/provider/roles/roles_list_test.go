package roles_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

const testRoleDefinitionsConfig = `
provider "singlestoredb" {
}

data "singlestoredb_roles" "all" {
  resource_type = "Organization"
}
`

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
		url := strings.Join([]string{"/v1/teams", listRolesTeamID.String(), "accessControls"}, "/")
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

var (
	roleDefCreatedAt   = time.Now().UTC()
	roleDefUpdatedAt   = time.Now().UTC()
	roleDefDescription = "A custom role"

	roleDefinitionsList = []management.RoleDefinition{
		{
			Role:         "Owner",
			ResourceType: "Organization",
			Permissions:  []string{"All Permissions"},
			Inherits:     []management.TypedRole{},
			IsCustom:     false,
		},
		{
			Role:         "custom-admin",
			ResourceType: "Organization",
			Description:  &roleDefDescription,
			Permissions:  []string{"View Organization", "Edit Organization"},
			Inherits: []management.TypedRole{
				{ResourceType: "Organization", Role: "Reader"},
			},
			IsCustom:  true,
			CreatedAt: &roleDefCreatedAt,
			UpdatedAt: &roleDefUpdatedAt,
		},
		{
			Role:         "custom-viewer",
			ResourceType: "Organization",
			Description:  &roleDefDescription,
			Permissions:  []string{"View Organization"},
			Inherits:     []management.TypedRole{},
			IsCustom:     true,
			CreatedAt:    &roleDefCreatedAt,
			UpdatedAt:    &roleDefUpdatedAt,
		},
	}
)

func TestReadRoleDefinitions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/roles/Organization", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(roleDefinitionsList))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testRoleDefinitionsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "resource_type", "Organization"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.#", "3"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.0.name", "Owner"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.0.is_custom", "false"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.1.name", "custom-admin"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.1.is_custom", "true"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.1.created_at", util.MaybeTimeValue(roleDefinitionsList[1].CreatedAt).ValueString()),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.1.updated_at", util.MaybeTimeValue(roleDefinitionsList[1].UpdatedAt).ValueString()),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.1.permissions.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.1.inherits.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.2.name", "custom-viewer"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.2.is_custom", "true"),
				),
			},
		},
	})
}

func TestReadRoleDefinitionsBuiltInOnly(t *testing.T) {
	builtInOnlyRolesList := []management.RoleDefinition{
		{
			Role:         "Owner",
			ResourceType: "Organization",
			Permissions:  []string{"All Permissions"},
			IsCustom:     false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/roles/Organization", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(builtInOnlyRolesList))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testRoleDefinitionsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "resource_type", "Organization"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.0.name", "Owner"),
					resource.TestCheckResourceAttr("data.singlestoredb_roles.all", "role_definitions.0.is_custom", "false"),
				),
			},
		},
	})
}
