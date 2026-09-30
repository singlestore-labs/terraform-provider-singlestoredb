package privateconnections_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
	allowedList         = "651246146166"
	updateAllowedList   = strings.Join([]string{"updated", allowedList}, "-")
	privateConnectionID = uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	workspaceID         = uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	workspaceGroupID    = uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")
	projectID           = uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
)

func TestCRUDPrivateConnection(t *testing.T) { //nolint:cyclop
	// Under /v2/clusters the workspace adopts the workspace_group starter cluster.
	cluster := management.Cluster{
		AllowAllTraffic: util.Ptr(false),
		CreatedAt:       util.Ptr(time.Now().UTC()),
		ExpiresAt:       util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:  util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:            config.TestInitialWorkspaceGroupName,
		Provider:        util.Ptr(management.CloudProviderAWS),
		Region:          util.Ptr("us-west-2"),
		State:           util.Ptr(management.ClusterStateACTIVE),
		GroupID:         util.Ptr(workspaceGroupID),
		ClusterID:       util.Ptr(workspaceID),
		ProjectID:       projectID,
		DeploymentType:  util.Ptr(management.PRODUCTION),
		SizeConfig:      &management.SizeConfig{Size: util.Ptr("S-00"), ScaleFactor: util.Ptr[float32](1)},
	}

	privateConnection := management.ClusterPrivateConnection{
		ActiveAt:            util.Ptr("2025-01-21T11:11:38.145343Z"),
		AllowList:           util.Ptr(allowedList),
		CreatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		UpdatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		Endpoint:            util.Ptr("com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
		OutboundAllowList:   util.Ptr("127.0.0.0"),
		PrivateConnectionID: privateConnectionID,
		ServiceName:         util.Ptr("test name"),
		Status:              util.Ptr(management.ClusterPrivateConnectionStatusACTIVE),
		Type:                util.Ptr(management.ClusterPrivateConnectionTypeINBOUND),
		ClusterID:           util.Ptr(workspaceID),
	}

	clusterExists := true
	postClusters := 0
	adoptPatches := 0
	patchAttempts := 0
	pcCreated := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		clusterPath := strings.Join([]string{pathV2Clusters, workspaceID.String()}, "/")
		pcPath := strings.Join([]string{"/v2/privateConnections", privateConnectionID.String()}, "/")

		switch {
		case r.URL.Path == "/v2/projects" && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{
				Name: "Standard Project", ProjectID: projectID, Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}}))
			require.NoError(t, err)
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodGet:
			clusters := []management.Cluster{}
			if clusterExists {
				clusters = append(clusters, cluster)
			}
			_, err := w.Write(testutil.MustJSON(clusters))
			require.NoError(t, err)
		case r.URL.Path == pathV2Clusters && r.Method == http.MethodPost:
			postClusters++
			require.Equal(t, 1, postClusters, "workspace create should adopt the starter cluster, not POST a second cluster")
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
			}{ClusterID: workspaceID, GroupID: workspaceGroupID}))
			require.NoError(t, err)
		case r.URL.Path == clusterPath && r.Method == http.MethodGet:
			if !clusterExists {
				w.WriteHeader(http.StatusNotFound)

				return
			}
			_, err := w.Write(testutil.MustJSON(cluster))
			require.NoError(t, err)
		case r.URL.Path == clusterPath && r.Method == http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			if input.Name == config.TestWorkspaceName && adoptPatches == 0 {
				adoptPatches++
				cluster.Name = config.TestWorkspaceName
				cluster.Endpoint = util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com")
				cluster.SizeConfig = &management.SizeConfig{
					Size:        util.Ptr(config.TestInitialWorkspaceSize),
					ScaleFactor: util.Ptr[float32](1),
				}
				_, err = w.Write(testutil.MustJSON(struct {
					ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				}{ClusterID: workspaceID}))
				require.NoError(t, err)

				return
			}
			_, err = w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: workspaceID}))
			require.NoError(t, err)
		case r.URL.Path == "/v2/privateConnections" && r.Method == http.MethodPost:
			pcCreated = true
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.PrivateConnectionCreateV2
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, workspaceID, input.ClusterID)
			_, err = w.Write(testutil.MustJSON(struct {
				PrivateConnectionID uuid.UUID `json:"privateConnectionID"` //nolint:tagliatelle // API uses privateConnectionID.
			}{PrivateConnectionID: privateConnectionID}))
			require.NoError(t, err)
		case r.URL.Path == pcPath && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(privateConnection))
			require.NoError(t, err)
		case r.URL.Path == pcPath && r.Method == http.MethodPatch:
			patchAttempts++
			if patchAttempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.PrivateConnectionUpdateV2
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, updateAllowedList, util.Deref(input.AllowList))
			privateConnection.AllowList = input.AllowList
			_, err = w.Write(testutil.MustJSON(struct {
				PrivateConnectionID uuid.UUID `json:"privateConnectionID"` //nolint:tagliatelle // API uses privateConnectionID.
			}{PrivateConnectionID: privateConnectionID}))
			require.NoError(t, err)
		case r.URL.Path == pcPath && r.Method == http.MethodDelete:
			_, err := w.Write(testutil.MustJSON(struct {
				PrivateConnectionID uuid.UUID `json:"privateConnectionID"` //nolint:tagliatelle // API uses privateConnectionID.
			}{PrivateConnectionID: privateConnectionID}))
			require.NoError(t, err)
		case strings.HasPrefix(r.URL.Path, pathV2Clusters+"/") && r.Method == http.MethodDelete:
			clusterExists = false
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{}))
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
				Config: examples.PrivateConnectionsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", config.IDAttribute, privateConnectionID.String()),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "active_at", "2025-01-21T11:11:38.145343Z"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "allow_list", allowedList),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "created_at", "2025-01-21T11:11:38.145343Z"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "endpoint", "com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "outbound_allow_list", "127.0.0.0"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "service_name", "test name"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "status", "ACTIVE"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "type", "INBOUND"),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "cluster_id", workspaceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "updated_at", "2025-01-21T11:11:38.145343Z"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsResource).
					WithPrivateConnectionResource("this")("allow_list", cty.StringVal(updateAllowedList)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", config.IDAttribute, privateConnectionID.String()),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "allow_list", updateAllowedList),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "cluster_id", workspaceID.String()),
				),
			},
		},
	})

	require.True(t, pcCreated)
	require.Equal(t, 1, adoptPatches)
	require.GreaterOrEqual(t, patchAttempts, 2)
}

func TestPrivateConnectionResourceIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.PrivateConnectionsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "allow_list", allowedList),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "type", "INBOUND"),
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection.this", "cluster_id"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsResource).
					WithPrivateConnectionResource("this")("allow_list", cty.StringVal(updateAllowedList)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection.this", config.IDAttribute),
				),
			},
		},
	})
}
