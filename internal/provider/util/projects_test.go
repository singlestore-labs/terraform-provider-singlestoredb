package util_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestResolveProjectIDByName(t *testing.T) {
	projectID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	otherID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	_, err := util.ResolveProjectIDByName(t.Context(), projectClient(t, http.StatusOK, []management.Project{
		{Name: "Other", ProjectID: otherID},
	}), "Standard Project", "cluster")
	require.NotNil(t, err)
	require.Equal(t, "Project not found", err.Summary)
	require.Contains(t, err.Detail, "Other")

	_, err = util.ResolveProjectIDByName(t.Context(), projectClient(t, http.StatusOK, []management.Project{}), "Standard Project", "cluster")
	require.Equal(t, "Project not found", err.Summary)
	require.NotContains(t, err.Detail, "Available projects")

	_, err = util.ResolveProjectIDByName(t.Context(), projectClient(t, http.StatusOK, []management.Project{
		{Name: "Standard Project", ProjectID: projectID},
		{Name: "Standard Project", ProjectID: otherID},
	}), "Standard Project", "cluster")
	require.Equal(t, "Multiple projects found", err.Summary)
	require.Contains(t, err.Detail, "cluster")

	got, err := util.ResolveProjectIDByName(t.Context(), projectClient(t, http.StatusOK, []management.Project{
		{Name: "Standard Project", ProjectID: projectID},
	}), "Standard Project", "cluster")
	require.Nil(t, err)
	require.Equal(t, projectID, *got)

	_, err = util.ResolveProjectIDByName(t.Context(), projectClient(t, http.StatusInternalServerError, nil), "Standard Project", "cluster")
	require.NotNil(t, err)
}

func projectClient(t *testing.T, status int, projects []management.Project) *management.ClientWithResponses {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if projects == nil {
			return
		}

		body, err := json.Marshal(projects)
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client, err := management.NewClientWithResponses(server.URL)
	require.NoError(t, err)

	return client
}
