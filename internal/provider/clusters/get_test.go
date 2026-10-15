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

func TestReadsClusterByID(t *testing.T) {
	clusterID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	groupID := uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")
	createdAt := time.Date(2023, 2, 28, 5, 33, 6, 300300000, time.UTC)
	state := management.ClusterStateACTIVE
	provider := management.CloudProviderAWS
	region := testRegion
	size := "S-00"
	sf := float32(1)
	cc := float32(1)
	endpoint := testEndpoint
	idleAfter := 1200
	suspendType := management.SCHEDULED

	cluster := management.Cluster{
		ClusterID:  &clusterID,
		CreatedAt:  &createdAt,
		Endpoint:   &endpoint,
		GroupID:    &groupID,
		Name:       "foo",
		ProjectID:  uuid.New(),
		Provider:   &provider,
		Region:     &region,
		SizeConfig: &management.SizeConfig{Size: &size, ScaleFactor: &sf, CacheConfig: &cc},
		State:      &state,
		AutoSuspend: &management.AutoSuspend{
			SuspendType:           &suspendType,
			ScheduledAfterSeconds: &idleAfter,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v2/clusters/%s", clusterID), r.URL.Path)
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(cluster))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ClustersGetDataSource).
					WithClusterGetDataSource("this")(config.IDAttribute, cty.StringVal(clusterID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", config.IDAttribute, clusterID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "group_id", groupID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "name", cluster.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "state", string(state)),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "size", size),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "created_at", createdAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "endpoint", endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "auto_suspend.suspend_type", "SCHEDULED"),
					resource.TestCheckResourceAttr("data.singlestoredb_cluster.this", "auto_suspend.suspend_after_seconds", "1200"),
				),
			},
		},
	})
}

func TestClusterNotFoundByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ClustersGetDataSource).
					WithClusterGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestInvalidClusterInputUUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.False(t, true, "should not get here")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.ClustersGetDataSource).
					WithClusterGetDataSource("this")(config.IDAttribute, cty.StringVal("invalid-uuid")).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}
