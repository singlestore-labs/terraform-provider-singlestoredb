package workspaces_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestReadsWorkspace(t *testing.T) {
	workspace := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             "foo",
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
		LastResumedAt:    util.Ptr("2023-03-14T17:28:32.430878Z"),
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		Size:             "S-00",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/workspaces/%s", workspace.WorkspaceID), r.URL.Path)
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(workspace))
		require.NoError(t, err)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithOverride(config.TestInitialWorkspaceID, workspace.WorkspaceID.String()).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", config.IDAttribute, workspace.WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", "workspace_group_id", workspace.WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", "name", workspace.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", "state", string(workspace.State)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", "size", workspace.Size),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", "created_at", workspace.CreatedAt),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", "endpoint", *workspace.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.example", "last_resumed_at", *workspace.LastResumedAt),
				),
			},
		},
	})
}

func TestWorkspaceNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	r := regexp.MustCompile(http.StatusText(http.StatusNotFound))

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithOverride(config.TestInitialWorkspaceID, uuid.New().String()).
					String(),
				ExpectError: r,
			},
		},
	})
}

func TestInvalidInputUUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.False(t, true, "should not get here")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	r := regexp.MustCompile("invalid UUID")

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithOverride(config.TestInitialWorkspaceID, "invalid-uuid").
					String(),
				ExpectError: r,
			},
		},
	})
}

func TestGetWorkspaceNotFoundIntegration(t *testing.T) {
	apiKey := os.Getenv(config.EnvTestAPIKey)

	r := regexp.MustCompile(http.StatusText(http.StatusNotFound))

	testutil.IntegrationTest(t, apiKey, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithOverride(config.TestInitialWorkspaceID, uuid.New().String()).
					String(),
				ExpectError: r, // Checking that at least the expected error.
			},
		},
	})
}
