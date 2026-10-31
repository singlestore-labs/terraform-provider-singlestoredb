package privateconnections

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/stretchr/testify/require"
)

func TestReadRestoresWorkspaceGroupID(t *testing.T) {
	connectionID := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	groupID := uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")
	status := management.ClusterPrivateConnectionStatusACTIVE

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v2/privateConnections/" + connectionID.String():
			body, err := json.Marshal(management.ClusterPrivateConnection{
				PrivateConnectionID: connectionID,
				ClusterID:           &clusterID,
				Status:              &status,
			})
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)
		case "/v1/workspaces/" + clusterID.String():
			body, err := json.Marshal(management.Workspace{
				WorkspaceID:      clusterID,
				WorkspaceGroupID: groupID,
				State:            management.WorkspaceStateACTIVE,
			})
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	got, diags := readPrivateConnection(t, client, PrivateConnectionModel{
		ID: types.StringValue(connectionID.String()),
	})
	require.False(t, diags.HasError())
	require.Equal(t, groupID.String(), got.WorkspaceGroupID.ValueString())
	require.Equal(t, clusterID.String(), got.ClusterID.ValueString())
	require.Equal(t, clusterID.String(), got.WorkspaceID.ValueString())
}

func TestReadLeavesWorkspaceGroupIDUnsetWhenWorkspaceIsMissing(t *testing.T) {
	connectionID := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	status := management.ClusterPrivateConnectionStatusACTIVE

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v2/privateConnections/"+connectionID.String() {
			body, err := json.Marshal(management.ClusterPrivateConnection{
				PrivateConnectionID: connectionID,
				ClusterID:           &clusterID,
				Status:              &status,
			})
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)

			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	got, diags := readPrivateConnection(t, client, PrivateConnectionModel{
		ID: types.StringValue(connectionID.String()),
	})
	require.False(t, diags.HasError())
	require.True(t, got.WorkspaceGroupID.IsNull())
	require.Equal(t, clusterID.String(), got.ClusterID.ValueString())
}

func TestReadKeepsConfiguredWorkspaceGroupID(t *testing.T) {
	connectionID := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	clusterID := uuid.MustParse("283d4b0d-b0d6-485a-bc2d-a763c523c68a")
	groupID := uuid.MustParse("a4df90a6-e2b2-4de6-a50e-bd0a05aeaa09")
	status := management.ClusterPrivateConnectionStatusACTIVE

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path != "/v2/privateConnections/"+connectionID.String() {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)

			return
		}
		body, err := json.Marshal(management.ClusterPrivateConnection{
			PrivateConnectionID: connectionID,
			ClusterID:           &clusterID,
			Status:              &status,
		})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	got, diags := readPrivateConnection(t, client, PrivateConnectionModel{
		ID:               types.StringValue(connectionID.String()),
		WorkspaceGroupID: types.StringValue(groupID.String()),
	})
	require.False(t, diags.HasError())
	require.Equal(t, groupID.String(), got.WorkspaceGroupID.ValueString())
}

func readPrivateConnection(t *testing.T, client management.ClientWithResponsesInterface, model PrivateConnectionModel) (PrivateConnectionModel, diag.Diagnostics) {
	t.Helper()

	var schemaResp resource.SchemaResponse
	(&privateConnectionResource{}).Schema(t.Context(), resource.SchemaRequest{}, &schemaResp)

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(t.Context(), model)
	require.False(t, diags.HasError(), diags)

	resp := &resource.ReadResponse{State: state}
	(&privateConnectionResource{ClientWithResponsesInterface: client}).Read(t.Context(), resource.ReadRequest{State: state}, resp)

	var got PrivateConnectionModel
	if !resp.Diagnostics.HasError() && !resp.State.Raw.IsNull() {
		diags = resp.State.Get(t.Context(), &got)
		require.False(t, diags.HasError(), diags)
	}

	return got, resp.Diagnostics
}
