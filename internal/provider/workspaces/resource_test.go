package workspaces_test

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

var updatedWorkspaceSize = "S-0"

func TestCRUDWorkspace(t *testing.T) { //nolint:cyclop,maintidx
	newEndpoint := util.Ptr("svc-14a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com")

	regions := []management.Region{
		{
			RegionID: uuid.MustParse("2ca3d358-021d-45ed-86cb-38b8d14ac507"),
			Region:   "GS - US West 2 (Oregon) - aws-oregon-gs1",
			Provider: management.AWS,
		},
	}

	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")

	workspaceGroup := management.WorkspaceGroup{
		AllowAllTraffic:  util.Ptr(false),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:             config.TestInitialWorkspaceGroupName,
		RegionID:         regions[0].RegionID,
		State:            management.ACTIVE,
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

	regionsHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != "/v1/regions" || r.Method != http.MethodGet {
			return false
		}

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(regions))
		require.NoError(t, err)

		return true
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

	returnNotFound := true
	workspacesGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/workspaces", workspaceID.String()}, "/") ||
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
		_, err := w.Write(testutil.MustJSON(workspace))
		require.NoError(t, err)

		return true
	}

	workspacesSuspendHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaces", workspace.WorkspaceID.String(), "suspend"}, "/"), r.URL.Path)
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

		workspace.State = management.WorkspaceStateSUSPENDED
		workspace.Endpoint = nil
	}

	workspacesResumeHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaces", workspace.WorkspaceID.String(), "resume"}, "/"), r.URL.Path)
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

		workspace.State = management.WorkspaceStateACTIVE
		workspace.Endpoint = util.Ptr("svc-14a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com") // New endpoint.
	}

	returnInternalError := true
	workspacesPatchHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaces", workspace.WorkspaceID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodPatch, r.Method)

		if returnInternalError {
			w.Header().Add("Content-Type", "json")
			w.WriteHeader(http.StatusInternalServerError)

			returnInternalError = false

			return
		}

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var input management.WorkspaceUpdate
		require.NoError(t, json.Unmarshal(body, &input))
		require.Equal(t, updatedWorkspaceSize, util.Deref(input.Size))

		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(
			struct {
				WorkspaceID uuid.UUID
			}{
				WorkspaceID: workspaceID,
			},
		))
		require.NoError(t, err)
		workspace.Size = *input.Size // Finally, the desired size after resume.
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

	readOnlyHandlers := []func(w http.ResponseWriter, r *http.Request) bool{
		regionsHandler,
		workspaceGroupsGetHandler,
		workspacesGetHandler,
	}

	writeHandlers := []func(w http.ResponseWriter, r *http.Request){
		workspaceGroupsPostHandler,
		workspacesPostHandler,
		workspacesSuspendHandler,
		workspacesResumeHandler,
		workspacesPatchHandler, // Retry.
		workspacesPatchHandler,
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
				Config: examples.WorkspacesResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", config.IDAttribute, workspace.WorkspaceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "workspace_group_id", workspace.WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "name", workspace.Name),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", workspace.Size),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "created_at", workspace.CreatedAt),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "endpoint", *workspace.Endpoint),
					resource.TestCheckNoResourceAttr("singlestoredb_workspace.this", "last_resumed_at"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(true)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", workspace.Size),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "true"),
					resource.TestCheckNoResourceAttr("singlestoredb_workspace.this", "endpoint"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(false)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", workspace.Size),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "endpoint", *newEndpoint),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(false)).
					WithWorkspaceResource("this")("size", cty.StringVal(updatedWorkspaceSize)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", updatedWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "endpoint", *newEndpoint),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}

func TestWorkspaceResourceIntegration(t *testing.T) {
	adminPassword := "fooBar1$"
	isConnectable := testutil.IsConnectableWithAdminPassword(adminPassword)

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "example",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "name", config.TestWorkspaceName),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", isConnectable),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(true)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "true"),
					resource.TestCheckNoResourceAttr("singlestoredb_workspace.this", "endpoint"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(false)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", isConnectable),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(false)).
					WithWorkspaceResource("this")("size", cty.StringVal(updatedWorkspaceSize)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", updatedWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", isConnectable),
				),
			},
		},
	})
}
