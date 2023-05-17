package workspacegroups_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
	"github.com/zclconf/go-cty/cty"
)

var (
	updatedWorkspaceGroupName = strings.Join([]string{"updated", config.TestInitialWorkspaceGroupName}, "-")
	updatedAdminPassword      = "buzzBAR123$"
)

func TestCRUDWorkspaceGroup(t *testing.T) {
	regions := []management.Region{
		{
			RegionID: uuid.MustParse("2ca3d358-021d-45ed-86cb-38b8d14ac507"),
			Region:   "GS - US West 2 (Oregon) - aws-oregon-gs1",
			Provider: management.AWS,
		},
	}

	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")

	workspaceGroup := management.WorkspaceGroup{
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   util.Ptr([]string{config.TestInitialFirewallRange}),
		Name:             config.TestInitialWorkspaceGroupName,
		RegionID:         regions[0].RegionID,
		State:            management.PENDING, // During the first poll, the status will still be PENDING.
		TerminatedAt:     nil,
		UpdateWindow:     nil,
		WorkspaceGroupID: workspaceGroupID,
	}

	updatedExpiresAt := time.Now().UTC().Add(time.Hour * 2).Format(time.RFC3339)

	regionsHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/v1/regions" || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(regions))
		require.NoError(t, err)

		return true
	}

	returnNotFound := true
	workspaceGroupsGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/workspaceGroups", workspaceGroupID.String()}, "/") ||
			r.Method != http.MethodGet {
			return false
		}

		if returnNotFound {
			w.Header().Add("Content-Type", "json")
			w.WriteHeader(http.StatusNotFound)

			returnNotFound = false

			return true
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspaceGroup))
		require.NoError(t, err)
		workspaceGroup.State = management.ACTIVE // Marking the state as ACTIVE to end polling.

		return true
	}

	workspaceGroupsPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaceGroups", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.WorkspaceGroupCreate
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, config.TestInitialAdminPassword, util.Deref(input.AdminPassword))
		require.False(t, util.Deref(input.AllowAllTraffic))
		require.Equal(t, config.TestInitialWorkspaceGroupExpiresAt, util.Deref(input.ExpiresAt))
		require.Equal(t, []string{config.TestInitialFirewallRange}, input.FirewallRanges)
		require.Equal(t, config.TestInitialWorkspaceGroupName, input.Name)
		require.Equal(t, regions[0].RegionID, input.RegionID)

		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: workspaceGroupID,
			},
		))
		require.NoError(t, err)
	}

	returnInternalError := true
	workspaceGroupsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupID.String()}, "/"), r.URL.Path)

		if returnInternalError {
			w.Header().Add("Content-Type", "json")
			w.WriteHeader(http.StatusInternalServerError)

			returnInternalError = false

			return
		}

		require.Equal(t, http.MethodPatch, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.WorkspaceGroupUpdate
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, updatedAdminPassword, util.Deref(input.AdminPassword))
		require.False(t, util.Deref(input.AllowAllTraffic))
		require.Equal(t, updatedExpiresAt, util.Deref(input.ExpiresAt))
		require.Empty(t, util.Deref(input.FirewallRanges))
		require.Equal(t, updatedWorkspaceGroupName, util.Deref(input.Name))

		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: workspaceGroupID,
			},
		))
		require.NoError(t, err)
		workspaceGroup.ExpiresAt = &updatedExpiresAt
		workspaceGroup.Name = updatedWorkspaceGroupName
		workspaceGroup.AllowAllTraffic = util.Ptr(true)
		workspaceGroup.FirewallRanges = util.Ptr([]string{}) // Updating for the next reads.
	}

	workspaceGroupsDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: workspaceGroupID,
			},
		))
		require.NoError(t, err)
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		regionsHandler,
		workspaceGroupsGetHandler,
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		workspaceGroupsPostHandler,
		workspaceGroupsPatchHandler, // Retry.
		workspaceGroupsPatchHandler,
		workspaceGroupsDeleteHandler,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}

		require.NotEmpty(t, writeHandlers, "already executed all the expected mutating REST calls")

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
				Config: examples.WorkspaceGroupsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", config.TestInitialWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "created_at", workspaceGroup.CreatedAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", *workspaceGroup.ExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "region_id", regions[0].RegionID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", config.TestInitialAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.0", config.TestInitialFirewallRange),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("name", cty.StringVal(updatedWorkspaceGroupName)).
					WithWorkspaceGroupResource("this")("admin_password", cty.StringVal(updatedAdminPassword)).
					WithWorkspaceGroupResource("this")("expires_at", cty.StringVal(updatedExpiresAt)).
					WithWorkspaceGroupResource("this")("firewall_ranges", cty.ListValEmpty(cty.String)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "created_at", workspaceGroup.CreatedAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", updatedExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "region_id", regions[0].RegionID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "0"),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestWorkspaceGroupResourceIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "this",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspaceGroupsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_workspace_group.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", config.TestInitialWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", config.TestInitialAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.0", config.TestInitialFirewallRange),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("name", cty.StringVal(updatedWorkspaceGroupName)).
					WithWorkspaceGroupResource("this")("admin_password", cty.StringVal(updatedAdminPassword)).
					WithWorkspaceGroupResource("this")("firewall_ranges", cty.ListValEmpty(cty.String)).
					String(), // Not testing updating expires at because of the limitations of testutil.IntegrationTest that ensures garbage collection.
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_workspace_group.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "0"),
				),
			},
		},
	})
}
