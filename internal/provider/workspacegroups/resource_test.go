package workspacegroups_test

import (
	"context"
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
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider"
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

const (
	pathV2Regions  = "/v2/regions"
	pathV1Projects = "/v1/projects"
)

func TestCRUDWorkspaceGroup(t *testing.T) { //nolint:maintidx,cyclop
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: "us-east-1",
		},
	}

	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	projectID := uuid.New()
	projectName := config.TestInitialProjectName

	workspaceGroup := management.WorkspaceGroup{
		CreatedAt:         time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:    util.Ptr([]string{config.TestInitialFirewallRange}),
		Name:              config.TestInitialWorkspaceGroupName,
		ProjectName:       &projectName,
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
		if r.URL.Path != pathV2Regions || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(regionsv2))
		require.NoError(t, err)

		return true
	}

	projectsHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != pathV1Projects || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Project{
			{
				Name:      projectName,
				ProjectID: projectID,
			},
		}))
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
		require.NotNil(t, input.ProjectID)
		require.Equal(t, projectID, *input.ProjectID)
		require.Equal(t, regionsv2[0].RegionName, *input.RegionName)
		require.Nil(t, input.UpdateWindow)

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
		require.NotNil(t, input.UpdateWindow)
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
		projectsHandler,
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", config.TestInitialWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "project_name", projectName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "created_at", workspaceGroup.CreatedAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", *workspaceGroup.ExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "cloud_provider", string(management.CloudProviderAWS)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "region_name", workspaceGroup.RegionName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", config.TestInitialAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.0", config.TestInitialFirewallRange),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(defaultDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "outbound_allow_list", testOutboundAllowList),
				),
			},
			{
				ResourceName:            "singlestoredb_workspace_group.this",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"admin_password"},
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("name", cty.StringVal(updatedWorkspaceGroupName)).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(projectName)).
					WithWorkspaceGroupResource("this")("admin_password", cty.StringVal(updatedAdminPassword)).
					WithWorkspaceGroupResource("this")("expires_at", cty.StringVal(updatedExpiresAt)).
					WithWorkspaceGroupResource("this")("firewall_ranges", cty.ListValEmpty(cty.String)).
					WithWorkspaceGroupResource("this")("deployment_type", cty.StringVal(string(updatedDeploymentType))).
					WithWorkspaceGroupResource("this")("cloud_provider", cty.StringVal(string(management.CloudProviderAWS))).
					WithWorkspaceGroupResource("this")("region_name", cty.StringVal(workspaceGroup.RegionName)).
					WithWorkspaceGroupResource("this")("update_window", cty.ObjectVal(map[string]cty.Value{
					"day":  cty.NumberIntVal(config.TestInitialUpdateWindowDay),
					"hour": cty.NumberIntVal(config.TestInitialUpdateWindowHour),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "project_name", projectName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "created_at", workspaceGroup.CreatedAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", updatedExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "cloud_provider", string(management.CloudProviderAWS)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "region_name", workspaceGroup.RegionName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "0"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(updatedDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
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
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "project_name", config.TestInitialProjectName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", config.TestInitialAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.0", config.TestInitialFirewallRange),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(defaultDeploymentType)),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("name", cty.StringVal(updatedWorkspaceGroupName)).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(config.TestInitialProjectName)).
					WithWorkspaceGroupResource("this")("admin_password", cty.StringVal(updatedAdminPassword)).
					WithWorkspaceGroupResource("this")("firewall_ranges", cty.ListValEmpty(cty.String)).
					WithWorkspaceGroupResource("this")("deployment_type", cty.StringVal(string(updatedDeploymentType))).
					WithWorkspaceGroupResource("this")(
					"update_window", cty.ObjectVal(map[string]cty.Value{
						"day":  cty.NumberIntVal(config.TestInitialUpdateWindowDay),
						"hour": cty.NumberIntVal(config.TestInitialUpdateWindowHour),
					})).
					String(), // Not testing updating expires at because of the limitations of testutil.IntegrationTest that ensures garbage collection.
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_workspace_group.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "project_name", config.TestInitialProjectName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "0"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(updatedDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
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
	projectID := uuid.New()
	projectName := config.TestInitialProjectName

	writeHandlers := []func(http.ResponseWriter, *http.Request){
		// CREATE workspace group
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/workspaceGroups", r.URL.Path)

			var input management.WorkspaceGroupCreate
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			require.NotNil(t, input.ProjectID)
			require.Equal(t, projectID, *input.ProjectID)

			// Verify update_window was specified in create
			require.NotNil(t, input.UpdateWindow)
			require.Equal(t, float32(config.TestInitialUpdateWindowDay), input.UpdateWindow.Day)
			require.Equal(t, float32(config.TestInitialUpdateWindowHour), input.UpdateWindow.Hour)

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(
				management.WorkspaceGroup{
					WorkspaceGroupID:  workspaceGroupID,
					Name:              config.TestInitialWorkspaceGroupName,
					ProjectName:       &projectName,
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
				ProjectName:       &projectName,
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
		require.Equal(t, pathV2Regions, r.URL.Path)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(regionsv2))
		require.NoError(t, err)
	}

	projectsHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, pathV1Projects, r.URL.Path)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Project{
			{
				Name:      projectName,
				ProjectID: projectID,
			},
		}))
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
		if r.URL.Path == pathV2Regions {
			regionsHandler(w, r)
			return //nolint:nlreturn
		}
		if r.URL.Path == pathV1Projects {
			projectsHandler(w, r)
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(projectName)).
					WithWorkspaceGroupResource("this")("update_window", cty.ObjectVal(map[string]cty.Value{
					"day":  cty.NumberIntVal(config.TestInitialUpdateWindowDay),
					"hour": cty.NumberIntVal(config.TestInitialUpdateWindowHour),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(projectName)).
					String(), // No update_window in config
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

func TestWorkspaceGroupProjectNameAssignmentAndImmutability(t *testing.T) {
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: "us-east-1",
		},
	}

	workspaceGroupID := uuid.New()
	regionID := uuid.New()
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	updatedProjectName := "updated-project"
	testOutboundAllowList := "test-account-id"

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != pathV2Regions || r.Method != http.MethodGet {
				return false
			}

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(regionsv2))
			require.NoError(t, err)

			return true
		},
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != pathV1Projects || r.Method != http.MethodGet {
				return false
			}

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON([]management.Project{
				{
					Name:      projectName,
					ProjectID: projectID,
				},
			}))
			require.NoError(t, err)

			return true
		},
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != fmt.Sprintf("/v1/workspaceGroups/%s", workspaceGroupID) || r.Method != http.MethodGet {
				return false
			}

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(
				management.WorkspaceGroup{
					WorkspaceGroupID:  workspaceGroupID,
					Name:              config.TestInitialWorkspaceGroupName,
					ProjectName:       &projectName,
					FirewallRanges:    &[]string{config.TestInitialFirewallRange},
					RegionID:          regionID,
					CreatedAt:         time.Now().UTC().Format(time.RFC3339),
					ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
					TerminatedAt:      nil,
					State:             management.WorkspaceGroupStateACTIVE,
					Provider:          management.CloudProviderAWS,
					RegionName:        regionsv2[0].RegionName,
					OutboundAllowList: &testOutboundAllowList,
					DeploymentType:    &defaultDeploymentType,
				},
			))
			require.NoError(t, err)

			return true
		},
	}

	writeHandlers := []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/v1/workspaceGroups", r.URL.Path)

			var input management.WorkspaceGroupCreate
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			require.NotNil(t, input.ProjectID)
			require.Equal(t, projectID, *input.ProjectID)
			require.Equal(t, config.TestInitialWorkspaceGroupName, input.Name)

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(
				struct {
					WorkspaceGroupID uuid.UUID
				}{
					WorkspaceGroupID: workspaceGroupID,
				},
			))
			require.NoError(t, err)
		},
		func(w http.ResponseWriter, r *http.Request) {
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
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(projectName)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "project_name", projectName),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(updatedProjectName)).
					String(),
				ExpectError: regexp.MustCompile("Cannot update workspace group project_name"),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestWorkspaceGroupProjectNameNotFound(t *testing.T) {
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: "us-east-1",
		},
	}

	projectName := "missing-project"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == pathV2Regions && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(regionsv2))
			require.NoError(t, err)
		case r.URL.Path == pathV1Projects && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON([]management.Project{
				{
					Name:      "zeta-project",
					ProjectID: uuid.New(),
				},
				{
					Name:      "alpha-project",
					ProjectID: uuid.New(),
				},
			}))
			require.NoError(t, err)
		default:
			require.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(projectName)).
					String(),
				ExpectError: regexp.MustCompile(`Available projects: 'alpha-project', 'zeta-project'\.`),
			},
		},
	})
}

