package customroles_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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
	"github.com/zclconf/go-cty/cty"
)

var (
	testRoleName           = "custom-reader"
	testResourceType       = "Organization"
	testDescription        = "A custom role with read-only permissions"
	testUpdatedDescription = "An updated custom role description"
	testPermissions        = []string{"View Organization"}
	testInherits           = []management.TypedRole{
		{
			ResourceType: "Organization",
			Role:         "Reader",
		},
	}
	testCreatedAt = time.Now().UTC()
	testUpdatedAt = time.Now().UTC()

	testRoleDefinition = management.RoleDefinition{
		Role:         testRoleName,
		ResourceType: testResourceType,
		Description:  &testDescription,
		Permissions:  testPermissions,
		Inherits:     testInherits,
		IsCustom:     true,
		CreatedAt:    &testCreatedAt,
		UpdatedAt:    &testUpdatedAt,
	}

	testBuiltInRoleDefinition = management.RoleDefinition{
		Role:         "Owner",
		ResourceType: testResourceType,
		Permissions:  []string{"All Permissions"},
		Inherits:     []management.TypedRole{},
		IsCustom:     false,
	}
)

var updatedDescriptionConfig = `provider "singlestoredb" {
}

resource "singlestoredb_role" "example" {
  name          = "custom-reader"
  resource_type = "Organization"
  description   = "An updated custom role description"

  permissions = [
    "View Organization",
  ]

  inherits = [
    {
      resource_type = "Organization"
      role          = "Reader"
    }
  ]
}

output "role" {
  value = singlestoredb_role.example
}
`

func TestCreateReadUpdateDeleteCustomRole(t *testing.T) {
	currentRole := testRoleDefinition

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var createReq management.RoleCreate
			require.NoError(t, json.Unmarshal(body, &createReq))

			currentRole = management.RoleDefinition{
				Role:         createReq.Role,
				ResourceType: testResourceType,
				Description:  createReq.Description,
				Permissions:  createReq.Permissions,
				Inherits:     createReq.Inherits,
				IsCustom:     true,
				CreatedAt:    &testCreatedAt,
				UpdatedAt:    &testUpdatedAt,
			}

			_, err = w.Write(testutil.MustJSON(currentRole))
			require.NoError(t, err)

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, testRoleName):
			_, err := w.Write(testutil.MustJSON(currentRole))
			require.NoError(t, err)

		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, testRoleName):
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var updateReq management.RoleUpdate
			require.NoError(t, json.Unmarshal(body, &updateReq))

			currentRole.Description = updateReq.Description
			currentRole.Permissions = updateReq.Permissions
			currentRole.Inherits = updateReq.Inherits
			now := time.Now().UTC()
			currentRole.UpdatedAt = &now

			_, err = w.Write(testutil.MustJSON(currentRole))
			require.NoError(t, err)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, testRoleName):
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`true`))
			require.NoError(t, err)

		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.RoleResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_role.example", "name", testRoleName),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "resource_type", testResourceType),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "description", testDescription),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "is_custom", "true"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "created_at", util.MaybeTimeValue(testRoleDefinition.CreatedAt).ValueString()),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "updated_at", util.MaybeTimeValue(testRoleDefinition.UpdatedAt).ValueString()),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "permissions.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "permissions.0", "View Organization"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "inherits.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "inherits.0.resource_type", "Organization"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "inherits.0.role", "Reader"),
				),
			},
			{
				Config: updatedDescriptionConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_role.example", "name", testRoleName),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "resource_type", testResourceType),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "description", testUpdatedDescription),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "is_custom", "true"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "permissions.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "permissions.0", "View Organization"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "inherits.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "inherits.0.resource_type", "Organization"),
					resource.TestCheckResourceAttr("singlestoredb_role.example", "inherits.0.role", "Reader"),
				),
			},
		},
	})
}

func TestImportCustomRole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			_, err := w.Write(testutil.MustJSON(testRoleDefinition))
			require.NoError(t, err)

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, testRoleName):
			_, err := w.Write(testutil.MustJSON(testRoleDefinition))
			require.NoError(t, err)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, testRoleName):
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`true`))
			require.NoError(t, err)

		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.RoleResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_role.example", "name", testRoleName),
				),
			},
			{
				ResourceName:      "singlestoredb_role.example",
				ImportState:       true,
				ImportStateId:     "Organization/custom-reader",
				ImportStateVerify: true,
			},
		},
	})
}

func TestImportBuiltInRoleFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/roles/"):
			_, err := w.Write(testutil.MustJSON(testRoleDefinition))
			require.NoError(t, err)

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, testRoleName):
			_, err := w.Write(testutil.MustJSON(testRoleDefinition))
			require.NoError(t, err)

		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, testBuiltInRoleDefinition.Role):
			_, err := w.Write(testutil.MustJSON(testBuiltInRoleDefinition))
			require.NoError(t, err)

		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, testRoleName):
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte(`true`))
			require.NoError(t, err)

		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.RoleResource,
			},
			{
				ResourceName:      "singlestoredb_role.example",
				ImportState:       true,
				ImportStateId:     "Organization/Owner",
				ImportStateVerify: false,
				ExpectError:       regexp.MustCompile("is a built-in role"),
			},
		},
	})
}

func TestCustomRoleInvalidInheritedResourceType(t *testing.T) {
	configWithInvalidInheritedRoleType := `provider "singlestoredb" {
  api_key         = "bar"
  api_service_url = "https://example.com"
}

resource "singlestoredb_role" "example" {
  name          = "custom-reader"
  resource_type = "Organization"
  description   = "A custom role with read-only permissions"

  permissions = [
    "View Organization",
  ]

  inherits = [
    {
      resource_type = "Invalid"
      role          = "Reader"
    }
  ]
}
`

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: "https://example.com",
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      configWithInvalidInheritedRoleType,
				ExpectError: regexp.MustCompile(`Attribute inherits\[0\]\.resource_type value must be one of`),
			},
		},
	})
}

func TestCustomRoleIntegration(t *testing.T) {
	uniqueRoleName := testutil.GenerateUniqueResourceName("custom-role")

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.RoleResourceIntegration).
					WithRoleResource("test")("name", cty.StringVal(uniqueRoleName)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_role.test", "name", uniqueRoleName),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "resource_type", "Organization"),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "is_custom", "true"),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "inherits.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "inherits.0.resource_type", "Organization"),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "inherits.0.role", "Reader"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.RoleResourceIntegration).
					WithRoleResource("test")("name", cty.StringVal(uniqueRoleName)).
					WithRoleResource("test")("description", cty.StringVal("Updated integration test description")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_role.test", "name", uniqueRoleName),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "resource_type", "Organization"),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "description", "Updated integration test description"),
					resource.TestCheckResourceAttr("singlestoredb_role.test", "is_custom", "true"),
				),
			},
		},
	})
}
