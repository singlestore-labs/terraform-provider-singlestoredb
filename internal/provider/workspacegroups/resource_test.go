package workspacegroups_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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
	updatedAdminPassword      = "mockPasswordUpdated193!"
	defaultDeploymentType     = management.WorkspaceGroupDeploymentTypePRODUCTION
	updatedDeploymentType     = management.WorkspaceGroupDeploymentTypeNONPRODUCTION
)

func TestCRUDWorkspaceGroup(t *testing.T) {
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: "us-east-1",
		},
	}

	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")

	workspaceGroup := management.WorkspaceGroup{
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:    util.Ptr([]string{config.TestInitialFirewallRange}),
		Name:              config.TestInitialWorkspaceGroupName,
		RegionName:        regionsv2[0].RegionName,
		Provider:          management.CloudProviderAWS,
		State:             management.WorkspaceGroupStatePENDING, // During the first poll, the status will still be PENDING.
		TerminatedAt:      nil,
		UpdateWindow:      &management.UpdateWindow{Day: config.TestInitialUpdateWindowDay, Hour: config.TestInitialUpdateWindowHour},
		WorkspaceGroupID:  workspaceGroupID,
		DeploymentType:    &defaultDeploymentType,
		OutboundAllowList: &testOutboundAllowList,
	}

	updatedExpiresAt := time.Now().UTC().Add(time.Hour * 2).Format(time.RFC3339)

	regionsv2Handler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/v2/regions" || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(regionsv2))
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
		workspaceGroup.State = management.WorkspaceGroupStateACTIVE // Marking the state as ACTIVE to end polling.

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
		require.Equal(t, regionsv2[0].RegionName, *input.RegionName)
		require.Equal(t, config.TestInitialUpdateWindowDay, int(input.UpdateWindow.Day))
		require.Equal(t, config.TestInitialUpdateWindowHour, int(input.UpdateWindow.Hour))

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
		require.Equal(t, string(updatedDeploymentType), string(*input.DeploymentType))

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
		workspaceGroup.DeploymentType = &updatedDeploymentType
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
		regionsv2Handler,
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
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "cloud_provider", string(management.CloudProviderAWS)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "region_name", workspaceGroup.RegionName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", config.TestInitialAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.0", config.TestInitialFirewallRange),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(defaultDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "outbound_allow_list", testOutboundAllowList),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("name", cty.StringVal(updatedWorkspaceGroupName)).
					WithWorkspaceGroupResource("this")("admin_password", cty.StringVal(updatedAdminPassword)).
					WithWorkspaceGroupResource("this")("expires_at", cty.StringVal(updatedExpiresAt)).
					WithWorkspaceGroupResource("this")("firewall_ranges", cty.ListValEmpty(cty.String)).
					WithWorkspaceGroupResource("this")("deployment_type", cty.StringVal(string(updatedDeploymentType))).
					WithWorkspaceGroupResource("this")("cloud_provider", cty.StringVal(string(management.CloudProviderAWS))).
					WithWorkspaceGroupResource("this")("region_name", cty.StringVal(workspaceGroup.RegionName)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "created_at", workspaceGroup.CreatedAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", updatedExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "cloud_provider", string(management.CloudProviderAWS)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "region_name", workspaceGroup.RegionName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "0"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(updatedDeploymentType)),
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
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(defaultDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("name", cty.StringVal(updatedWorkspaceGroupName)).
					WithWorkspaceGroupResource("this")("admin_password", cty.StringVal(updatedAdminPassword)).
					WithWorkspaceGroupResource("this")("firewall_ranges", cty.ListValEmpty(cty.String)).
					WithWorkspaceGroupResource("this")("deployment_type", cty.StringVal(string(updatedDeploymentType))).
					String(), // Not testing updating expires at because of the limitations of testutil.IntegrationTest that ensures garbage collection.
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_workspace_group.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "0"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(updatedDeploymentType)),
				),
			},
		},
	})
}

func TestUpdateWindowValidation(t *testing.T) {
	testCases := []struct {
		day         int
		hour        int
		expectError string
	}{
		{7, 12, `update_window\.day`},
		{-1, 12, `update_window\.day`},
		{3, 24, `update_window\.hour`},
		{3, -1, `update_window\.hour`},
	}

	for _, tc := range testCases {
		testutil.UnitTest(t, testutil.UnitTestConfig{
			APIKey:        testutil.UnusedAPIKey,
			APIServiceURL: "http://unused",
		}, resource.TestCase{
			Steps: []resource.TestStep{
				{
					Config: fmt.Sprintf(`
provider "singlestoredb" {
}
resource "singlestoredb_workspace_group" "test" {
	name            = %[1]q
	cloud_provider  = "AWS"
	region_name     = "us-east-1"
	firewall_ranges = [%[2]q]
	update_window   = { day = %[3]d, hour = %[4]d }
}`, config.TestInitialWorkspaceGroupName, config.TestInitialFirewallRange, tc.day, tc.hour),
					ExpectError: regexp.MustCompile(tc.expectError),
				},
			},
		})
	}
}

