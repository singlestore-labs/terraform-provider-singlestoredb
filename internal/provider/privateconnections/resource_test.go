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
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	updateAllowedList   = strings.Join([]string{"updated", "12345"}, "-")
	privateConnectionID = uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	workspaceID         = uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	workspaceGroupID    = uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")

	regions = []management.Region{
		{
			RegionID: uuid.MustParse("2ca3d358-021d-45ed-86cb-38b8d14ac507"),
			Region:   "GS - US West 2 (Oregon) - aws-oregon-gs1",
			Provider: management.AWS,
		},
	}

	workspaceGroup = management.WorkspaceGroup{
		AllowAllTraffic:  util.Ptr(false),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:             config.TestInitialWorkspaceGroupName,
		RegionID:         regions[0].RegionID,
		State:            management.WorkspaceGroupStateACTIVE,
		TerminatedAt:     nil,
		UpdateWindow:     nil,
		WorkspaceGroupID: workspaceGroupID,
	}

	workspace = management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             config.TestWorkspaceName,
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      workspaceID,
		WorkspaceGroupID: workspaceGroupID,
		LastResumedAt:    nil,
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		KaiEnabled:       util.Ptr(true),
		Size:             config.TestInitialWorkspaceSize,
		ScaleFactor:      util.MaybeFloat32(types.Float32Value(1)),
	}

	privateConnection = management.PrivateConnection{
		ActiveAt:            util.Ptr("2025-01-21T11:11:38.145343Z"),
		AllowList:           util.Ptr("12345"),
		CreatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		UpdatedAt:           util.Ptr("2025-01-21T11:11:38.145343Z"),
		Endpoint:            util.Ptr("com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406"),
		OutboundAllowList:   util.Ptr("127.0.0.0"),
		PrivateConnectionID: privateConnectionID,
		ServiceName:         util.Ptr(string("test name")),
		Status:              util.Ptr(management.PrivateConnectionStatusACTIVE),
		Type:                util.Ptr(management.PrivateConnectionTypeINBOUND),
		WorkspaceID:         util.Ptr(workspaceID),
		WorkspaceGroupID:    workspaceGroupID,
	}
)

func TestCRUDPrivateConnection(t *testing.T) {
	regionsHandler := createGetHandler(t, "/v1/regions", http.MethodGet, regions)

	workspaceGroupsGetHandler := createGetHandler(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupID.String()}, "/"), http.MethodGet, workspaceGroup)

	workspacesGetHandler := createGetHandler(t, strings.Join([]string{"/v1/workspaces", workspaceID.String()}, "/"), http.MethodGet, workspace)

	workspaceGroupsPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaceGroups", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: workspaceGroupID,
			},
		))
		require.NoError(t, err)
	}

	workspaceGroupsDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: workspaceGroupID,
			},
		))
		require.NoError(t, err)
	}

	workspacesDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaces", workspaceID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				WorkspaceGroupID uuid.UUID
			}{
				WorkspaceGroupID: workspaceGroupID,
			},
		))
		require.NoError(t, err)
	}

	workspacesPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				WorkspaceID uuid.UUID
			}{
				WorkspaceID: workspaceID,
			},
		))
		require.NoError(t, err)
	}

	returnNotFound := true
	privateConnectionsGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/privateConnections", privateConnectionID.String()}, "/") ||
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
		_, err := w.Write(testutil.MustJSON(privateConnection))
		require.NoError(t, err)
		privateConnection.Status = util.Ptr(management.PrivateConnectionStatusACTIVE) // Marking the state as ACTIVE to end polling.

		return true
	}

	privateConnectionsPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/privateConnections", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.PrivateConnectionCreate
		require.NoError(t, json.Unmarshal(body, &input))
		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				PrivateConnectionID uuid.UUID
			}{
				PrivateConnectionID: privateConnectionID,
			},
		))
		require.NoError(t, err)
	}

	returnInternalError := true
	privateConnectionsPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/privateConnections", privateConnectionID.String()}, "/"), r.URL.Path)

		if returnInternalError {
			w.Header().Add("Content-Type", "json")
			w.WriteHeader(http.StatusInternalServerError)

			returnInternalError = false

			return
		}

		require.Equal(t, http.MethodPatch, r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.PrivateConnectionUpdate
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, updateAllowedList, util.Deref(input.AllowList))
		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				PrivateConnectionID uuid.UUID
			}{
				PrivateConnectionID: privateConnectionID,
			},
		))
		require.NoError(t, err)
		privateConnection.AllowList = input.AllowList // Finally, the AllowList after resume.
	}

	privateConnectionsDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/privateConnections", privateConnectionID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				PrivateConnectionID uuid.UUID
			}{
				PrivateConnectionID: privateConnectionID,
			},
		))
		require.NoError(t, err)
	}

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		regionsHandler,
		workspaceGroupsGetHandler,
		workspacesGetHandler,
		privateConnectionsGetHandler,
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		workspaceGroupsPostHandler,
		workspacesPostHandler,
		privateConnectionsPostHandler,
		privateConnectionsPatchHandler,
		privateConnectionsPatchHandler, // retry
		privateConnectionsDeleteHandler,
		workspacesDeleteHandler,
		workspaceGroupsDeleteHandler,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range readOnlyHandlers {
			if h(w, r) {
				return
			}
		}

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
				Config: examples.PrivateConnectionsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", config.IDAttribute, privateConnectionID.String()),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "active_at", string("2025-01-21T11:11:38.145343Z")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "allow_list", string("12345")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "created_at", string("2025-01-21T11:11:38.145343Z")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "endpoint", string("com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "outbound_allow_list", string("127.0.0.0")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "service_name", string("test name")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "status", string("ACTIVE")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "type", string("INBOUND")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "workspace_id", string("283d4b0d-b0d6-485a-bc2d-a763c523c68a")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "workspace_group_id", string("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "updated_at", string("2025-01-21T11:11:38.145343Z")),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsResource).
					WitPrivateConnectionResource("this")("allow_list", cty.StringVal(updateAllowedList)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", config.IDAttribute, privateConnectionID.String()),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "active_at", string("2025-01-21T11:11:38.145343Z")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "allow_list", updateAllowedList),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "created_at", string("2025-01-21T11:11:38.145343Z")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "endpoint", string("com.amazonaws.vpce.eu-central-1.vpce-svc-074a8eb58bb50c406")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "outbound_allow_list", string("127.0.0.0")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "service_name", string("test name")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "status", string("ACTIVE")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "type", string("INBOUND")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "workspace_id", string("283d4b0d-b0d6-485a-bc2d-a763c523c68a")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "workspace_group_id", string("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "updated_at", string("2025-01-21T11:11:38.145343Z")),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func createGetHandler(t *testing.T, expectedPath string, expectedMethod string, responseData interface{}) func(w http.ResponseWriter, r *http.Request) bool {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != expectedPath || r.Method != expectedMethod {
			return false
		}

		w.Header().Add("Content-Type", "application/json")
		_, err := w.Write(testutil.MustJSON(responseData))
		require.NoError(t, err)

		return true
	}
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
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "allow_list", string("12345")),
					resource.TestCheckResourceAttr("singlestoredb_private_connection.this", "type", string("INBOUND")),
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection.this", "workspace_id"),
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection.this", "workspace_group_id"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.PrivateConnectionsResource).
					WitPrivateConnectionResource("this")("allow_list", cty.StringVal(updateAllowedList)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_private_connection.this", config.IDAttribute),
				),
			},
		},
	})
}
