package workspaces_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/examples"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/workspaces"
	"github.com/stretchr/testify/require"
)

var (
	updatedWorkspaceSize = "0.5"                                                                                     //nolint
	newEndpoint          = util.Ptr("svc-14a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com") //nolint
)

func TestCRUDWorkspace(t *testing.T) { //nolint:cyclop,maintidx
	mustSize := func(value string) string {
		result, err := workspaces.ParseSize(value, management.WorkspaceStateACTIVE)
		require.Nil(t, err)

		return result.String()
	}

	regions := []management.Region{
		{
			RegionID: uuid.MustParse("2ca3d358-021d-45ed-86cb-38b8d14ac507"),
			Region:   "GS - US West 2 (Oregon) - aws-oregon-gs1",
			Provider: management.AWS,
		},
	}

	workspaceGroupCreateResponse := struct {
		WorkspaceGroupID uuid.UUID
	}{
		WorkspaceGroupID: uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507"),
	}

	workspaceGroup := management.WorkspaceGroup{
		AllowAllTraffic:  util.Ptr(false),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   nil,
		Name:             config.TestInitialWorkspaceGroupName,
		RegionID:         regions[0].RegionID,
		State:            management.WorkspaceGroupStateACTIVE,
		TerminatedAt:     nil,
		UpdateWindow:     nil,
		WorkspaceGroupID: workspaceGroupCreateResponse.WorkspaceGroupID,
	}

	workspaceCreateResponse := struct {
		WorkspaceID uuid.UUID
	}{
		WorkspaceID: uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
	}

	mustDecimalSizeToSFormatSize := func(s string) string {
		if s == "0.25" {
			return "S-00"
		}

		if s == "0.5" {
			return "S-0"
		}

		require.False(t, true, "implement conversion from the decimal size %s to the S-format size for the test", s)

		return ""
	}

	workspace := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             config.TestInitialWorkspaceName,
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      workspaceCreateResponse.WorkspaceID,
		WorkspaceGroupID: workspaceGroup.WorkspaceGroupID,
		LastResumedAt:    nil,
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		Size:             mustDecimalSizeToSFormatSize(config.TestInitialWorkspaceSize),
	}

	workspaceSuspendResponse := struct {
		WorkspaceID uuid.UUID
	}{
		WorkspaceID: workspace.WorkspaceID,
	}

	workspaceResumeResponse := struct {
		WorkspaceID uuid.UUID
	}{
		WorkspaceID: workspace.WorkspaceID,
	}

	workspacePatchResponse := struct {
		WorkspaceID uuid.UUID
	}{
		WorkspaceID: workspace.WorkspaceID,
	}

	workspaceGroupTerminateResponse := struct {
		WorkspaceGroupID uuid.UUID
	}{
		WorkspaceGroupID: workspaceGroup.WorkspaceGroupID,
	}

	workspaceTerminateResponse := struct {
		WorkspaceGroupID uuid.UUID
	}{
		WorkspaceGroupID: workspace.WorkspaceID,
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
		_, err := w.Write(testutil.MustJSON(workspaceGroupCreateResponse))
		require.NoError(t, err)
	}

	workspacesPostHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspaceCreateResponse))
		require.NoError(t, err)
	}

	workspaceGroupsGetHandler := func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path != strings.Join([]string{"/v1/workspaceGroups", workspaceGroupCreateResponse.WorkspaceGroupID.String()}, "/") ||
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
		if r.URL.Path != strings.Join([]string{"/v1/workspaces", workspaceCreateResponse.WorkspaceID.String()}, "/") ||
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
		_, err := w.Write(testutil.MustJSON(workspaceSuspendResponse))
		require.NoError(t, err)

		workspace.State = management.WorkspaceStateSUSPENDED
		workspace.Endpoint = nil
	}

	workspacesResumeHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaces", workspace.WorkspaceID.String(), "resume"}, "/"), r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspaceResumeResponse))
		require.NoError(t, err)

		workspace.State = management.WorkspaceStateACTIVE
		workspace.Endpoint = util.Ptr("svc-14a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com") // New endpoint.
		// But after resume, size is still the old size. A scale should be perform right after resuming.
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
		require.Equal(t, updatedWorkspaceSize, mustSize(util.Deref(input.Size)))

		w.Header().Add("Content-Type", "json")
		_, err = w.Write(testutil.MustJSON(workspacePatchResponse))
		require.NoError(t, err)
		workspace.Size = *input.Size // Finally, the desired size after resume.
	}

	workspaceGroupsDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupCreateResponse.WorkspaceGroupID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspaceGroupTerminateResponse))
		require.NoError(t, err)
	}

	workspacesDeleteHandler := func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, strings.Join([]string{"/v1/workspaces", workspaceCreateResponse.WorkspaceID.String()}, "/"), r.URL.Path)
		require.Equal(t, http.MethodDelete, r.Method)

		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspaceTerminateResponse))
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
	defer server.Close()

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspacesResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestore_workspace.example", config.IDAttribute, workspace.WorkspaceID.String()),
					resource.TestCheckResourceAttr("singlestore_workspace.example", "workspace_group_id", workspace.WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestore_workspace.example", "name", workspace.Name),
					resource.TestCheckResourceAttr("singlestore_workspace.example", "size", mustSize(workspace.Size)),
					resource.TestCheckResourceAttr("singlestore_workspace.example", "created_at", workspace.CreatedAt),
					resource.TestCheckResourceAttr("singlestore_workspace.example", "endpoint", *workspace.Endpoint),
					resource.TestCheckNoResourceAttr("singlestore_workspace.example", "last_resumed_at"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithOverride(config.TestInitialWorkspaceSize, config.WorkspaceSizeSuspended).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestore_workspace.example", "size", config.WorkspaceSizeSuspended),
					resource.TestCheckNoResourceAttr("singlestore_workspace.example", "endpoint"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithOverride(config.TestInitialWorkspaceSize, updatedWorkspaceSize).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestore_workspace.example", "size", updatedWorkspaceSize),
					resource.TestCheckResourceAttr("singlestore_workspace.example", "endpoint", *newEndpoint),
				),
			},
		},
	})

	require.Empty(t, writeHandlers, "all the mutating REST calls should have been called, but %d is left not called yet", len(writeHandlers))
}
