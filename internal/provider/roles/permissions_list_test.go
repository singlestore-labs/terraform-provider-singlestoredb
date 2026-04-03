package roles_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

const testRolePermissionsConfig = `
provider "singlestoredb" {
}

data "singlestoredb_role_permissions" "org" {
  resource_type = "Organization"
}
`

var permissionsRolesList = []management.RoleDefinition{
	{
		Role:         "Owner",
		ResourceType: "Organization",
		Permissions:  []string{"View Virtual Workspaces", "Edit Organization", "Delete Organization", "Manage Members"},
		IsCustom:     false,
	},
	{
		Role:         "Reader",
		ResourceType: "Organization",
		Permissions:  []string{"View Virtual Workspaces"},
		IsCustom:     false,
	},
}

func TestReadRolePermissions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/roles/Organization", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(permissionsRolesList))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testRolePermissionsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", "resource_type", "Organization"),
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", "permissions.#", "4"),
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", "permissions.0", permissionsRolesList[0].Permissions[0]),
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", "permissions.1", permissionsRolesList[0].Permissions[1]),
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", "permissions.2", permissionsRolesList[0].Permissions[2]),
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", "permissions.3", permissionsRolesList[0].Permissions[3]),
				),
			},
		},
	})
}

func TestReadRolePermissionsOwnerNotFound(t *testing.T) {
	rolesWithoutOwner := []management.RoleDefinition{
		{
			Role:         "Reader",
			ResourceType: "Organization",
			Permissions:  []string{"View Virtual Workspaces"},
			IsCustom:     false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/roles/Organization", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(rolesWithoutOwner))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      testRolePermissionsConfig,
				ExpectError: regexp.MustCompile("Failed to find Owner role"),
			},
		},
	})
}

func TestReadRolePermissionsIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testRolePermissionsConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_role_permissions.org", "resource_type", "Organization"),
					resource.TestCheckResourceAttrSet("data.singlestoredb_role_permissions.org", "permissions.#"),
					resource.TestCheckResourceAttrSet("data.singlestoredb_role_permissions.org", "permissions.0"),
				),
			},
		},
	})
}
