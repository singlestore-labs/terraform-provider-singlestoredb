package flow_test

import (
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
	testWorkspaceGroupID = uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	testWorkspaceID      = uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	testFlowInstanceID   = uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc")
	testFlowInstanceName = "my-flow-instance"
	testFlowEndpoint     = "flow-svc-94a328d2-8c3d-412d-91a0-c32a750673cb.aws-oregon-3.svc.singlestore.com"
)

func newTestWorkspaceGroup() management.WorkspaceGroup {
	return management.WorkspaceGroup{
		AllowAllTraffic:  util.Ptr(false),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:             config.TestInitialWorkspaceGroupName,
		RegionName:       "us-east-1",
		Provider:         management.CloudProviderAWS,
		State:            management.WorkspaceGroupStateACTIVE,
		TerminatedAt:     nil,
		UpdateWindow:     nil,
		WorkspaceGroupID: testWorkspaceGroupID,
		DeploymentType:   util.Ptr(management.WorkspaceGroupDeploymentTypePRODUCTION),
	}
}

func newTestWorkspace() management.Workspace {
	return management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             config.TestWorkspaceName,
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      testWorkspaceID,
		WorkspaceGroupID: testWorkspaceGroupID,
		LastResumedAt:    nil,
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		Size:             config.TestInitialWorkspaceSize,
		ScaleFactor:      util.Ptr[float32](1),
	}
}

func newTestFlowInstance() management.Flow {
	return management.Flow{
		FlowID:      testFlowInstanceID,
		Name:        testFlowInstanceName,
		WorkspaceID: util.Ptr(testWorkspaceID),
		CreatedAt:   time.Now().UTC(),
		Endpoint:    util.Ptr(testFlowEndpoint),
		Size:        util.Ptr("F1"),
	}
}

func createGetHandler(t *testing.T, expectedPath string, responseData any) func(w http.ResponseWriter, r *http.Request) bool {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != expectedPath || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(responseData))
		require.NoError(t, err)

		return true
	}
}

type routeKey struct {
	path   string
	method string
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

	workspaceGroup := newTestWorkspaceGroup()
	workspace := newTestWorkspace()
	flowInstance := newTestFlowInstance()

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		createGetHandler(t, strings.Join([]string{"/v1/workspaceGroups", testWorkspaceGroupID.String()}, "/"), workspaceGroup),
		createGetHandler(t, strings.Join([]string{"/v1/workspaces", testWorkspaceID.String()}, "/"), workspace),
		createGetHandler(t, strings.Join([]string{"/v1/flow", testFlowInstanceID.String()}, "/"), flowInstance),
	}

	writeRoutes := map[routeKey]func(w http.ResponseWriter){
		{"/v1/workspaceGroups", http.MethodPost}: func(w http.ResponseWriter) {
			writeJSONResponse(t, w, struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: testWorkspaceGroupID,
			})
		},
		{"/v1/workspaces", http.MethodPost}: func(w http.ResponseWriter) {
			writeJSONResponse(t, w, struct {
				WorkspaceID uuid.UUID
			}{
				WorkspaceID: testWorkspaceID,
			})
		},
		{"/v1/flow", http.MethodPost}: func(w http.ResponseWriter) {
			writeJSONResponse(t, w, newFlowIDResponse())
		},
		{strings.Join([]string{"/v1/flow", testFlowInstanceID.String()}, "/"), http.MethodDelete}: func(w http.ResponseWriter) {
			writeJSONResponse(t, w, newFlowIDResponse())
		},
		{strings.Join([]string{"/v1/workspaces", testWorkspaceID.String()}, "/"), http.MethodDelete}: func(w http.ResponseWriter) {
			writeJSONResponse(t, w, struct {
				WorkspaceID uuid.UUID
			}{
				WorkspaceID: testWorkspaceID,
			})
		},
		{strings.Join([]string{"/v1/workspaceGroups", testWorkspaceGroupID.String()}, "/"), http.MethodDelete}: func(w http.ResponseWriter) {
			writeJSONResponse(t, w, struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: testWorkspaceGroupID,
			})
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}

		if handler, ok := writeRoutes[routeKey{r.URL.Path, r.Method}]; ok {
			handler(w)

			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	t.Cleanup(server.Close)

	return server
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
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "workspace_id", testWorkspaceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "endpoint", testFlowEndpoint),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "size", "F1"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "user_name", "admin"),
					resource.TestCheckResourceAttr("singlestoredb_flow.this", "database_name", "my_database"),
				),
			},
		},
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
					resource.TestCheckResourceAttrSet("singlestoredb_flow.this", "workspace_id"),
				),
			},
		},
	})
}
