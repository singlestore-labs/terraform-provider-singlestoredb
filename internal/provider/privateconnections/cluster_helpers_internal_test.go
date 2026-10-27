package privateconnections

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/stretchr/testify/require"
)

func TestResolveClusterIDPrefersExplicitIDs(t *testing.T) {
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	workspaceID := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")

	got, err := resolveClusterID(t.Context(), nil, PrivateConnectionModel{
		ClusterID:   types.StringValue(clusterID.String()),
		WorkspaceID: types.StringValue(workspaceID.String()),
	})
	require.Nil(t, err)
	require.Equal(t, clusterID, got)

	got, err = resolveClusterID(t.Context(), nil, PrivateConnectionModel{
		WorkspaceID: types.StringValue(workspaceID.String()),
	})
	require.Nil(t, err)
	require.Equal(t, workspaceID, got)

	_, err = resolveClusterID(t.Context(), nil, PrivateConnectionModel{})
	require.NotNil(t, err)
	require.Equal(t, "Missing cluster identifier", err.Summary)
}

func TestResolveClusterIDFromWorkspaceGroup(t *testing.T) {
	groupID := uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	otherGroup := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v1/workspaces" {
			w.WriteHeader(http.StatusNotFound)

			return
		}
		body, err := json.Marshal([]management.Workspace{
			{WorkspaceGroupID: otherGroup, WorkspaceID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), State: management.WorkspaceStateACTIVE},
			{WorkspaceGroupID: groupID, WorkspaceID: clusterID, State: management.WorkspaceStateACTIVE},
		})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	got, serr := resolveClusterID(t.Context(), client, PrivateConnectionModel{
		WorkspaceGroupID: types.StringValue(groupID.String()),
	})
	require.Nil(t, serr)
	require.Equal(t, clusterID, got)
}

func TestResolveClusterIDGroupLookupFailures(t *testing.T) {
	groupID := uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")
	plan := PrivateConnectionModel{WorkspaceGroupID: types.StringValue(groupID.String())}

	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(errorServer.Close)
	errorClient, err := management.NewClientWithResponses(errorServer.URL)
	require.NoError(t, err)

	_, serr := resolveClusterID(t.Context(), errorClient, plan)
	require.NotNil(t, serr)

	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, writeErr := w.Write([]byte("[]"))
		require.NoError(t, writeErr)
	}))
	t.Cleanup(emptyServer.Close)
	emptyClient, err := management.NewClientWithResponses(emptyServer.URL)
	require.NoError(t, err)

	_, serr = resolveClusterID(t.Context(), emptyClient, plan)
	require.NotNil(t, serr)
	require.Equal(t, "No workspace found in workspace group", serr.Summary)
}

func TestResolveClusterIDAmbiguousWorkspaceGroup(t *testing.T) {
	groupID := uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal([]management.Workspace{
			{WorkspaceGroupID: groupID, WorkspaceID: uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a"), State: management.WorkspaceStateACTIVE},
			{WorkspaceGroupID: groupID, WorkspaceID: uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"), State: management.WorkspaceStateSUSPENDED},
			{WorkspaceGroupID: groupID, WorkspaceID: uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc"), State: management.WorkspaceStateTERMINATED},
		})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	_, serr := resolveClusterID(t.Context(), client, PrivateConnectionModel{
		WorkspaceGroupID: types.StringValue(groupID.String()),
	})
	require.NotNil(t, serr)
	require.Equal(t, "Ambiguous workspace group", serr.Summary)
}
