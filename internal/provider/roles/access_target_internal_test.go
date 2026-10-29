package roles

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

const v2ClustersPath = "/v2/clusters"

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
		case v2ClustersPath:
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
		case v2ClustersPath:
			writeJSON(t, w, http.StatusOK, []management.Cluster{})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	_, err := resolveClusterAccessTarget(t.Context(), client, resourceID)
	require.Error(t, err)
	require.Contains(t, err.Error(), resourceID.String())
}

func TestResolveClusterAccessTargetReportsAPIError(t *testing.T) {
	resourceID := uuid.MustParse("99999999-9999-9999-9999-999999999999")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/clusters/"+resourceID.String(), r.URL.Path)
		writeJSON(t, w, http.StatusInternalServerError, map[string]string{"message": "boom"})
	})

	_, err := resolveClusterAccessTarget(t.Context(), client, resourceID)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "No cluster or workspace group was found")
}

func TestPickConfiguredClusterResourceIDPrefersClusterID(t *testing.T) {
	groupID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	clusterID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	id, ok := pickConfiguredClusterResourceID([]uuid.UUID{groupID, clusterID}, clusterID, groupID)
	require.True(t, ok)
	require.Equal(t, clusterID, id)

	id, ok = pickConfiguredClusterResourceID([]uuid.UUID{groupID}, clusterID, groupID)
	require.True(t, ok)
	require.Equal(t, groupID, id)
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

func TestAlignClusterRoleResourceIDsKeepsAPIClusterID(t *testing.T) {
	clusterID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	groupID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	otherID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, v2ClustersPath, r.URL.Path)
		writeJSON(t, w, http.StatusOK, []management.Cluster{{
			Name:      "child",
			ClusterID: &clusterID,
			GroupID:   &groupID,
		}})
	})

	apiRole := clusterGrant(clusterID.String())
	organizationRole := RoleAttributesModel{
		RoleName:     types.StringValue("Reader"),
		ResourceType: types.StringValue(string(ResourceTypeOrganization)),
		ResourceID:   types.StringValue(otherID.String()),
	}
	invalidID := clusterGrant("not-a-uuid")
	got, err := alignClusterRoleResourceIDs(t.Context(), client, []RoleAttributesModel{apiRole, organizationRole, invalidID}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, clusterID.String(), got[0].ResourceID.ValueString())
	require.Equal(t, otherID.String(), got[1].ResourceID.ValueString())
	require.Equal(t, "not-a-uuid", got[2].ResourceID.ValueString())

	configuredCluster := []RoleAttributesModel{clusterGrant(clusterID.String())}
	got, err = alignClusterRoleResourceIDs(t.Context(), client, []RoleAttributesModel{apiRole}, nil, &configuredCluster)
	require.NoError(t, err)
	require.Equal(t, clusterID.String(), got[0].ResourceID.ValueString())

	unrelated := []RoleAttributesModel{clusterGrant(otherID.String())}
	got, err = alignClusterRoleResourceIDs(t.Context(), client, []RoleAttributesModel{apiRole}, &unrelated, nil)
	require.NoError(t, err)
	require.Equal(t, clusterID.String(), got[0].ResourceID.ValueString())

	groupRole := clusterGrant(groupID.String())
	got, err = alignClusterRoleResourceIDs(t.Context(), client, []RoleAttributesModel{groupRole}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, groupID.String(), got[0].ResourceID.ValueString())
}

func TestAlignClusterRoleResourceIDsKeepsWorkspaceGroupScope(t *testing.T) {
	clusterID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	groupID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case v2ClustersPath:
			writeJSON(t, w, http.StatusOK, []management.Cluster{{
				Name:      "child",
				ClusterID: &clusterID,
				GroupID:   &groupID,
			}})
		case "/v1/workspaceGroups/" + groupID.String():
			writeJSON(t, w, http.StatusOK, management.WorkspaceGroup{Name: "group"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	configuredGroup := []RoleAttributesModel{clusterGrant(groupID.String())}
	got, err := alignClusterRoleResourceIDs(t.Context(), client, []RoleAttributesModel{clusterGrant(clusterID.String())}, &configuredGroup, nil)
	require.NoError(t, err)
	require.Equal(t, clusterID.String(), got[0].ResourceID.ValueString())
}

func TestAlignClusterRoleResourceIDsAlignsParentGroupAlias(t *testing.T) {
	clusterID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	groupID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	client := accessControlClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case v2ClustersPath:
			writeJSON(t, w, http.StatusOK, []management.Cluster{{
				Name:      "child",
				ClusterID: &clusterID,
				GroupID:   &groupID,
			}})
		case "/v1/workspaceGroups/" + groupID.String():
			writeJSON(t, w, http.StatusNotFound, map[string]string{"message": "not found"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})

	configuredGroup := []RoleAttributesModel{clusterGrant(groupID.String())}
	got, err := alignClusterRoleResourceIDs(t.Context(), client, []RoleAttributesModel{clusterGrant(clusterID.String())}, &configuredGroup, nil)
	require.NoError(t, err)
	require.Equal(t, groupID.String(), got[0].ResourceID.ValueString())
}

func TestAlignClusterRoleResourceIDsReportsAPIError(t *testing.T) {
	client := accessControlClient(t, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, http.StatusInternalServerError, map[string]string{"message": "boom"})
	})

	configured := []RoleAttributesModel{clusterGrant("44444444-4444-4444-4444-444444444444")}
	_, err := alignClusterRoleResourceIDs(t.Context(), client, []RoleAttributesModel{
		clusterGrant("44444444-4444-4444-4444-444444444444"),
	}, &configured, nil)
	require.Error(t, err)
}

func TestAlignClusterRoleResourceIDsSkipsNonClusterGrants(t *testing.T) {
	role := RoleAttributesModel{
		RoleName:     types.StringValue("Owner"),
		ResourceType: types.StringValue(string(ResourceTypeTeam)),
		ResourceID:   types.StringValue("not-a-uuid"),
	}

	got, err := alignClusterRoleResourceIDs(t.Context(), nil, []RoleAttributesModel{role}, nil, nil)
	require.NoError(t, err)
	require.Equal(t, role, got[0])
}

func TestRoleWithPlannedSpelling(t *testing.T) {
	state := RoleAttributesModel{
		RoleName:     types.StringValue("Owner"),
		ResourceType: types.StringValue("Cluster"),
		ResourceID:   types.StringValue("resource"),
	}
	plan := state
	plan.RoleName = types.StringValue("owner")
	require.False(t, IsRoleChanged(plan, state))

	next, ok := roleWithPlannedSpelling(plan, state)
	require.True(t, ok)
	require.Equal(t, "owner", next.RoleName.ValueString())
	require.Equal(t, state.ResourceID, next.ResourceID)

	plan = state
	plan.ResourceType = types.StringValue("cluster")
	next, ok = roleWithPlannedSpelling(plan, state)
	require.True(t, ok)
	require.Equal(t, "cluster", next.ResourceType.ValueString())

	next, ok = roleWithPlannedSpelling(state, state)
	require.False(t, ok)
	require.Equal(t, state, next)
}

func clusterGrant(id string) RoleAttributesModel {
	return RoleAttributesModel{
		RoleName:     types.StringValue("Owner"),
		ResourceType: types.StringValue(string(ResourceTypeWorkspaceGroup)),
		ResourceID:   types.StringValue(id),
	}
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
