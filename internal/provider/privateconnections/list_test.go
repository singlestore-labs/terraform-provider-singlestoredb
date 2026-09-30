package privateconnections_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

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

const pathV2Clusters = "/v2/clusters"

func TestReadPrivateConnections(t *testing.T) {
	WorkspaceGroupID := uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")

	privateConnections := []management.ClusterPrivateConnection{
		{
			ActiveAt:            util.Ptr("2025-01-21T11:11:38.145343Z"),
			AllowList:           util.Ptr("12345"),
			CreatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
			UpdatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
			Endpoint:            util.Ptr("com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
			OutboundAllowList:   util.Ptr("127.0.0.0"),
			PrivateConnectionID: uuid.MustParse("c73ef470-68e6-46ac-9e98-6d5c29e48ba5"),
			ServiceName:         util.Ptr("test name"),
			Status:              util.Ptr(management.ClusterPrivateConnectionStatusACTIVE),
			Type:                util.Ptr(management.ClusterPrivateConnectionTypeOUTBOUND),
			ClusterID:           util.Ptr(uuid.MustParse("fe10982b-c0b6-4c36-b8e7-ce56c5eb0636")),
		},
		{
			ActiveAt:            util.Ptr("2022-01-21T11:11:38.145343Z"),
			AllowList:           util.Ptr("1111"),
			CreatedAt:           util.Ptr("2022-01-21T11:11:38.145343Z"),
			OutboundAllowList:   nil,
			PrivateConnectionID: uuid.MustParse("20d49abc-5900-4836-b896-2a29a59f183e"),
			ServiceName:         util.Ptr("private"),
			Status:              util.Ptr(management.ClusterPrivateConnectionStatusDELETED),
			Type:                util.Ptr(management.ClusterPrivateConnectionTypeINBOUND),
			ClusterID:           util.Ptr(uuid.MustParse("d92fc918-041e-4637-973e-10bbbb956a0a")),
		},
	}

	clusterID := uuid.MustParse("fe10982b-c0b6-4c36-b8e7-ce56c5eb0636")
	clusters := []management.Cluster{{
		ClusterID: util.Ptr(clusterID),
		GroupID:   util.Ptr(WorkspaceGroupID),
		Name:      "c",
		ProjectID: WorkspaceGroupID,
		State:     util.Ptr(management.ClusterStateACTIVE),
	}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch {
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(clusters))
			require.NoError(t, err)
		case r.URL.Path == fmt.Sprintf("%s/%s/privateConnections", pathV2Clusters, clusterID):
			_, err := w.Write(testutil.MustJSON(privateConnections))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsListDataSource).
					WithPrivateConnectionListDataSource("all")(config.WorkspaceGroupIDAttribute, cty.StringVal(WorkspaceGroupID.String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", fmt.Sprintf("private_connections.0.%s", config.IDAttribute), privateConnections[0].PrivateConnectionID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.active_at", *privateConnections[0].ActiveAt),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.created_at", *privateConnections[0].CreatedAt),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.endpoint", *privateConnections[0].Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.outbound_allow_list", *privateConnections[0].OutboundAllowList),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.allow_list", *privateConnections[0].AllowList),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.service_name", *privateConnections[0].ServiceName),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.status", "ACTIVE"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.type", "OUTBOUND"),
					resource.TestCheckResourceAttr("data.singlestoredb_private_connections.all", "private_connections.0.cluster_id", util.Deref(privateConnections[0].ClusterID).String()),
					resource.TestCheckNoResourceAttr("data.singlestoredb_private_connections.all", "private_connections.1.endpoint"),
					resource.TestCheckNoResourceAttr("data.singlestoredb_private_connections.all", "private_connections.1.updated_at"),
				),
			},
		},
	})
}

func TestReadPrivateConnectionsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.PrivateConnectionsListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}
