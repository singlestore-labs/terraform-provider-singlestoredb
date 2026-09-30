package clusters_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	updatedClusterSize    = "S-0"
	updatedAdminPassword  = "mockPasswordUpdated193!"
	defaultDeploymentType = management.PRODUCTION
	updatedDeploymentType = management.NONPRODUCTION
	updatedFirewallRanges = []string{"198.51.100.0/24", "192.0.2.0/24"}
	testOutboundAllowList = "123456789012"
)

const (
	pathV2Regions  = "/v2/regions"
	pathV2Projects = "/v2/projects"
	pathV2Clusters = "/v2/clusters"
	testRegion     = "us-east-1"
	testEndpoint   = "svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"
)

func TestCRUDCluster(t *testing.T) { //nolint:maintidx,cyclop
	regionsv2 := []management.RegionV2{
		{
			Provider:   management.CloudProviderAWS,
			RegionName: testRegion,
		},
	}

	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	groupID := uuid.MustParse("4db4e46a-132e-56fe-97dc-49c9e25bd618")
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	createdAt := time.Now().UTC()
	stateActive := management.ClusterStateACTIVE
	statePending := management.ClusterStatePENDING
	endpoint := testEndpoint
	provider := management.CloudProviderAWS
	region := testRegion
	size := config.TestInitialWorkspaceSize
	scaleFactor := float32(1)
	cacheConfig := float32(1)

	cluster := management.Cluster{
		AdminPassword:     util.Ptr(config.TestInitialAdminPassword),
		ClusterID:         &clusterID,
		CreatedAt:         &createdAt,
		DeploymentType:    &defaultDeploymentType,
		Endpoint:          &endpoint,
		ExpiresAt:         util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:    util.Ptr([]string{config.TestInitialFirewallRange}),
		GroupID:           &groupID,
		Name:              "cluster-1",
		OutboundAllowList: &testOutboundAllowList,
		ProjectID:         projectID,
		Provider:          &provider,
		Region:            &region,
		SizeConfig: &management.SizeConfig{
			Size:        &size,
			ScaleFactor: &scaleFactor,
			CacheConfig: &cacheConfig,
		},
		State: &statePending,
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
		if r.URL.Path != pathV2Projects || r.Method != http.MethodGet {
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
	clustersGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{pathV2Clusters, clusterID.String()}, "/") ||
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
		_, err := w.Write(testutil.MustJSON(cluster))
		require.NoError(t, err)
		cluster.State = &stateActive

		return true
	}

	clustersPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, pathV2Clusters, r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.Cluster
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, config.TestInitialAdminPassword, util.Deref(input.AdminPassword))
		require.Equal(t, config.TestInitialWorkspaceGroupExpiresAt, util.Deref(input.ExpiresAt))
		require.Equal(t, []string{config.TestInitialFirewallRange}, util.Deref(input.FirewallRanges))
		require.Equal(t, "cluster-1", input.Name)
		require.Equal(t, projectID, input.ProjectID)
		require.Equal(t, region, util.Deref(input.Region))
		require.NotNil(t, input.SizeConfig)
		require.Equal(t, size, util.Deref(input.SizeConfig.Size))

		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				AdminPassword *string   `json:"adminPassword"`
				ClusterID     uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				GroupID       uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
			}{
				AdminPassword: util.Ptr(config.TestInitialAdminPassword),
				ClusterID:     clusterID,
				GroupID:       groupID,
			},
		))
		require.NoError(t, err)
	}

	returnInternalError := true
	clustersPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{pathV2Clusters, clusterID.String()}, "/"), r.URL.Path)

		if returnInternalError {
			w.Header().Add("Content-Type", "json")
			w.WriteHeader(http.StatusInternalServerError)

			returnInternalError = false

			return
		}

		require.Equal(t, http.MethodPatch, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.Cluster
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, updatedAdminPassword, util.Deref(input.AdminPassword))
		require.Equal(t, updatedExpiresAt, util.Deref(input.ExpiresAt))
		require.Equal(t, updatedFirewallRanges, util.Deref(input.FirewallRanges))
		require.Equal(t, string(updatedDeploymentType), string(util.Deref(input.DeploymentType)))
		require.NotNil(t, input.SizeConfig)
		require.Equal(t, updatedClusterSize, util.Deref(input.SizeConfig.Size))

		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{
				ClusterID: clusterID,
			},
		))
		require.NoError(t, err)
		cluster.ExpiresAt = &updatedExpiresAt
		cluster.FirewallRanges = &updatedFirewallRanges
		cluster.DeploymentType = &updatedDeploymentType
		cluster.SizeConfig.Size = &updatedClusterSize
		cluster.AdminPassword = &updatedAdminPassword
	}

	clustersDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{pathV2Clusters, clusterID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{
				ClusterID: clusterID,
			},
		))
		require.NoError(t, err)
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		regionsv2Handler,
		projectsHandler,
		clustersGetHandler,
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		clustersPostHandler,
		clustersPatchHandler, // Retry.
		clustersPatchHandler,
		clustersDeleteHandler,
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
				Config: examples.ClustersResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", config.IDAttribute, clusterID.String()),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "name", "cluster-1"),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "project_name", projectName),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "group_id", groupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "size", size),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "created_at", createdAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "expires_at", *cluster.ExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "cloud_provider", string(management.CloudProviderAWS)),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "region_name", region),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "admin_password", config.TestInitialAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "firewall_ranges.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "firewall_ranges.0", config.TestInitialFirewallRange),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "deployment_type", string(defaultDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "outbound_allow_list", testOutboundAllowList),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "endpoint", endpoint),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "suspended", "false"),
				),
			},
			{
				ResourceName:            "singlestoredb_cluster.this",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"admin_password"},
			},
			{
				Config: testutil.UpdatableConfig(examples.ClustersResource).
					WithClusterResource("this")("admin_password", cty.StringVal(updatedAdminPassword)).
					WithClusterResource("this")("expires_at", cty.StringVal(updatedExpiresAt)).
					WithClusterResource("this")("size", cty.StringVal(updatedClusterSize)).
					WithClusterResource("this")("deployment_type", cty.StringVal(string(updatedDeploymentType))).
					WithClusterResource("this")("firewall_ranges", cty.ListVal([]cty.Value{
					cty.StringVal(updatedFirewallRanges[0]),
					cty.StringVal(updatedFirewallRanges[1]),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", config.IDAttribute, clusterID.String()),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "admin_password", updatedAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "expires_at", updatedExpiresAt),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "size", updatedClusterSize),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "deployment_type", string(updatedDeploymentType)),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "firewall_ranges.#", "2"),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "firewall_ranges.0", updatedFirewallRanges[0]),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "firewall_ranges.1", updatedFirewallRanges[1]),
				),
			},
		},
	})
}

