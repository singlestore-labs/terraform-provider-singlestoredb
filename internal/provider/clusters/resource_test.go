package clusters_test

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
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
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
	defaultDeploymentType = management.ClusterDeploymentTypePRODUCTION
	updatedDeploymentType = management.ClusterDeploymentTypeNONPRODUCTION
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
		require.Nil(t, input.AdminPassword, "create must not send adminPassword; the API generates it")
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
		require.Nil(t, input.AdminPassword, "update must not send adminPassword")
		require.Nil(t, input.ExpiresAt, "update must not send expiresAt while PATCH /v2/clusters ignores it")
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
		cluster.FirewallRanges = &updatedFirewallRanges
		cluster.DeploymentType = &updatedDeploymentType
		cluster.SizeConfig.Size = &updatedClusterSize
	}

	clustersDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{pathV2Clusters, clusterID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)
		cluster.State = util.Ptr(management.ClusterStateTERMINATED)

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
				ResourceName:      "singlestoredb_cluster.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testutil.UpdatableConfig(examples.ClustersResource).
					WithClusterResource("this")("size", cty.StringVal(updatedClusterSize)).
					WithClusterResource("this")("deployment_type", cty.StringVal(string(updatedDeploymentType))).
					WithClusterResource("this")("firewall_ranges", cty.ListVal([]cty.Value{
					cty.StringVal(updatedFirewallRanges[0]),
					cty.StringVal(updatedFirewallRanges[1]),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", config.IDAttribute, clusterID.String()),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "admin_password", config.TestInitialAdminPassword),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "expires_at", config.TestInitialWorkspaceGroupExpiresAt),
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
		cluster.State = util.Ptr(management.ClusterStateTERMINATED)
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
		cluster.State = util.Ptr(management.ClusterStateTERMINATED)
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

func TestClusterExpiresAtIsCreateOnly(t *testing.T) {
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

	readOnly := func(w http.ResponseWriter, r *http.Request) bool {
		switch {
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON([]management.Project{{Name: projectName, ProjectID: projectID}}))
			require.NoError(t, err)

			return true
		case r.URL.Path == strings.Join([]string{pathV2Clusters, clusterID.String()}, "/") && r.Method == http.MethodGet:
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(cluster))
			require.NoError(t, err)

			return true
		default:
			return false
		}
	}
	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
			}{ClusterID: clusterID, GroupID: groupID}))
			require.NoError(t, err)
		},
		func(w http.ResponseWriter, _ *http.Request) {
			cluster.State = util.Ptr(management.ClusterStateTERMINATED)
			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: clusterID}))
			require.NoError(t, err)
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if readOnly(w, r) {
			return
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
					WithClusterResource("this")("expires_at", cty.StringVal("2222-06-01T00:00:00Z")).
					String(),
				ExpectError: regexp.MustCompile("Cannot change cluster expires_at"),
			},
		},
	})
}

func TestClusterUpdateWindowIsNotConfigurable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ClustersResource).
					WithClusterResource("this")("update_window", cty.ObjectVal(map[string]cty.Value{
					"day":  cty.NumberIntVal(config.TestInitialUpdateWindowDay),
					"hour": cty.NumberIntVal(config.TestInitialUpdateWindowHour),
				})).
					String(),
				ExpectError: regexp.MustCompile(`Invalid Configuration for Read-Only Attribute`),
			},
		},
	})
}

func TestClusterAdminPasswordIsNotConfigurable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Failf(t, "unexpected request", "%s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ClustersResource).
					WithClusterResource("this")("admin_password", cty.StringVal(config.TestInitialAdminPassword)).
					String(),
				ExpectError: regexp.MustCompile(`Invalid Configuration for Read-Only Attribute`),
			},
		},
	})
}

func TestClusterCreateWaitFailureTracksCluster(t *testing.T) { //nolint:cyclop
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	groupID := uuid.MustParse("4db4e46a-132e-56fe-97dc-49c9e25bd618")
	projectID := uuid.New()
	stateFailed := management.ClusterStateFAILED
	stateActive := management.ClusterStateACTIVE
	provider := management.CloudProviderAWS
	region := testRegion
	size := config.TestInitialWorkspaceSize
	scaleFactor := float32(1)
	cacheConfig := float32(1)
	var gets int
	var deleted bool
	var mutations []string

	active := management.Cluster{
		AllowAllTraffic: util.Ptr(true),
		ClusterID:       &clusterID,
		ExpiresAt:       util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		GroupID:         &groupID,
		Name:            "cluster-1",
		ProjectID:       projectID,
		Provider:        &provider,
		Region:          &region,
		SizeConfig: &management.SizeConfig{
			Size:        &size,
			ScaleFactor: &scaleFactor,
			CacheConfig: &cacheConfig,
		},
		State: &stateActive,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		clusterPath := strings.Join([]string{pathV2Clusters, clusterID.String()}, "/")
		switch {
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{Name: config.TestInitialProjectName, ProjectID: projectID}}))
			require.NoError(t, err)
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodPost:
			mutations = append(mutations, http.MethodPost)
			deleted = false
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
			}{ClusterID: clusterID, GroupID: groupID}))
			require.NoError(t, err)
		case r.URL.Path == clusterPath && r.Method == http.MethodGet && deleted:
			w.WriteHeader(http.StatusNotFound)
		case r.URL.Path == clusterPath && r.Method == http.MethodGet:
			gets++
			cluster := active
			if gets == 1 {
				cluster.State = &stateFailed
			}
			_, err := w.Write(testutil.MustJSON(cluster))
			require.NoError(t, err)
		case r.URL.Path == clusterPath && r.Method == http.MethodDelete:
			mutations = append(mutations, http.MethodDelete)
			deleted = true
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: clusterID}))
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
				Config:      examples.ClustersResource,
				ExpectError: regexp.MustCompile("Failed to wait for a cluster"),
			},
			{
				// A failed create taints the saved cluster, so the next apply deletes
				// it before provisioning a replacement.
				Config: examples.ClustersResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", config.IDAttribute, clusterID.String()),
				),
			},
		},
	})

	require.Equal(t, []string{http.MethodPost, http.MethodDelete, http.MethodPost, http.MethodDelete}, mutations)
}

