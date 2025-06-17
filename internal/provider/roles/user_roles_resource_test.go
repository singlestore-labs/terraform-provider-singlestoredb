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
	testRole1 = management.IdentityRole{
		Role:         "Owner",
		ResourceID:   uuid.MustParse("d4f34ce0-e79f-46e4-994f-3da004d98bff"),
		ResourceType: "Team",
	}
	testRole2 = management.IdentityRole{
		Role:         "Operator",
		ResourceID:   uuid.MustParse("c7e83804-2e49-4dcf-bbd4-27fd7ad28d5d"),
		ResourceType: "Organization",
	}
)

func TestGrantRevokeUserRoles(t *testing.T) {
	grantedRoles := []management.IdentityRole{outsideRole}
	identityRolesHandler := func(w http.ResponseWriter, r *http.Request) bool {
		url := strings.Join([]string{"/v1beta/users", userID.String(), "identityRoles"}, "/")
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
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Team", testRole1.ResourceID)
	}

	organizationAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Organization", testRole2.ResourceID)
	}

	organizationUpdateAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Organization", grantRoleOnUpdate.ResourceID)
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		organizationAccessControlsPatchHandler, // grant org
		teamsAccessControlsPatchHandler,        // grant team

		organizationAccessControlsPatchHandler,       // revoke org on update
		teamsAccessControlsPatchHandler,              // revoke team on update
		organizationUpdateAccessControlsPatchHandler, // grant org on update

		organizationUpdateAccessControlsPatchHandler, // revoke updated org
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
				Config: examples.UserRolesResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "user_id", userID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.resource_type", testRole1.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.resource_id", testRole1.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.role_name", testRole1.Role),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.1.resource_type", testRole2.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.1.resource_id", testRole2.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.1.role_name", testRole2.Role),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.UserRolesResource).
					WithUserRolesResource("this")("roles", cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"resource_type": cty.StringVal(grantRoleOnUpdate.ResourceType),
					"role_name":     cty.StringVal(grantRoleOnUpdate.Role),
					"resource_id":   cty.StringVal(grantRoleOnUpdate.ResourceID.String()),
				})})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "user_id", userID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.resource_type", grantRoleOnUpdate.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.resource_id", grantRoleOnUpdate.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.role_name", grantRoleOnUpdate.Role),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestGrantRevokeUserRolesIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.UserRolesResourceIntegration,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.resource_type", "Team"),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.0.role_name", "Owner"),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.1.resource_type", "Cluster"),
					resource.TestCheckResourceAttr("singlestoredb_user_roles.this", "roles.1.role_name", "Owner"),
				),
			},
		},
	})
}