func TestClusterSuspendResume(t *testing.T) { //nolint:cyclop
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	groupID := uuid.MustParse("4db4e46a-132e-56fe-97dc-49c9e25bd618")
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	createdAt := time.Now().UTC()
	stateActive := management.ClusterStateACTIVE
	stateSuspended := management.ClusterStateSUSPENDED
	endpoint := testEndpoint
	provider := management.CloudProviderAWS
	region := testRegion
	size := config.TestInitialWorkspaceSize
	scaleFactor := float32(1)
	cacheConfig := float32(1)

	cluster := management.Cluster{
		AdminPassword:  util.Ptr(config.TestInitialAdminPassword),
		ClusterID:      &clusterID,
		CreatedAt:      &createdAt,
		DeploymentType: &defaultDeploymentType,
		Endpoint:       &endpoint,
		ExpiresAt:      util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges: util.Ptr([]string{config.TestInitialFirewallRange}),
		GroupID:        &groupID,
		Name:           "cluster-1",
		ProjectID:      projectID,
		Provider:       &provider,
		Region:         &region,
		SizeConfig: &management.SizeConfig{
			Size:        &size,
			ScaleFactor: &scaleFactor,
			CacheConfig: &cacheConfig,
		},
		State: &stateActive,
	}

	projectsHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != pathV2Projects || r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Project{{Name: projectName, ProjectID: projectID}}))
		require.NoError(t, err)

		return true
	}

	clustersGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{pathV2Clusters, clusterID.String()}, "/") || r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(cluster))
		require.NoError(t, err)

		return true
	}

	clustersPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, pathV2Clusters, r.URL.Path)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(struct {
			ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
		}{ClusterID: clusterID, GroupID: groupID}))
		require.NoError(t, err)
	}

	clustersSuspendHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{pathV2Clusters, clusterID.String(), "suspend"}, "/"), r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(struct {
			ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
		}{ClusterID: clusterID}))
		require.NoError(t, err)
		cluster.State = &stateSuspended
		cluster.Endpoint = nil
	}

	clustersResumeHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{pathV2Clusters, clusterID.String(), "resume"}, "/"), r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(struct {
			ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
		}{ClusterID: clusterID}))
		require.NoError(t, err)
		cluster.State = &stateActive
		cluster.Endpoint = &endpoint
	}

	clustersDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{pathV2Clusters, clusterID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(struct {
			ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
		}{ClusterID: clusterID}))
		require.NoError(t, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case projectsHandler(w, r):
			return
		case clustersGetHandler(w, r):
			return
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodPost:
			clustersPostHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/suspend") && r.Method == http.MethodPost:
			clustersSuspendHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/resume") && r.Method == http.MethodPost:
			clustersResumeHandler(w, r)
		case r.URL.Path == strings.Join([]string{pathV2Clusters, clusterID.String()}, "/") && r.Method == http.MethodDelete:
			clustersDeleteHandler(w, r)
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
				Config: examples.ClustersResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "endpoint", endpoint),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.ClustersResource).
					WithClusterResource("this")("suspended", cty.BoolVal(true)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "suspended", "true"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.ClustersResource).
					WithClusterResource("this")("suspended", cty.BoolVal(false)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "endpoint", endpoint),
				),
			},
		},
	})
}

