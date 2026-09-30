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
	defaultDeploymentType     = management.PRODUCTION
	updatedDeploymentType     = management.NONPRODUCTION
	updatedFirewallRanges     = []string{"198.51.100.0/24", "192.0.2.0/24"}
)

const (
	pathV2Regions  = "/v2/regions"
	pathV2Projects = "/v2/projects"
	pathV2Clusters = "/v2/clusters"
)

func TestCRUDWorkspaceGroup(t *testing.T) { //nolint:maintidx,cyclop
	regionsv2 := []management.RegionV2{{
		Provider:   management.CloudProviderAWS,
		Region:     "US East 1 (N. Virginia)",
		RegionName: "us-east-1",
	}}

	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	clusterID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	createdAt := time.Now().UTC()

	workspaceGroup := management.Cluster{
		CreatedAt:         util.Ptr(createdAt),
		ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:    util.Ptr([]string{config.TestInitialFirewallRange}),
		Name:              config.TestInitialWorkspaceGroupName,
		Region:            util.Ptr(regionsv2[0].RegionName),
		Provider:          util.Ptr(management.CloudProviderAWS),
		State:             util.Ptr(management.ClusterStatePENDING),
		TerminatedAt:      nil,
		UpdateWindow:      &management.UpdateWindow{Day: config.TestInitialUpdateWindowDay, Hour: config.TestInitialUpdateWindowHour},
		GroupID:           util.Ptr(workspaceGroupID),
		ClusterID:         util.Ptr(clusterID),
		ProjectID:         projectID,
		DeploymentType:    &defaultDeploymentType,
		OutboundAllowList: &testOutboundAllowList,
		SizeConfig:        &management.SizeConfig{Size: util.Ptr("S-00")},
	}

	updatedExpiresAt := time.Now().UTC().Add(time.Hour * 2).Format(time.RFC3339)
	patchAttempts := 0
	deleteCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		clusterPath := strings.Join([]string{pathV2Clusters, clusterID.String()}, "/")
		switch {
		case r.URL.Path == pathV2Regions && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(regionsv2))
			require.NoError(t, err)
			return
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{
				Name: projectName, ProjectID: projectID, Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}}))
			require.NoError(t, err)
			return
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodGet:
			if workspaceGroup.State != nil && *workspaceGroup.State == management.ClusterStatePENDING {
				workspaceGroup.State = util.Ptr(management.ClusterStateACTIVE)
			}
			_, err := w.Write(testutil.MustJSON([]management.Cluster{workspaceGroup}))
			require.NoError(t, err)
			return
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodPost:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, config.TestInitialAdminPassword, util.Deref(input.AdminPassword))
			require.Equal(t, config.TestInitialWorkspaceGroupExpiresAt, util.Deref(input.ExpiresAt))
			require.Equal(t, []string{config.TestInitialFirewallRange}, util.Deref(input.FirewallRanges))
			require.Equal(t, config.TestInitialWorkspaceGroupName, input.Name)
			require.Equal(t, projectID, input.ProjectID)
			require.Equal(t, regionsv2[0].RegionName, util.Deref(input.Region))
			_, err = w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
				GroupID   uuid.UUID `json:"groupID"`
			}{ClusterID: clusterID, GroupID: workspaceGroupID}))
			require.NoError(t, err)
			return
		case r.URL.Path == clusterPath && r.Method == http.MethodPatch:
			patchAttempts++
			if patchAttempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, updatedAdminPassword, util.Deref(input.AdminPassword))
			require.Equal(t, updatedExpiresAt, util.Deref(input.ExpiresAt))
			require.Empty(t, util.Deref(input.FirewallRanges))
			require.Equal(t, updatedWorkspaceGroupName, input.Name)
			require.Equal(t, string(updatedDeploymentType), string(*input.DeploymentType))
			require.NotNil(t, input.UpdateWindow)
			_, err = w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: clusterID}))
			require.NoError(t, err)
			workspaceGroup.ExpiresAt = &updatedExpiresAt
			workspaceGroup.Name = updatedWorkspaceGroupName
			workspaceGroup.AllowAllTraffic = util.Ptr(false)
			workspaceGroup.FirewallRanges = util.Ptr([]string{})
			workspaceGroup.DeploymentType = &updatedDeploymentType
			return
		case r.URL.Path == clusterPath && r.Method == http.MethodDelete:
			deleteCalled = true
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: clusterID}))
			require.NoError(t, err)
			return
		}
		t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
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
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "project_name", projectName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "created_at", createdAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", config.TestInitialWorkspaceGroupExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "cloud_provider", string(management.CloudProviderAWS)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "region_name", regionsv2[0].RegionName),
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
					WithWorkspaceGroupResource("this")("region_name", cty.StringVal(regionsv2[0].RegionName)).
					WithWorkspaceGroupResource("this")("update_window", cty.ObjectVal(map[string]cty.Value{
					"day":  cty.NumberIntVal(config.TestInitialUpdateWindowDay),
					"hour": cty.NumberIntVal(config.TestInitialUpdateWindowHour),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", config.IDAttribute, workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "project_name", projectName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", updatedExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "0"),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "deployment_type", string(updatedDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.day", fmt.Sprint(config.TestInitialUpdateWindowDay)),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "update_window.hour", fmt.Sprint(config.TestInitialUpdateWindowHour)),
				),
			},
		},
	})

	require.GreaterOrEqual(t, patchAttempts, 2)
	require.True(t, deleteCalled)
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
					WithWorkspaceGroupResource("this")("firewall_ranges", cty.ListVal([]cty.Value{
					cty.StringVal(updatedFirewallRanges[0]),
					cty.StringVal(updatedFirewallRanges[1]),
				})).
					WithWorkspaceGroupResource("this")("deployment_type", cty.StringVal(string(updatedDeploymentType))).
					WithWorkspaceGroupResource("this")("update_window", cty.ObjectVal(map[string]cty.Value{
					"day":  cty.NumberIntVal(config.TestInitialUpdateWindowDay),
					"hour": cty.NumberIntVal(config.TestInitialUpdateWindowHour),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_workspace_group.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "firewall_ranges.#", "2"),
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
	project_name    = "Standard Project"
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

func TestWorkspaceGroupProjectNameAssignmentAndImmutability(t *testing.T) {
	workspaceGroupID := uuid.New()
	clusterID := uuid.New()
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	updatedProjectName := "updated-project"
	createdAt := time.Now().UTC()

	workspaceGroup := management.Cluster{
		GroupID:           util.Ptr(workspaceGroupID),
		ClusterID:         util.Ptr(clusterID),
		ProjectID:         projectID,
		Name:              config.TestInitialWorkspaceGroupName,
		FirewallRanges:    util.Ptr([]string{config.TestInitialFirewallRange}),
		CreatedAt:         util.Ptr(createdAt),
		ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		State:             util.Ptr(management.ClusterStateACTIVE),
		Provider:          util.Ptr(management.CloudProviderAWS),
		Region:            util.Ptr("us-east-1"),
		OutboundAllowList: &testOutboundAllowList,
		DeploymentType:    &defaultDeploymentType,
		SizeConfig:        &management.SizeConfig{Size: util.Ptr("S-00")},
	}

	writeQueue := []func(http.ResponseWriter, *http.Request){
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, pathV2Clusters, r.URL.Path)
			var input management.Cluster
			require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
			require.Equal(t, projectID, input.ProjectID)
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
				GroupID   uuid.UUID `json:"groupID"`
			}{ClusterID: clusterID, GroupID: workspaceGroupID}))
			require.NoError(t, err)
		},
		func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, strings.Join([]string{pathV2Clusters, clusterID.String()}, "/"), r.URL.Path)
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: clusterID}))
			require.NoError(t, err)
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch {
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{
				Name: projectName, ProjectID: projectID, Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}}))
			require.NoError(t, err)
			return
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Cluster{workspaceGroup}))
			require.NoError(t, err)
			return
		}
		require.NotEmpty(t, writeQueue, "unexpected %s %s", r.Method, r.URL.Path)
		h := writeQueue[0]
		writeQueue = writeQueue[1:]
		h(w, r)
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

	require.Empty(t, writeQueue)
}

func TestWorkspaceGroupProjectNameNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch {
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{
				Name: "Other Project", ProjectID: uuid.New(), Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.WorkspaceGroupsResource,
				ExpectError: regexp.MustCompile("Project not found"),
			},
		},
	})
}

func TestWorkspaceGroupProjectNameMultipleProjectsFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch {
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{
				{Name: config.TestInitialProjectName, ProjectID: uuid.New(), Edition: management.STANDARD, CreatedAt: time.Now().UTC()},
				{Name: config.TestInitialProjectName, ProjectID: uuid.New(), Edition: management.STANDARD, CreatedAt: time.Now().UTC()},
			}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.WorkspaceGroupsResource,
				ExpectError: regexp.MustCompile("Multiple projects found"),
			},
		},
	})
}

func TestUpdateWithoutAdminPasswordDoesNotSendEmptyPassword(t *testing.T) {
	workspaceGroupID := uuid.New()
	clusterID := uuid.New()
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	initialExpiresAt := config.TestInitialWorkspaceGroupExpiresAt
	updatedExpiresAt := time.Now().UTC().Add(time.Hour * 24).Format(time.RFC3339)
	generatedPassword := "serverGeneratedPass1!"
	createdAt := time.Now().UTC()

	workspaceGroup := management.Cluster{
		ExpiresAt:         util.Ptr(initialExpiresAt),
		FirewallRanges:    util.Ptr([]string{config.TestInitialFirewallRange}),
		Name:              config.TestInitialWorkspaceGroupName,
		Region:            util.Ptr("us-east-1"),
		Provider:          util.Ptr(management.CloudProviderAWS),
		State:             util.Ptr(management.ClusterStateACTIVE),
		UpdateWindow:      &management.UpdateWindow{Day: config.TestInitialUpdateWindowDay, Hour: config.TestInitialUpdateWindowHour},
		GroupID:           util.Ptr(workspaceGroupID),
		ClusterID:         util.Ptr(clusterID),
		ProjectID:         projectID,
		CreatedAt:         util.Ptr(createdAt),
		DeploymentType:    &defaultDeploymentType,
		OutboundAllowList: &testOutboundAllowList,
		SizeConfig:        &management.SizeConfig{Size: util.Ptr("S-00")},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch {
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{
				Name: projectName, ProjectID: projectID, Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}}))
			require.NoError(t, err)
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Cluster{workspaceGroup}))
			require.NoError(t, err)
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodPost:
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID     uuid.UUID `json:"clusterID"`
				GroupID       uuid.UUID `json:"groupID"`
				AdminPassword string    `json:"adminPassword"`
			}{ClusterID: clusterID, GroupID: workspaceGroupID, AdminPassword: generatedPassword}))
			require.NoError(t, err)
		case r.URL.Path == strings.Join([]string{pathV2Clusters, clusterID.String()}, "/") && r.Method == http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			require.Nil(t, input.AdminPassword, "empty admin password must not be sent on update")
			require.Equal(t, updatedExpiresAt, util.Deref(input.ExpiresAt))
			workspaceGroup.ExpiresAt = &updatedExpiresAt
			_, err = w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: clusterID}))
			require.NoError(t, err)
		case r.URL.Path == strings.Join([]string{pathV2Clusters, clusterID.String()}, "/") && r.Method == http.MethodDelete:
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: clusterID}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

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
`, config.TestInitialWorkspaceGroupName, projectName, config.TestInitialFirewallRange, expiresAt)
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
				Config: makeConfig(updatedExpiresAt),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace_group.this", "expires_at", updatedExpiresAt),
				),
			},
		},
	})
}

func TestWorkspaceGroupUpdateWithoutAdminPasswordIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "this",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider "singlestoredb" {
}
resource "singlestoredb_workspace_group" "this" {
  name            = %q
  project_name    = %q
  firewall_ranges = [%q]
  expires_at      = %q
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
}
`, config.TestInitialWorkspaceGroupName, config.TestInitialProjectName, config.TestInitialFirewallRange, config.TestInitialWorkspaceGroupExpiresAt),
				Check: resource.TestCheckResourceAttrSet("singlestoredb_workspace_group.this", "admin_password"),
			},
		},
	})
}