func TestWorkspaceGroupProjectNameMultipleProjectsFound(t *testing.T) {
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: "us-east-1",
		},
	}

	projectName := "duplicate-project"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == pathV2Regions && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(regionsv2))
			require.NoError(t, err)
		case r.URL.Path == pathV1Projects && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON([]management.Project{
				{
					Name:      projectName,
					ProjectID: uuid.New(),
				},
				{
					Name:      projectName,
					ProjectID: uuid.New(),
				},
			}))
			require.NoError(t, err)
		default:
			require.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithWorkspaceGroupResource("this")("project_name", cty.StringVal(projectName)).
					String(),
				ExpectError: regexp.MustCompile("Multiple projects found"),
			},
		},
	})
}

// TestUpdateWithoutAdminPasswordDoesNotSendEmptyPassword reproduces the bug where a workspace
// group created without an explicit admin_password (server-generated) fails on a subsequent
// update because the PATCH request sends admin_password="", which the API rejects with
// "password must contain at least 14 characters".
func TestUpdateWithoutAdminPasswordDoesNotSendEmptyPassword(t *testing.T) {
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: "us-east-1",
		},
	}

	workspaceGroupID := uuid.New()
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	initialExpiresAt := config.TestInitialWorkspaceGroupExpiresAt
	updatedExpiresAt := time.Now().UTC().Add(time.Hour * 24).Format(time.RFC3339)

	currentExpiresAt := initialExpiresAt
	makeWorkspaceGroup := func() management.WorkspaceGroup {
		return management.WorkspaceGroup{
			ExpiresAt:         &currentExpiresAt,
			FirewallRanges:    util.Ptr([]string{config.TestInitialFirewallRange}),
			Name:              config.TestInitialWorkspaceGroupName,
			ProjectName:       &projectName,
			RegionName:        regionsv2[0].RegionName,
			Provider:          management.CloudProviderAWS,
			State:             management.WorkspaceGroupStateACTIVE,
			UpdateWindow:      &management.UpdateWindow{Day: config.TestInitialUpdateWindowDay, Hour: config.TestInitialUpdateWindowHour},
			WorkspaceGroupID:  workspaceGroupID,
			DeploymentType:    &defaultDeploymentType,
			OutboundAllowList: &testOutboundAllowList,
		}
	}

	server := newWorkspaceGroupUpdateWithoutAdminPasswordTestServer(t,
		regionsv2,
		projectName,
		projectID,
		workspaceGroupID,
		makeWorkspaceGroup,
		updatedExpiresAt,
		&currentExpiresAt,
	)
	t.Cleanup(server.Close)

	// Build a config without admin_password (server generates one)
	makeConfig := func(expiresAt string) string {
		return fmt.Sprintf(`
provider "singlestoredb" {
}
resource "singlestoredb_workspace_group" "this" {
  name            = %[1]q
  project_name    = %[2]q
  firewall_ranges = [%[3]q]
  expires_at      = %[4]q
  cloud_provider  = "AWS"
  region_name     = %[5]q
}
`, config.TestInitialWorkspaceGroupName, projectName, config.TestInitialFirewallRange, expiresAt, regionsv2[0].RegionName)
	}

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: makeConfig(initialExpiresAt),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", initialExpiresAt),
				),
			},
			{
				// Triggering an update by changing expires_at must not result in an empty
				// admin_password being sent to the API.
				Config: makeConfig(updatedExpiresAt),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", updatedExpiresAt),
				),
			},
		},
	})
}