func TestClusterCreatePersistentFailureCanBeReplaced(t *testing.T) { //nolint:cyclop
	failedID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	replacementID := uuid.MustParse("5ec5f57b-243f-67af-a8ed-5ad0f36ce729")
	groupID := uuid.MustParse("4db4e46a-132e-56fe-97dc-49c9e25bd618")
	projectID := uuid.New()
	stateFailed := management.ClusterStateFAILED
	stateActive := management.ClusterStateACTIVE
	provider := management.CloudProviderAWS
	region := testRegion
	size := config.TestInitialWorkspaceSize
	scaleFactor := float32(1)
	cacheConfig := float32(1)
	var mutations []string
	posts := 0
	deleted := map[uuid.UUID]bool{}

	clusterFor := func(id uuid.UUID, state management.ClusterState) management.Cluster {
		return management.Cluster{
			AllowAllTraffic: util.Ptr(true),
			ClusterID:       &id,
			ExpiresAt:       util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
			GroupID:         &groupID,
			Name:            "cluster-1",
			ProjectID:       projectID,
			Provider:        &provider,
			Region:          &region,
			SizeConfig: &management.SizeConfig{
				Size:        &size,
				ScaleFactor: &scaleFactor,
				CacheConfig: &cacheConfig,
			},
			State: &state,
		}
	}
	clusters := map[string]management.Cluster{
		strings.Join([]string{pathV2Clusters, failedID.String()}, "/"):      clusterFor(failedID, stateFailed),
		strings.Join([]string{pathV2Clusters, replacementID.String()}, "/"): clusterFor(replacementID, stateActive),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		cluster, known := clusters[r.URL.Path]
		switch {
		case r.URL.Path == pathV2Projects && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{Name: config.TestInitialProjectName, ProjectID: projectID}}))
			require.NoError(t, err)
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodPost:
			posts++
			id := failedID
			if posts > 1 {
				require.True(t, deleted[failedID], "the failed cluster must be deleted before its replacement is created")
				id = replacementID
			}
			mutations = append(mutations, http.MethodPost+" "+id.String())
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
			}{ClusterID: id, GroupID: groupID}))
			require.NoError(t, err)
		case known && r.Method == http.MethodGet && deleted[*cluster.ClusterID]:
			w.WriteHeader(http.StatusNotFound)
		case known && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(cluster))
			require.NoError(t, err)
		case known && r.Method == http.MethodDelete:
			mutations = append(mutations, http.MethodDelete+" "+cluster.ClusterID.String())
			deleted[*cluster.ClusterID] = true
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: *cluster.ClusterID}))
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
				Config:      examples.ClustersResource,
				ExpectError: regexp.MustCompile("Failed to wait for a cluster"),
			},
			{
				// Refresh still sees FAILED and must keep the tainted cluster in state,
				// so this apply deletes it and provisions a new cluster.
				Config: examples.ClustersResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", config.IDAttribute, replacementID.String()),
				),
			},
		},
	})

	require.Equal(t, []string{
		http.MethodPost + " " + failedID.String(),
		http.MethodDelete + " " + failedID.String(),
		http.MethodPost + " " + replacementID.String(),
		http.MethodDelete + " " + replacementID.String(),
	}, mutations)
}

func TestClusterResourceIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				// IntegrationTest rewrites name to config.TestClusterName and expires_at
				// to a timestamp config.TestWorkspaceGroupExpiration from now.
				Config: examples.ClustersResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_cluster.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "name", config.TestClusterName),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "project_name", config.TestInitialProjectName),
					resource.TestCheckResourceAttr("singlestoredb_cluster.this", "suspended", "false"),
					resource.TestCheckResourceAttrSet("singlestoredb_cluster.this", "admin_password"),
					resource.TestCheckResourceAttrWith("singlestoredb_cluster.this", "expires_at", integrationExpiresAtIsSoon),
					clusterAdminCanConnect("singlestoredb_cluster.this"),
				),
			},
		},
	})
}

// clusterAdminCanConnect logs in with the password Terraform read from the API.
func clusterAdminCanConnect(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %s not in state", resourceName)
		}

		password := rs.Primary.Attributes["admin_password"]
		if password == "" {
			return fmt.Errorf("admin_password was empty after reading the cluster from the Management API")
		}

		return testutil.IsConnectableWithAdminPassword(password)(rs.Primary.Attributes["endpoint"])
	}
}

func integrationExpiresAtIsSoon(value string) error {
	expires, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return fmt.Errorf("expires_at %q is not a timestamp", value)
	}

	now := time.Now().UTC()
	if !expires.After(now) {
		return fmt.Errorf("expires_at %s is not in the future", value)
	}

	latest := now.Add(config.TestWorkspaceGroupExpiration + 15*time.Minute)
	if expires.After(latest) {
		return fmt.Errorf("expires_at %s is later than the integration-test safeguard of %s", value, config.TestWorkspaceGroupExpiration)
	}

	return nil
}
