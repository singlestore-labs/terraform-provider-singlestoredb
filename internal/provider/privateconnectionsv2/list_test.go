package privateconnectionsv2_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestPrivateConnectionsV2DataSource(t *testing.T) {
	clusterID := uuid.MustParse("319a4b58-0f84-4933-bc19-3b5d11297a1a")
	status := management.ClusterPrivateConnectionStatusACTIVE
	connectionType := management.ClusterPrivateConnectionTypeOUTBOUND
	connections := []management.ClusterPrivateConnection{
		{
			PrivateConnectionID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
			ClusterID:           &clusterID,
			ServiceName:         util.Ptr("com.amazonaws.vpce.svc"),
			Status:              &status,
			Type:                &connectionType,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v2/clusters/"+clusterID.String()+"/privateConnections", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(connections))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.PrivateConnectionsV2ListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections_v2.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections_v2.all", "cluster_id", clusterID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections_v2.all", "private_connections.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections_v2.all", "private_connections.0.type", "OUTBOUND"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections_v2.all", "private_connections.0.service_name", "com.amazonaws.vpce.svc"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections_v2.all", "private_connections.0.cluster_id", clusterID.String()),
				),
			},
		},
	})
}