func newWorkspaceGroupUpdateWithoutAdminPasswordTestServer(
	t *testing.T,
	regionsv2 []management.RegionV2,
	projectName string,
	projectID uuid.UUID,
	workspaceGroupID uuid.UUID,
	makeWorkspaceGroup func() management.WorkspaceGroup,
	updatedExpiresAt string,
	currentExpiresAt *string,
) *httptest.Server {
	t.Helper()

	workspaceGroupPath := fmt.Sprintf("/v1/workspaceGroups/%s", workspaceGroupID)
	readOnlyHandlers := []func(http.ResponseWriter, *http.Request) bool{
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != pathV2Regions || r.Method != http.MethodGet {
				return false
			}

			writeJSONResponse(t, w, regionsv2)

			return true
		},
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != pathV1Projects || r.Method != http.MethodGet {
				return false
			}

			writeJSONResponse(t, w, []management.Project{{Name: projectName, ProjectID: projectID}})

			return true
		},
		func(w http.ResponseWriter, r *http.Request) bool {
			if r.URL.Path != workspaceGroupPath || r.Method != http.MethodGet {
				return false
			}

			writeJSONResponse(t, w, makeWorkspaceGroup())

			return true
		},
	}

	writeHandlers := []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/workspaceGroups", r.URL.Path)
			require.Equal(t, http.MethodPost, r.Method)

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var input management.WorkspaceGroupCreate
			require.NoError(t, json.Unmarshal(body, &input))
			require.Nil(t, input.AdminPassword, "Create must not send an empty admin_password when omitted from config")

			writeJSONResponse(t, w, struct {
				WorkspaceGroupID uuid.UUID
			}{WorkspaceGroupID: workspaceGroupID})
		},
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, workspaceGroupPath, r.URL.Path)
			require.Equal(t, http.MethodPatch, r.Method)

			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var input management.WorkspaceGroupUpdate
			require.NoError(t, json.Unmarshal(body, &input))
			if input.AdminPassword != nil {
				w.Header().Add("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`password must contain at least 14 characters`))

				return
			}

			*currentExpiresAt = updatedExpiresAt
			writeJSONResponse(t, w, struct {
				WorkspaceGroupID uuid.UUID
			}{WorkspaceGroupID: workspaceGroupID})
		},
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, workspaceGroupPath, r.URL.Path)
			require.Equal(t, http.MethodDelete, r.Method)

			writeJSONResponse(t, w, struct {
				WorkspaceGroupID uuid.UUID
			}{WorkspaceGroupID: workspaceGroupID})
		},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}

		require.NotEmpty(t, writeHandlers, "unexpected request: %s %s", r.Method, r.URL.Path)

		h := writeHandlers[0]
		h(w, r)
		writeHandlers = writeHandlers[1:]
	}))
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, body interface{}) {
	t.Helper()

	w.Header().Add("Content-Type", "json")
	_, err := w.Write(testutil.MustJSON(body))
	require.NoError(t, err)
}

