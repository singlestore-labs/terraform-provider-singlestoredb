package customroles_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

var (
	dsTestCreatedAt   = time.Now().UTC()
	dsTestUpdatedAt   = time.Now().UTC()
	dsTestDescription = "A custom role"

	rolesList = []management.RoleDefinition{
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
			Description:  &dsTestDescription,
			Permissions:  []string{"View Organization", "Edit Organization"},
			Inherits: []management.TypedRole{
				{ResourceType: "Organization", Role: "Reader"},
			},
			IsCustom:  true,
			CreatedAt: &dsTestCreatedAt,
			UpdatedAt: &dsTestUpdatedAt,
		},
		{
			Role:         "custom-viewer",
			ResourceType: "Organization",
			Description:  &dsTestDescription,
			Permissions:  []string{"View Organization"},
			Inherits:     []management.TypedRole{},
			IsCustom:     true,
			CreatedAt:    &dsTestCreatedAt,
			UpdatedAt:    &dsTestUpdatedAt,
		},
	}
)

func TestReadAllRoles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasPrefix(r.URL.Path, "/v1/roles/"))
		require.Equal(t, http.MethodGet, r.Method)

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
				Config: examples.AllRolesListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "resource_type", "Organization"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.#", "3"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.0.name", "Owner"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.0.is_custom", "false"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.1.name", "custom-admin"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.1.is_custom", "true"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.1.created_at", util.MaybeTimeValue(rolesList[1].CreatedAt).ValueString()),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.1.updated_at", util.MaybeTimeValue(rolesList[1].UpdatedAt).ValueString()),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.1.permissions.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.1.inherits.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.2.name", "custom-viewer"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.2.is_custom", "true"),
				),
			},
		},
	})
}

func TestReadAllRolesWithBuiltInOnly(t *testing.T) {
	builtInOnlyRolesList := []management.RoleDefinition{
		{
			Role:         "Owner",
			ResourceType: "Organization",
			Permissions:  []string{"All Permissions"},
			IsCustom:     false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.True(t, strings.HasPrefix(r.URL.Path, "/v1/roles/"))
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
				Config: examples.AllRolesListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "resource_type", "Organization"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.0.name", "Owner"),
					resource.TestCheckResourceAttr("data.singlestoredb_all_roles.all", "roles.0.is_custom", "false"),
				),
			},
		},
	})
}
