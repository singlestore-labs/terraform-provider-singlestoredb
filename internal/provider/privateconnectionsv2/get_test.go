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

func TestPrivateConnectionV2DataSource(t *testing.T) {
	connectionID := uuid.MustParse("44b5e1a5-a761-4ee0-8240-fb6381541600")
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	status := management.ClusterPrivateConnectionStatusACTIVE
	connectionType := management.ClusterPrivateConnectionTypeINBOUND
	regions := []string{"eu-west-1", "us-east-1"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/v2/privateConnections/"+connectionID.String(), r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(management.ClusterPrivateConnection{
			PrivateConnectionID:   connectionID,
			ClusterID:             &clusterID,
			AllowList:             util.Ptr("651246146166"),
			Status:                &status,
			Type:                  &connectionType,
			SupportedCrossRegions: &regions,
			ServiceName:           util.Ptr("test name"),
		}))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.PrivateConnectionV2GetDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection_v2.this", config.IDAttribute, connectionID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection_v2.this", "cluster_id", clusterID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection_v2.this", "allow_list", "651246146166"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection_v2.this", "status", "ACTIVE"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection_v2.this", "supported_cross_regions.0", "eu-west-1"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection_v2.this", "supported_cross_regions.1", "us-east-1"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connection_v2.this", "sql_port", "3306"),
				),
			},
		},
	})
}