func TestUpdateWindowRemoval(t *testing.T) {
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: "us-east-1",
		},
	}

	workspaceGroupID := uuid.New()
	regionID := uuid.New()
	testOutboundAllowList := "test-account-id"

	writeHandlers := []func(http.ResponseWriter, *http.Request){
		// CREATE workspace group
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/workspaceGroups", r.URL.Path)

			var input management.WorkspaceGroupCreate
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))

			// Verify update_window was specified in create
			require.NotNil(t, input.UpdateWindow)
			require.Equal(t, float32(config.TestInitialUpdateWindowDay), input.UpdateWindow.Day)
			require.Equal(t, float32(config.TestInitialUpdateWindowHour), input.UpdateWindow.Hour)

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(
				management.WorkspaceGroup{
					WorkspaceGroupID:  workspaceGroupID,
					Name:              config.TestInitialWorkspaceGroupName,
					FirewallRanges:    &[]string{config.TestInitialFirewallRange},
					RegionID:          regionID,
					CreatedAt:         time.Now().UTC().Format(time.RFC3339),
					ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
					TerminatedAt:      nil,
					State:             management.WorkspaceGroupStateACTIVE,
					Provider:          management.CloudProviderAWS,
					RegionName:        regionsv2[0].RegionName,
					UpdateWindow:      &management.UpdateWindow{Day: float32(config.TestInitialUpdateWindowDay), Hour: float32(config.TestInitialUpdateWindowHour)},
					OutboundAllowList: &testOutboundAllowList,
					DeploymentType:    &defaultDeploymentType,
				},
			))
			require.NoError(t, err)
		},
		// UPDATE is NOT called when update_window is just removed from config
		// because it's Optional+Computed, Terraform keeps the existing value
	}

	readHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, fmt.Sprintf("/v1/workspaceGroups/%s", workspaceGroupID), r.URL.Path)

		// Value persists at the initially set value
		uw := &management.UpdateWindow{Day: float32(config.TestInitialUpdateWindowDay), Hour: float32(config.TestInitialUpdateWindowHour)}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			management.WorkspaceGroup{
				WorkspaceGroupID:  workspaceGroupID,
				Name:              config.TestInitialWorkspaceGroupName,
				FirewallRanges:    &[]string{config.TestInitialFirewallRange},
				RegionID:          regionID,
				CreatedAt:         time.Now().UTC().Format(time.RFC3339),
				ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
				TerminatedAt:      nil,
				State:             management.WorkspaceGroupStateACTIVE,
				Provider:          management.CloudProviderAWS,
				RegionName:        regionsv2[0].RegionName,
				UpdateWindow:      uw,
				OutboundAllowList: &testOutboundAllowList,
				DeploymentType:    &defaultDeploymentType,
			},
		))
		require.NoError(t, err)
	}

	regionsHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v2/regions", r.URL.Path)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(regionsv2))
		require.NoError(t, err)
	}

	deleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodDelete, r.Method)
		require.Equal(t, fmt.Sprintf("/v1/workspaceGroups/%s", workspaceGroupID), r.URL.Path)

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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/regions" {
			regionsHandler(w, r)
			return //nolint:nlreturn
		}

		if r.Method == http.MethodGet {
			readHandler(w, r)
			return //nolint:nlreturn
		}

		if r.Method == http.MethodDelete {
			deleteHandler(w, r)
			return //nolint:nlreturn
		}

		require.NotEmpty(t, writeHandlers, "unexpected write request: %s %s", r.Method, r.URL.Path)
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
				// Create with update_window specified
				Config: examples.WorkspaceGroupsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
				),
			},
			{
				// Remove update_window from config
				Config: fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_workspace_group" "this" {
	name            = %[1]q
	cloud_provider  = "AWS"
	region_name     = "us-east-1"
	admin_password  = %[2]q
	firewall_ranges = [%[3]q]
	expires_at      = %[4]q
	# update_window intentionally removed
}`, config.TestInitialWorkspaceGroupName, config.TestInitialAdminPassword, config.TestInitialFirewallRange, config.TestInitialWorkspaceGroupExpiresAt),
				Check: resource.ComposeAggregateTestCheckFunc(
					// When update_window is removed from config, the value persists from previous state
					// No update is triggered because the field is Optional+Computed
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}
