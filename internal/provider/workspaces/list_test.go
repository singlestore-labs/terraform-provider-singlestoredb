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
	"github.com/singlestore-labs/terraform-provider-singlestore/examples"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/workspaces"
	"github.com/stretchr/testify/require"
)

func TestReadsWorkspaces(t *testing.T) {
	workspaceGroups := []management.WorkspaceGroup{
		{
			AllowAllTraffic: nil,
			CreatedAt:       "2023-02-28T05:33:06.3003Z",
			ExpiresAt:       nil,
			FirewallRanges:  util.Ptr([]string{"127.0.0.1/32"}),
			Name:            "foo",
			RegionID:        uuid.MustParse("0aa1aff3-4092-4a0c-bf36-da54e85a4fdf"),
			State:           management.WorkspaceGroupStateACTIVE,
			TerminatedAt:    nil,
			UpdateWindow: &management.UpdateWindow{
				Day:  3,
				Hour: 15,
			},
			WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
		},
	}

	mustSize := func(ws management.Workspace) string {
		result, err := workspaces.ParseSize(ws.Size, ws.State)
		require.Nil(t, err)

		return result.String()
	}

	workspaces := []management.Workspace{
		{
			CreatedAt:        "2023-02-28T05:33:06.3003Z",
			Name:             "foo",
			State:            management.WorkspaceStateACTIVE,
			WorkspaceID:      uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
			WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
			LastResumedAt:    util.Ptr("2023-03-14T17:28:32.430878Z"),
			Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
			Size:             "S-00",
		},
		{
			CreatedAt:        "2023-02-29T05:33:06.3003Z",
			Name:             "bar",
			State:            management.WorkspaceStateSUSPENDED,
			WorkspaceID:      uuid.MustParse("f3a1a960-8591-4156-bb26-f53f0f8e35ce"),
			WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
			LastResumedAt:    nil,
			Endpoint:         nil,
			Size:             "S-1",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaces", r.URL.Path)
		workspaceGroupID := r.URL.Query().Get("workspaceGroupID") // Terraform workspace_group_id translates to the query parameter ?workspaceGroupID.
		require.Equal(t, workspaceGroups[0].WorkspaceGroupID.String(), workspaceGroupID,
			"workspace_group_id is mandatory for listing workspaces",
		)

		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(workspaces))
		require.NoError(t, err)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesListDataSource).
					WithOverride(config.TestInitialWorkspaceGroupID, workspaceGroups[0].WorkspaceGroupID.String()).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.#", "2"),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", fmt.Sprintf("workspaces.0.%s", config.IDAttribute), workspaces[0].WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.0.workspace_group_id", workspaces[0].WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.0.name", workspaces[0].Name),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.0.state", string(workspaces[0].State)),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.0.size", mustSize(workspaces[0])),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.0.created_at", workspaces[0].CreatedAt),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.0.endpoint", *workspaces[0].Endpoint),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.0.last_resumed_at", *workspaces[0].LastResumedAt),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", fmt.Sprintf("workspaces.1.%s", config.IDAttribute), workspaces[1].WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.1.workspace_group_id", workspaces[1].WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.1.name", workspaces[1].Name),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.1.state", string(workspaces[1].State)),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.1.size", mustSize(workspaces[1])),
					resource.TestCheckResourceAttr("data.singlestore_workspaces.all", "workspaces.1.created_at", workspaces[1].CreatedAt),
					resource.TestCheckNoResourceAttr("data.singlestore_workspaces.all", "workspaces.1.endpoint"),
					resource.TestCheckNoResourceAttr("data.singlestore_workspaces.all", "workspaces.1.last_resumed_at"),
				),
			},
		},
	})
}

func TestReadWorkspaceGroupsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	r := regexp.MustCompile(http.StatusText(http.StatusUnauthorized))

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.WorkspacesListDataSource,
				ExpectError: r,
			},
		},
	})
}

func TestListWorkspacesWorkspaceGroupNotFoundIntegration(t *testing.T) {
	apiKey := os.Getenv(config.EnvTestAPIKey)

	r := regexp.MustCompile(http.StatusText(http.StatusNotFound))

	testutil.IntegrationTest(t, apiKey, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithOverride(config.TestInitialWorkspaceGroupID, uuid.New().String()).
					String(),
				ExpectError: r, // Checking that at least the expected error.
			},
		},
	})
}