// TestWorkspaceGroupUpdateWithoutAdminPasswordIntegration is the integration variant of
// TestUpdateWithoutAdminPasswordDoesNotSendEmptyPassword: it creates a workspace group without
// admin_password, then changes expires_at to force an update, asserting that the real API does
// not reject the PATCH because of an empty admin_password.
func TestWorkspaceGroupUpdateWithoutAdminPasswordIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test because go test is run with the flag -short")
	}

	apiKey := os.Getenv(config.EnvTestAPIKey)
	require.NotEmpty(t, apiKey, "envirnomental variable %s should be set for running integration tests", config.EnvTestAPIKey)

	uniqueName := testutil.GenerateUniqueResourceName("tf-no-admin-pw")
	initialExpiresAt := time.Now().UTC().Add(config.TestWorkspaceGroupExpiration).Format(time.RFC3339)
	updatedExpiresAt := time.Now().UTC().Add(config.TestWorkspaceGroupExpiration + time.Hour).Format(time.RFC3339)

	makeConfig := func(expiresAt string) string {
		return fmt.Sprintf(`
provider "singlestoredb" {
}
resource "singlestoredb_workspace_group" "this" {
  name            = %[1]q
  project_name    = %[2]q
  firewall_ranges = [%[3]q]
  expires_at      = %[4]q
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
}
`, uniqueName, config.TestInitialProjectName, config.TestInitialFirewallRange, expiresAt)
	}

	// 1. Create the workspace group directly via the management API, outside Terraform.
	// Importantly, we do NOT specify admin_password — the server generates one — to mirror
	// the user-reported scenario where the password is unknown to the Terraform config.
	apiClient := newWorkspaceGroupAPIClient(t, apiKey)
	createResp, err := apiClient.PostV1WorkspaceGroupsWithResponse(t.Context(), management.PostV1WorkspaceGroupsJSONRequestBody{
		Name:           uniqueName,
		FirewallRanges: []string{config.TestInitialFirewallRange},
		ExpiresAt:      &initialExpiresAt,
		Provider:       util.Ptr(management.CloudProviderAWS),
		RegionName:     util.Ptr("us-east-1"),
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, createResp.StatusCode(), "create workspace group failed: %s", string(createResp.Body))
	workspaceGroupID := createResp.JSON200.WorkspaceGroupID

	// Ensure cleanup runs even if Terraform doesn't (e.g., the test fails before destroy).
	// Use a fresh context so cleanup is not canceled with the test context.
	t.Cleanup(func() {
		_, _ = apiClient.DeleteV1WorkspaceGroupsWorkspaceGroupIDWithResponse(context.Background(), workspaceGroupID, //nolint:usetesting
			&management.DeleteV1WorkspaceGroupsWorkspaceGroupIDParams{Force: util.Ptr(true)})
	})

	// Wait until the API reports the workspace group as ACTIVE before letting Terraform import it.
	waitWorkspaceGroupActive(t, apiClient, workspaceGroupID)

	t.Setenv("TF_ACC", "on")
	t.Setenv(config.EnvAPIKey, apiKey)

	f := provider.New("dev")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			config.ProviderName: providerserver.NewProtocol6WithError(f()),
		},
		Steps: []resource.TestStep{
			{
				// 2. Import the API-created workspace group into Terraform state.
				// ImportStateVerify is omitted because there's no prior TF apply to compare
				// against (the resource was created via the management API, not by Terraform).
				Config:             makeConfig(initialExpiresAt),
				ResourceName:       "singlestoredb_workspace_group.this",
				ImportState:        true,
				ImportStateId:      workspaceGroupID.String(),
				ImportStatePersist: true,
			},
			{
				// 3. Update the imported workspace group. Triggers a PATCH which, prior to the
				// fix, sent admin_password="" and was rejected by the API with
				// "password must contain at least 14 characters".
				Config: makeConfig(updatedExpiresAt),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", updatedExpiresAt),
				),
			},
		},
	})
}

func newWorkspaceGroupAPIClient(t *testing.T, apiKey string) *management.ClientWithResponses {
	t.Helper()

	c, err := management.NewClientWithResponses(config.APIServiceURL,
		management.WithHTTPClient(util.NewHTTPClient()),
		management.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

			return nil
		}),
	)
	require.NoError(t, err)

	return c
}

func waitWorkspaceGroupActive(t *testing.T, c *management.ClientWithResponses, id management.WorkspaceGroupID) {
	t.Helper()

	deadline := time.Now().Add(config.WorkspaceGroupCreationTimeout)
	for time.Now().Before(deadline) {
		resp, err := c.GetV1WorkspaceGroupsWorkspaceGroupIDWithResponse(t.Context(), id, &management.GetV1WorkspaceGroupsWorkspaceGroupIDParams{})
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode(), "get workspace group failed: %s", string(resp.Body))
		if resp.JSON200.State == management.WorkspaceGroupStateACTIVE {
			return
		}
		time.Sleep(10 * time.Second)
	}
	t.Fatalf("workspace group %s did not reach ACTIVE state within timeout", id)
}
