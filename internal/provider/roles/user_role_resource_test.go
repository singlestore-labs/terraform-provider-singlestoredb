package roles_test

import (
	"encoding/json"
	"io"
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
	userID   = uuid.MustParse("17290909-3016-4f63-b601-e30410f1b05f")
	teamID   = uuid.MustParse("c2757c25-26d2-434a-91ee-f47683e6cdb3")
	orgID    = uuid.MustParse("26c13376-f8a4-469b-b93d-a85d1469e3f9")
	testRole = management.IdentityRole{
		Role:         "Owner",
		ResourceID:   teamID,
		ResourceType: "Team",
	}
	grantRoleOnUpdate = management.IdentityRole{
		Role:         "Owner",
		ResourceID:   orgID,
		ResourceType: "Organization",
	}

	outsideRole = management.IdentityRole{
		Role:         "Reader",
		ResourceID:   uuid.MustParse("73fded30-f4dc-4348-bc5c-dbca46832e04"),
		ResourceType: "Team",
	}
)

func TestGrantRevokeUserRole(t *testing.T) {
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
	}

	teamsAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Team", teamID)
	}

	organizationAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Organization", orgID)
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		teamsAccessControlsPatchHandler,        // grant team
		teamsAccessControlsPatchHandler,        // revoke team
		organizationAccessControlsPatchHandler, // grant update to org
		organizationAccessControlsPatchHandler, // revoke org
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
				Config: examples.UserRoleResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "user_id", userID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.resource_type", testRole.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.resource_id", testRole.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.role_name", testRole.Role),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.UserRoleResource).
					WithUserRoleResource("this")("role", cty.ObjectVal(map[string]cty.Value{
					"resource_type": cty.StringVal(grantRoleOnUpdate.ResourceType),
					"role_name":     cty.StringVal(grantRoleOnUpdate.Role),
					"resource_id":   cty.StringVal(grantRoleOnUpdate.ResourceID.String()),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "user_id", userID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.resource_type", grantRoleOnUpdate.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.resource_id", grantRoleOnUpdate.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.role_name", grantRoleOnUpdate.Role),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestGrantAlreadyGrantedUserRole(t *testing.T) {
	grantedRoles := []management.IdentityRole{testRole}
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
	}

	teamsAccessControlsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		accessControlsPatchHandler(t, w, r, &grantedRoles, "Team", teamID)
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
				Config: examples.UserRoleResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "user_id", userID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.resource_type", testRole.ResourceType),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.resource_id", testRole.ResourceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.role_name", testRole.Role),
				),
			},
		},
	})
}

func accessControlsPatchHandler(t *testing.T, w http.ResponseWriter, r *http.Request, grantedRoles *[]management.IdentityRole, resourceType string, resourceID uuid.UUID) {
	t.Helper()
	var url string
	switch resourceType {
	case "Organization":
		url = strings.Join([]string{"/v1beta/organizations", resourceID.String(), "accessControls"}, "/")
	case "Team":
		url = strings.Join([]string{"/v1beta/teams", resourceID.String(), "accessControls"}, "/")
	case "Secret":
		url = strings.Join([]string{"/v1beta/secrets", resourceID.String(), "accessControls"}, "/")
	case "Cluster":
		url = strings.Join([]string{"/v1beta/workspaceGroups", resourceID.String(), "accessControls"}, "/")
	default:
		t.Fatalf("%s resource type is not supported", resourceType)
	}
	require.Equal(t, url, r.URL.Path)
	require.Equal(t, http.MethodPatch, r.Method)

	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)

	var input management.ControlAccessAction
	require.NoError(t, json.Unmarshal(body, &input))

	handleGrants(grantedRoles, input.Grants, resourceType, resourceID)
	handleRevokes(grantedRoles, input.Revokes, resourceType, resourceID)

	w.Header().Add("Content-Type", "json")
	_, err = w.Write(testutil.MustJSON(
		struct {
			ResourceID uuid.UUID
		}{
			ResourceID: resourceID,
		},
	))
	require.NoError(t, err)
}

func handleGrants(grantedRoles *[]management.IdentityRole, grants []management.ControlAccessRole, resourceType string, resourceID uuid.UUID) {
	for _, grant := range grants {
		newGrant := management.IdentityRole{
			Role:         grant.Role,
			ResourceID:   resourceID,
			ResourceType: resourceType,
		}
		*grantedRoles = append(*grantedRoles, newGrant)
	}
}

func handleRevokes(grantedRoles *[]management.IdentityRole, revokes []management.ControlAccessRole, resourceType string, resourceID uuid.UUID) {
	for _, revoke := range revokes {
		for i, role := range *grantedRoles {
			if role.Role == revoke.Role && role.ResourceID == resourceID && role.ResourceType == resourceType {
				*grantedRoles = append((*grantedRoles)[:i], (*grantedRoles)[i+1:]...)

				break
			}
		}
	}
}

func TestGrantRevokeUserRoleIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.UserRoleResourceIntegration,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.resource_type", "Team"),
					resource.TestCheckResourceAttr("singlestoredb_user_role.this", "role.role_name", "Owner"),
				),
			},
		},
	})
}
