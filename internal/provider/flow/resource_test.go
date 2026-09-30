package flow_test

import (
	"encoding/json"
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
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/flow"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

var (
	testWorkspaceGroupID  = uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	testWorkspaceID       = uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	testFlowInstanceID    = uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc")
	testFlowInstanceName  = "my-flow-instance"
	testFlowStatusRunning = "Running"
	testFlowEndpoint      = "example.com"
)

func newTestStarterCluster() management.Cluster {
	return management.Cluster{
		AllowAllTraffic: util.Ptr(false),
		CreatedAt:       util.Ptr(time.Now().UTC()),
		ExpiresAt:       util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:  util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:            config.TestInitialWorkspaceGroupName,
		Region:          util.Ptr("us-east-1"),
		Provider:        util.Ptr(management.CloudProviderAWS),
		State:           util.Ptr(management.ClusterStateACTIVE),
		TerminatedAt:    nil,
		UpdateWindow:    nil,
		GroupID:         util.Ptr(testWorkspaceGroupID),
		ClusterID:       util.Ptr(testWorkspaceID),
		ProjectID:       testWorkspaceGroupID, // placeholder project id for mocks
		DeploymentType:  util.Ptr(management.PRODUCTION),
		SizeConfig: &management.SizeConfig{
			Size:        util.Ptr(config.TestInitialWorkspaceSize),
			ScaleFactor: util.Ptr[float32](1),
			CacheConfig: util.Ptr[float32](1),
		},
	}
}

func newTestFlowInstance() management.FlowV2 {
	return management.FlowV2{
		FlowID:       testFlowInstanceID,
		Name:         testFlowInstanceName,
		ClusterID:    util.Ptr(testWorkspaceID),
		CreatedAt:    time.Now().UTC(),
		Endpoint:     util.Ptr(testFlowEndpoint),
		Size:         util.Ptr("F1"),
		Status:       util.Ptr(testFlowStatusRunning),
		UserName:     util.Ptr("admin"),
		DatabaseName: util.Ptr("my_database"),
	}
}

func writeJSONResponse(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()

	w.Header().Add("Content-Type", "json")
	_, err := w.Write(testutil.MustJSON(data))
	require.NoError(t, err)
}

func newFlowIDResponse() struct {
	FlowID uuid.UUID `json:"flowID"` //nolint:tagliatelle // API uses flowID.
} {
	return struct {
		FlowID uuid.UUID `json:"flowID"` //nolint:tagliatelle // API uses flowID.
	}{FlowID: testFlowInstanceID}
}

func setupCRUDServer(t *testing.T) *httptest.Server {
	t.Helper()

	server, _ := setupCRUDServerWithFlow(t)

	return server
}

func setupCRUDServerWithFlow(t *testing.T) (*httptest.Server, *management.FlowV2) {
	t.Helper()

	// Under /v2/clusters the workspace adopts this sole starter cluster.
	cluster := newTestStarterCluster()
	clusterExists := true
	flowInstance := newTestFlowInstance()
	clusterPath := strings.Join([]string{"/v2/clusters", testWorkspaceID.String()}, "/")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/projects" && r.Method == http.MethodGet:
			writeJSONResponse(t, w, []management.Project{{
				Name: "Standard Project", ProjectID: testWorkspaceGroupID, Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}})
		case r.URL.Path == "/v2/clusters" && r.Method == http.MethodGet:
			clusters := []management.Cluster{}
			if clusterExists {
				clusters = append(clusters, cluster)
			}
			writeJSONResponse(t, w, clusters)
		case r.URL.Path == "/v2/clusters" && r.Method == http.MethodPost:
			writeJSONResponse(t, w, struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
			}{
				ClusterID: testWorkspaceID,
				GroupID:   testWorkspaceGroupID,
			})
		case r.URL.Path == clusterPath && r.Method == http.MethodGet:
			if !clusterExists {
				w.WriteHeader(http.StatusNotFound)

				return
			}
			writeJSONResponse(t, w, cluster)
		case r.URL.Path == clusterPath && r.Method == http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			if input.Name == config.TestWorkspaceName {
				cluster.Name = config.TestWorkspaceName
				cluster.Endpoint = util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com")
				cluster.SizeConfig = &management.SizeConfig{
					Size:        util.Ptr(config.TestInitialWorkspaceSize),
					ScaleFactor: util.Ptr[float32](1),
					CacheConfig: util.Ptr[float32](1),
				}
			}
			writeJSONResponse(t, w, struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: testWorkspaceID})
		case r.URL.Path == clusterPath && r.Method == http.MethodDelete:
			clusterExists = false
			writeJSONResponse(t, w, struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: testWorkspaceID})
		case r.URL.Path == "/v2/flow" && r.Method == http.MethodPost:
			writeJSONResponse(t, w, newFlowIDResponse())
		case r.URL.Path == strings.Join([]string{"/v2/flow", testFlowInstanceID.String()}, "/") && r.Method == http.MethodGet:
			writeJSONResponse(t, w, &flowInstance)
		case r.URL.Path == strings.Join([]string{"/v2/flow", testFlowInstanceID.String()}, "/") && r.Method == http.MethodDelete:
			writeJSONResponse(t, w, newFlowIDResponse())
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	t.Cleanup(server.Close)

	return server, &flowInstance
}

