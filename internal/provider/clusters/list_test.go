package clusters_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestReadsClusters(t *testing.T) {
	projectID := uuid.MustParse("bc8c0deb-50dd-4a58-a5a5-1c62eb5c456d")
	createdAt1 := time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC)
	createdAt2 := time.Date(2023, 2, 29, 5, 33, 6, 300300000, time.UTC)
	stateActive := management.ClusterStateACTIVE
	stateSuspended := management.ClusterStateSUSPENDED
	size0 := "S-00"
	size1 := "S-1"
	sf := float32(1)
	cc := float32(1)
	endpoint := "svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"
	clusterID1 := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	clusterID2 := uuid.MustParse("f3a1a960-8591-4156-bb26-f53f0f8e35ce")
	groupID := uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")

	clusters := []management.Cluster{
		{
			ClusterID:  &clusterID1,
			CreatedAt:  &createdAt1,
			Endpoint:   &endpoint,
			GroupID:    &groupID,
			Name:       "foo",
			ProjectID:  projectID,
			SizeConfig: &management.SizeConfig{Size: &size0, ScaleFactor: &sf, CacheConfig: &cc},
			State:      &stateActive,
		},
		{
			ClusterID:  &clusterID2,
			CreatedAt:  &createdAt2,
			GroupID:    &groupID,
			Name:       "bar",
			ProjectID:  projectID,
			SizeConfig: &management.SizeConfig{Size: &size1, ScaleFactor: &sf, CacheConfig: &cc},
			State:      &stateSuspended,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/clusters", r.URL.Path)
		require.Equal(t, projectID.String(), r.URL.Query().Get("projectID"))

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(clusters))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ClustersListDataSource).
					WithClusterListDataSource("all")("project_id", cty.StringVal(projectID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", fmt.Sprintf("clusters.0.%s", config.IDAttribute), clusterID1.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.0.group_id", groupID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.0.name", "foo"),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.0.state", string(stateActive)),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.0.size", size0),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.0.suspended", "false"),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.0.created_at", createdAt1.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.0.endpoint", endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", fmt.Sprintf("clusters.1.%s", config.IDAttribute), clusterID2.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.1.name", "bar"),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.1.state", string(stateSuspended)),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.1.size", size1),
					resource.TestCheckResourceAttr("data.singlestoredb_clusters.all", "clusters.1.suspended", "true"),
					resource.TestCheckNoResourceAttr("data.singlestoredb_clusters.all", "clusters.1.endpoint"),
				),
			},
		},
	})
}

func TestReadClustersError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.ClustersListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}