func TestImmutableClusterAttributes(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	groupID := uuid.MustParse("4db4e46a-132e-56fe-97dc-49c9e25bd618")
	projectID := uuid.New()
	projectName := config.TestInitialProjectName
	createdAt := time.Now().UTC()
	stateActive := management.ClusterStateACTIVE
	provider := management.CloudProviderAWS
	region := testRegion
	size := config.TestInitialWorkspaceSize
	sf := float32(1)
	cc := float32(1)

	cluster := management.Cluster{
		ClusterID:      &clusterID,
		CreatedAt:      &createdAt,
		DeploymentType: &defaultDeploymentType,
		ExpiresAt:      util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges: util.Ptr([]string{config.TestInitialFirewallRange}),
		GroupID:        &groupID,
		Name:           "cluster-1",
		ProjectID:      projectID,
		Provider:       &provider,
		Region:         &region,
		SizeConfig:     &management.SizeConfig{Size: &size, ScaleFactor: &sf, CacheConfig: &cc},
		State:          &stateActive,
	}

	projectsHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != pathV2Projects || r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON([]management.Project{{Name: projectName, ProjectID: projectID}}))
		require.NoError(t, err)

		return true
	}

	clustersGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{pathV2Clusters, clusterID.String()}, "/") || r.Method != http.MethodGet {
			return false
		}
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(cluster))
		require.NoError(t, err)

		return true
	}

	clustersPostHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(struct {
			ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
		}{ClusterID: clusterID, GroupID: groupID}))
		require.NoError(t, err)
	}

	clustersDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(struct {
			ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
		}{ClusterID: clusterID}))
		require.NoError(t, err)
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{projectsHandler, clustersGetHandler}
	writeHandlers := []func(w http.ResponseWriter, r *http.Request){clustersPostHandler, clustersDeleteHandler}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}
		require.NotEmpty(t, writeHandlers)
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
			{Config: examples.ClustersResource},
			{
				Config: testutil.UpdatableConfig(examples.ClustersResource).
					WithClusterResource("this")("name", cty.StringVal("renamed")).
					String(),
				ExpectError: regexp.MustCompile("Cannot update cluster name"),
			},
		},
	})
}
