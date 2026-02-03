package flow_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestCRUDFlowInstance(t *testing.T) {
	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")

	workspaceGroup := management.WorkspaceGroup{
		AllowAllTraffic:  util.Ptr(false),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:             config.TestInitialWorkspaceGroupName,
		RegionName:       "us-east-1",
		State:            management.WorkspaceGroupStateACTIVE,
		TerminatedAt:     nil,
		UpdateWindow:     nil,
		WorkspaceGroupID: workspaceGroupID,
	}

	workspaceID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")

	workspace := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             config.TestWorkspaceName,
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      workspaceID,
		WorkspaceGroupID: workspaceGroup.WorkspaceGroupID,
		LastResumedAt:    nil,
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		Size:             config.TestInitialWorkspaceSize,
	}

	flowInstanceID := uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc")
	flowInstanceName := "my-flow-instance"
	flowInstanceEndpoint := "flow-svc-94a328d2-8c3d-412d-91a0-c32a750673cb.aws-oregon-3.svc.singlestore.com"

	flowInstance := management.Flow{
		FlowID:      flowInstanceID,
		Name:        flowInstanceName,
		WorkspaceID: util.Ptr(workspaceID),
		CreatedAt:   time.Now().UTC(),
		Endpoint:    util.Ptr(flowInstanceEndpoint),
		Size:        util.Ptr("F1"),
	}

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

	workspaceGroupsGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/workspaceGroups", workspaceGroupID.String()}, "/") ||
			r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspaceGroup))
		require.NoError(t, err)

		return true
	}

	workspacesGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/workspaces", workspaceID.String()}, "/") ||
			r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspace))
		require.NoError(t, err)

		return true
	}

	flowInstancePostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/flow", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				FlowID uuid.UUID `json:"flowID"`
			}{
				FlowID: flowInstanceID,
			},
		))
		require.NoError(t, err)
	}

	flowInstanceGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/flow", flowInstanceID.String()}, "/") ||
			r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(flowInstance))
		require.NoError(t, err)

		return true
	}

	flowInstanceDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/flow", flowInstanceID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(
			struct {
				FlowID uuid.UUID `json:"flowID"`
			}{
				FlowID: flowInstanceID,
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
				WorkspaceID uuid.UUID
			}{
				WorkspaceID: workspaceID,
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if workspaceGroupsGetHandler(w, r) {
			return
		}

		if workspacesGetHandler(w, r) {
			return
		}

		if flowInstanceGetHandler(w, r) {
			return
		}

		switch {
		case r.URL.Path == "/v1/workspaceGroups" && r.Method == http.MethodPost:
			workspaceGroupsPostHandler(w, r)
		case r.URL.Path == "/v1/workspaces" && r.Method == http.MethodPost:
			workspacesPostHandler(w, r)
		case r.URL.Path == "/v1/flow" && r.Method == http.MethodPost:
			flowInstancePostHandler(w, r)
		case r.URL.Path == strings.Join([]string{"/v1/flow", flowInstanceID.String()}, "/") && r.Method == http.MethodDelete:
			flowInstanceDeleteHandler(w, r)
		case r.URL.Path == strings.Join([]string{"/v1/workspaces", workspaceID.String()}, "/") && r.Method == http.MethodDelete:
			workspacesDeleteHandler(w, r)
		case r.URL.Path == strings.Join([]string{"/v1/workspaceGroups", workspaceGroupID.String()}, "/") && r.Method == http.MethodDelete:
			workspaceGroupsDeleteHandler(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowInstanceResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(flowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_flow_instance.this", config.IDAttribute, flowInstanceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_flow_instance.this", "name", flowInstanceName),
					resource.TestCheckResourceAttr("singlestoredb_flow_instance.this", "workspace_id", workspaceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_flow_instance.this", "endpoint", flowInstanceEndpoint),
					resource.TestCheckResourceAttr("singlestoredb_flow_instance.this", "size", "F1"),
					resource.TestCheckResourceAttr("singlestoredb_flow_instance.this", "user_name", "admin"),
					resource.TestCheckResourceAttr("singlestoredb_flow_instance.this", "database_name", "my_database"),
				),
			},
		},
	})
}

func TestFlowInstanceImport(t *testing.T) {
	flowInstanceID := uuid.MustParse("a1b2c3d4-5678-9abc-def0-123456789abc")
	workspaceID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")
	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	flowInstanceName := "my-flow-instance"
	flowInstanceEndpoint := "flow-svc-94a328d2-8c3d-412d-91a0-c32a750673cb.aws-oregon-3.svc.singlestore.com"

	workspaceGroup := management.WorkspaceGroup{
		AllowAllTraffic:  util.Ptr(false),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:             config.TestInitialWorkspaceGroupName,
		RegionName:       "us-east-1",
		State:            management.WorkspaceGroupStateACTIVE,
		WorkspaceGroupID: workspaceGroupID,
	}

	workspace := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             config.TestWorkspaceName,
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      workspaceID,
		WorkspaceGroupID: workspaceGroup.WorkspaceGroupID,
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		Size:             config.TestInitialWorkspaceSize,
	}

	flowInstance := management.Flow{
		FlowID:      flowInstanceID,
		Name:        flowInstanceName,
		WorkspaceID: util.Ptr(workspaceID),
		CreatedAt:   time.Now().UTC(),
		Endpoint:    util.Ptr(flowInstanceEndpoint),
		Size:        util.Ptr("F1"),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")

		switch {
		case r.URL.Path == fmt.Sprintf("/v1/flow/%s", flowInstanceID.String()) && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(flowInstance))
			require.NoError(t, err)
		case r.URL.Path == fmt.Sprintf("/v1/workspaces/%s", workspaceID.String()) && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(workspace))
			require.NoError(t, err)
		case r.URL.Path == fmt.Sprintf("/v1/workspaceGroups/%s", workspaceGroupID.String()) && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(workspaceGroup))
			require.NoError(t, err)
		case r.URL.Path == "/v1/workspaceGroups" && r.Method == http.MethodPost:
			_, err := w.Write(testutil.MustJSON(struct {
				WorkspaceGroupID uuid.UUID
			}{WorkspaceGroupID: workspaceGroupID}))
			require.NoError(t, err)
		case r.URL.Path == "/v1/workspaces" && r.Method == http.MethodPost:
			_, err := w.Write(testutil.MustJSON(struct {
				WorkspaceID uuid.UUID
			}{WorkspaceID: workspaceID}))
			require.NoError(t, err)
		case r.URL.Path == "/v1/flow" && r.Method == http.MethodPost:
			_, err := w.Write(testutil.MustJSON(struct {
				FlowID uuid.UUID `json:"flowID"`
			}{FlowID: flowInstanceID}))
			require.NoError(t, err)
		case r.Method == http.MethodDelete:
			_, err := w.Write(testutil.MustJSON(struct{}{}))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.FlowInstanceResource).
					WithFlowInstanceResource("this")("name", cty.StringVal(flowInstanceName)).
					WithFlowInstanceResource("this")("user_name", cty.StringVal("admin")).
					WithFlowInstanceResource("this")("database_name", cty.StringVal("my_database")).
					WithFlowInstanceResource("this")("size", cty.StringVal("F1")).
					String(),
			},
			{
				ResourceName:            "singlestoredb_flow_instance.this",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"user_name", "database_name"}, // Write-only fields not returned by API.
			},
		},
	})
}