func TestCRUDFlowInstance(t *testing.T) {
	server := setupCRUDServer(t)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", config.IDAttribute, testFlowInstanceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "name", testFlowInstanceName),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "cluster_id", testWorkspaceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "endpoint", testFlowEndpoint),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "size", "F1"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "user_name", "admin"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "database_name", "my_database"),
				),
			},
			{
				ResourceName:      "singlestoredb_flow.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestFlowInstanceImmutableName(t *testing.T) {
	server := setupCRUDServer(t)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "name", testFlowInstanceName),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal("different-name")).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				ExpectError: regexp.MustCompile(`Cannot update name`),
			},
		},
	})
}

func TestFlowInstanceUserNameConfigDriftDoesNotError(t *testing.T) {
	server := setupCRUDServer(t)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "user_name", "admin"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("different-user")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "user_name", "admin"),
				),
			},
		},
	})
}

func TestFlowInstanceReadRefreshesFromAPI(t *testing.T) {
	server, flowInstance := setupCRUDServerWithFlow(t)
	const migratedEndpoint = "migrated.example.com"

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "endpoint", testFlowEndpoint),
				),
			},
			{
				PreConfig: func() {
					flowInstance.Endpoint = util.Ptr(migratedEndpoint)
					flowInstance.UserName = util.Ptr("migrated_user")
					flowInstance.DatabaseName = util.Ptr("migrated_db")
				},
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "endpoint", migratedEndpoint),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "user_name", "migrated_user"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "database_name", "migrated_db"),
				),
			},
		},
	})
}

func TestFlowInstanceUnknownPlaceholderPreservedOnRead(t *testing.T) {
	server, flowInstance := setupCRUDServerWithFlow(t)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "user_name", "admin"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "database_name", "my_database"),
				),
			},
			{
				PreConfig: func() {
					flowInstance.UserName = util.Ptr("Unknown")
					flowInstance.DatabaseName = util.Ptr("Unknown")
				},
				Config: testutil.UpdatableConfig(examples.FlowResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(testFlowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "user_name", "admin"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "database_name", "my_database"),
				),
			},
		},
	})
}

func TestFlowFieldAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value *string
		want  bool
	}{
		{name: "nil", value: nil, want: false},
		{name: "empty", value: util.Ptr(""), want: false},
		{name: "unknown lowercase", value: util.Ptr("unknown"), want: false},
		{name: "unknown capitalized", value: util.Ptr("Unknown"), want: false},
		{name: "unknown uppercase", value: util.Ptr("UNKNOWN"), want: false},
		{name: "valid", value: util.Ptr("adam_ss_flow_rw"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, flow.FlowFieldAvailableForTest(tt.value))
		})
	}
}

func TestToFlowInstanceResourceModel(t *testing.T) {
	t.Parallel()

	workspaceID := uuid.New()
	priorUserName := "admin"
	priorDatabaseName := "my_database"

	t.Run("uses API values when available", func(t *testing.T) {
		t.Parallel()

		model := flow.ToFlowInstanceResourceModelForTest(management.FlowV2{
			FlowID:       uuid.New(),
			Name:         "flow",
			ClusterID:    util.Ptr(workspaceID),
			CreatedAt:    time.Now().UTC(),
			Endpoint:     util.Ptr("new.example.com"),
			Size:         util.Ptr("F1"),
			UserName:     util.Ptr("api_user"),
			DatabaseName: util.Ptr("api_db"),
		}, &priorUserName, &priorDatabaseName)

		require.Equal(t, "new.example.com", model.Endpoint)
		require.True(t, model.UserNameSet)
		require.Equal(t, "api_user", model.UserName)
		require.True(t, model.DatabaseSet)
		require.Equal(t, "api_db", model.DatabaseName)
	})

	t.Run("preserves prior user fields when API returns placeholder", func(t *testing.T) {
		t.Parallel()

		model := flow.ToFlowInstanceResourceModelForTest(management.FlowV2{
			FlowID:       uuid.New(),
			Name:         "flow",
			ClusterID:    util.Ptr(workspaceID),
			CreatedAt:    time.Now().UTC(),
			Endpoint:     util.Ptr("example.com"),
			Size:         util.Ptr("F1"),
			UserName:     util.Ptr("Unknown"),
			DatabaseName: util.Ptr("unknown"),
		}, &priorUserName, &priorDatabaseName)

		require.True(t, model.UserNameSet)
		require.Equal(t, "admin", model.UserName)
		require.True(t, model.DatabaseSet)
		require.Equal(t, "my_database", model.DatabaseName)
	})

	t.Run("leaves user fields unset without prior state", func(t *testing.T) {
		t.Parallel()

		model := flow.ToFlowInstanceResourceModelForTest(management.FlowV2{
			FlowID:       uuid.New(),
			Name:         "flow",
			ClusterID:    util.Ptr(workspaceID),
			CreatedAt:    time.Now().UTC(),
			UserName:     util.Ptr("Unknown"),
			DatabaseName: util.Ptr("Unknown"),
		}, nil, nil)

		require.False(t, model.UserNameSet)
		require.False(t, model.DatabaseSet)
	})
}

func TestFlowInstanceIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "example",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowResource).String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "name", "my-flow-instance"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "size", "F1"),
					resource.TestCheckResourceAttrSet("singlestoredb_flow.this", config.IDAttribute),
					resource.TestCheckResourceAttrSet("singlestoredb_flow.this", "endpoint"),
					resource.TestCheckResourceAttrSet("singlestoredb_flow.this", "cluster_id"),
				),
			},
		},
	})
}
