package workspaces_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
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

func TestReadsWorkspaces(t *testing.T) {
	workspaceGroups := []management.Cluster{
		{
			AllowAllTraffic: nil,
			CreatedAt:       mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
			ExpiresAt:       nil,
			FirewallRanges:  util.Ptr([]string{"127.0.0.1/32"}),
			Name:            "foo",
			State:           util.Ptr(management.ClusterStateACTIVE),
			TerminatedAt:    nil,
			UpdateWindow: &management.UpdateWindow{
				Day:  3,
				Hour: 15,
			},
			GroupID: util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		},
	}

	workspaces := []management.Cluster{
		{
			CreatedAt:     mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
			Name:          "foo",
			State:         util.Ptr(management.ClusterStateACTIVE),
			ClusterID:     util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
			GroupID:       util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
			LastResumedAt: mustParseTimePtr("2023-03-14T17:28:32.430878Z"),
			Endpoint:      util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
			SizeConfig:    &management.SizeConfig{Size: util.Ptr("S-00")},
		},
		{
			CreatedAt:     mustParseTimePtr("2023-03-01T05:33:06.3003Z"),
			Name:          "bar",
			State:         util.Ptr(management.ClusterStateSUSPENDED),
			ClusterID:     util.Ptr(uuid.MustParse("f3a1a960-8591-4156-bb26-f53f0f8e35ce")),
			GroupID:       util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
			LastResumedAt: nil,
			Endpoint:      nil,
			SizeConfig:    &management.SizeConfig{Size: util.Ptr("S-1")},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v2/clusters", r.URL.Path)
		require.Equal(t, http.MethodGet, r.Method)
		// Filtering by group is done client-side; return clusters in the group.
		w.Header().Add("Content-Type", "json")
		_, err := w.Write(testutil.MustJSON(workspaces))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesListDataSource).
					WithWorkspaceListDataSource("all")(config.WorkspaceGroupIDAttribute, cty.StringVal(util.Deref(workspaceGroups[0].GroupID).String())).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", fmt.Sprintf("workspaces.0.%s", config.IDAttribute), util.Deref(workspaces[0].ClusterID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.workspace_group_id", util.Deref(workspaces[0].GroupID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.name", workspaces[0].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.state", string(util.Deref(workspaces[0].State))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.size", util.Deref(workspaces[0].SizeConfig.Size)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.suspended", "false"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.created_at", workspaces[0].CreatedAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.endpoint", *workspaces[0].Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.0.last_resumed_at", workspaces[0].LastResumedAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", fmt.Sprintf("workspaces.1.%s", config.IDAttribute), util.Deref(workspaces[1].ClusterID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.workspace_group_id", util.Deref(workspaces[1].GroupID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.name", workspaces[1].Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.state", string(util.Deref(workspaces[1].State))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.size", util.Deref(workspaces[1].SizeConfig.Size)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.suspended", "true"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.created_at", workspaces[1].CreatedAt.Format(time.RFC3339)),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.endpoint"),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspaces.all", "workspaces.1.last_resumed_at"),
				),
			},
		},
	})
}

func TestReadWorkspaceGroupsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.WorkspacesListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestListWorkspacesWorkspaceGroupNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesListDataSource).
					WithWorkspaceListDataSource("all")(config.WorkspaceGroupIDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)), // Checking that at least the expected error.
			},
		},
	})
}
