package roles

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/stretchr/testify/require"
)

func TestResolveClusterAccessTargetUsesClusterAPI(t *testing.T) {
	clusterID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/clusters/"+clusterID.String(), r.URL.Path)
		writeJSON(t, w, http.StatusOK, management.Cluster{Name: "cluster"})
	})

	target, err := resolveClusterAccessTarget(t.Context(), client, clusterID)
	require.NoError(t, err)
	require.Equal(t, clusterAccessEndpointV2Cluster, target.endpoint)
	require.Equal(t, clusterID, target.id)
}

func TestResolveClusterAccessTargetUsesWorkspaceGroupAPI(t *testing.T) {
	groupID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/clusters/" + groupID.String():
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
		case "/v1/workspaceGroups/" + groupID.String():
			writeJSON(t, w, http.StatusOK, management.WorkspaceGroup{Name: "group"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	target, err := resolveClusterAccessTarget(t.Context(), client, groupID)
	require.NoError(t, err)
	require.Equal(t, clusterAccessEndpointV1WorkspaceGroup, target.endpoint)
	require.Equal(t, groupID, target.id)
}

func TestResolveClusterAccessTargetMapsParentGroupToCluster(t *testing.T) {
	groupID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	clusterID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/clusters/" + groupID.String(), "/v1/workspaceGroups/" + groupID.String():
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
		case "/v2/clusters":
			writeJSON(t, w, http.StatusOK, []management.Cluster{{
				Name:      "child",
				ClusterID: &clusterID,
				GroupID:   &groupID,
			}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	target, err := resolveClusterAccessTarget(t.Context(), client, groupID)
	require.NoError(t, err)
	require.Equal(t, clusterAccessEndpointV2Cluster, target.endpoint)
	require.Equal(t, clusterID, target.id)
}

func TestResolveClusterAccessTargetNotFound(t *testing.T) {
	resourceID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/clusters/" + resourceID.String(), "/v1/workspaceGroups/" + resourceID.String():
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
		case "/v2/clusters":
			writeJSON(t, w, http.StatusOK, []management.Cluster{})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	_, err := resolveClusterAccessTarget(t.Context(), client, resourceID)
	require.Error(t, err)
	require.Contains(t, err.Error(), resourceID.String())
}

func TestApplyWorkspaceGroupAccessControlsChoosesEndpoint(t *testing.T) {
	groupID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
	clusterID := uuid.MustParse("77777777-7777-7777-7777-777777777777")

	groupClient := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/clusters/" + groupID.String():
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
		case "/v1/workspaceGroups/" + groupID.String():
			writeJSON(t, w, http.StatusOK, management.WorkspaceGroup{Name: "group"})
		case "/v1/workspaceGroups/" + groupID.String() + "/accessControls":
			require.Equal(t, http.MethodPatch, r.Method)
			writeJSON(t, w, http.StatusOK, map[string]string{})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	ok, err := applyWorkspaceGroupAccessControls(t.Context(), groupClient, groupID, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)

	clusterClient := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/clusters/" + clusterID.String():
			if r.Method == http.MethodGet {
				writeJSON(t, w, http.StatusOK, management.Cluster{Name: "cluster"})

				return
			}
			t.Fatalf("unexpected method %s", r.Method)
		case "/v2/clusters/" + clusterID.String() + "/accessControls":
			require.Equal(t, http.MethodPatch, r.Method)
			writeJSON(t, w, http.StatusOK, map[string]string{})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	ok, err = applyWorkspaceGroupAccessControls(t.Context(), clusterClient, clusterID, nil, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestGetWorkspaceGroupRolesUsesWorkspaceGroupAPI(t *testing.T) {
	groupID := uuid.MustParse("88888888-8888-8888-8888-888888888888")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/clusters/" + groupID.String():
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
		case "/v1/workspaceGroups/" + groupID.String():
			writeJSON(t, w, http.StatusOK, management.WorkspaceGroup{Name: "group"})
		case "/v1/workspaceGroups/" + groupID.String() + "/accessControls":
			writeJSON(t, w, http.StatusOK, []management.ResourceRole{{Role: "Owner"}})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	roles, err := (&rolesDataSourceList{ClientWithResponsesInterface: client}).getWorkspaceGroupRoles(t.Context(), groupID)
	require.NoError(t, err)
	require.Equal(t, []management.ResourceRole{{Role: "Owner"}}, *roles)
}

func accessControlClient(t *testing.T, handler http.HandlerFunc) management.ClientWithResponsesInterface {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	require.NoError(t, json.NewEncoder(w).Encode(body))
}
