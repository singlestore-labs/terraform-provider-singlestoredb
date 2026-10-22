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

const (
	scheduledAfterSeconds float32 = 1200
	v2ClustersPath                = "/v2/clusters"
)

var unset = cty.Value{}

func TestReadsWorkspaceByID(t *testing.T) {
	workspace := management.Cluster{
		CreatedAt:     mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		Name:          "foo",
		State:         util.Ptr(management.ClusterStateACTIVE),
		ClusterID:     util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		GroupID:       util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		LastResumedAt: mustParseTimePtr("2023-03-14T17:28:32.430878Z"),
		Endpoint:      util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		SizeConfig:    &management.SizeConfig{Size: util.Ptr("S-00")},
		AutoSuspend: &management.AutoSuspend{
			SuspendType:           util.Ptr(management.SCHEDULED),
			ScheduledAfterSeconds: util.Ptr(int(scheduledAfterSeconds)),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v2/clusters/%s", workspace.ClusterID), r.URL.Path)
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(workspace))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal(util.Deref(workspace.ClusterID).String())).
					WithWorkspaceGetDataSource("this")("name", unset).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", config.IDAttribute, util.Deref(workspace.ClusterID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "workspace_group_id", util.Deref(workspace.GroupID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "name", workspace.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "state", string(util.Deref(workspace.State))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "size", util.Deref(workspace.SizeConfig.Size)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "created_at", workspace.CreatedAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "endpoint", *workspace.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "last_resumed_at", workspace.LastResumedAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "auto_suspend.suspend_type", "SCHEDULED"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "auto_suspend.suspend_after_seconds", fmt.Sprintf("%.0f", scheduledAfterSeconds)),
				),
			},
		},
	})
}

func TestWorkspaceNotFoundByID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithWorkspaceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)),
			},
		},
	})
}

func TestInvalidInputUUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.False(t, true, "should not get here")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal("invalid-uuid")).
					WithWorkspaceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestGetWorkspaceNotFoundByIDIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithWorkspaceGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)), // Checking that at least the expected error.
			},
		},
	})
}

func TestReadsWorkspaceByName(t *testing.T) {
	workspaceGroup1 := management.Cluster{
		GroupID: util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		Name:    "group1",
	}

	workspaceGroup2 := management.Cluster{
		GroupID: util.Ptr(uuid.MustParse("f2b1b970-8591-4196-bb26-f53f0f8e35ce")),
		Name:    "group2",
	}

	workspace1 := management.Cluster{
		CreatedAt:     mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		Name:          "target-workspace",
		State:         util.Ptr(management.ClusterStateACTIVE),
		ClusterID:     util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		GroupID:       workspaceGroup1.GroupID,
		LastResumedAt: mustParseTimePtr("2023-03-14T17:28:32.430878Z"),
		Endpoint:      util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		SizeConfig:    &management.SizeConfig{Size: util.Ptr("S-00")},
		AutoSuspend: &management.AutoSuspend{
			SuspendType:           util.Ptr(management.SCHEDULED),
			ScheduledAfterSeconds: util.Ptr(int(scheduledAfterSeconds)),
		},
	}

	workspace2 := management.Cluster{
		CreatedAt:  mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		Name:       "other-workspace",
		State:      util.Ptr(management.ClusterStateACTIVE),
		ClusterID:  util.Ptr(uuid.MustParse("a3c2d980-8591-4156-bb26-f53f0f8e35ce")),
		GroupID:    workspaceGroup1.GroupID,
		SizeConfig: &management.SizeConfig{Size: util.Ptr("S-1")},
	}

	allClusters := []management.Cluster{workspaceGroup1, workspaceGroup2, workspace1, workspace2}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		require.Equal(t, v2ClustersPath, r.URL.Path)
		_, err := w.Write(testutil.MustJSON(allClusters))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")("name", cty.StringVal(workspace1.Name)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", config.IDAttribute, util.Deref(workspace1.ClusterID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "workspace_group_id", util.Deref(workspace1.GroupID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "name", workspace1.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "state", string(util.Deref(workspace1.State))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "size", util.Deref(workspace1.SizeConfig.Size)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "created_at", workspace1.CreatedAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "endpoint", *workspace1.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "last_resumed_at", workspace1.LastResumedAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "auto_suspend.suspend_type", "SCHEDULED"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "auto_suspend.suspend_after_seconds", fmt.Sprintf("%.0f", scheduledAfterSeconds)),
				),
			},
		},
	})
}

func TestWorkspaceNotFoundByName(t *testing.T) {
	workspaceGroup := management.Cluster{
		GroupID: util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		Name:    "group1",
	}

	workspace := management.Cluster{
		Name:      "existing-workspace",
		ClusterID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		GroupID:   workspaceGroup.GroupID,
	}

	allClusters := []management.Cluster{workspaceGroup, workspace}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		require.Equal(t, v2ClustersPath, r.URL.Path)
		_, err := w.Write(testutil.MustJSON(allClusters))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")("name", cty.StringVal("non-existent-workspace")).
					String(),
				ExpectError: regexp.MustCompile("No workspace with the name 'non-existent-workspace' was found"),
			},
		},
	})
}

func TestMultipleWorkspacesWithSameName(t *testing.T) {
	workspaceGroup1 := management.Cluster{
		GroupID: util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		Name:    "group1",
	}

	workspaceGroup2 := management.Cluster{
		GroupID: util.Ptr(uuid.MustParse("f2b1b970-8591-4196-bb26-f53f0f8e35ce")),
		Name:    "group2",
	}

	workspace1 := management.Cluster{
		Name:      "duplicate-name",
		ClusterID: util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		GroupID:   workspaceGroup1.GroupID,
	}

	workspace2 := management.Cluster{
		Name:      "duplicate-name",
		ClusterID: util.Ptr(uuid.MustParse("a3c2d980-8591-4156-bb26-f53f0f8e35ce")),
		GroupID:   workspaceGroup2.GroupID,
	}

	allClusters := []management.Cluster{workspaceGroup1, workspaceGroup2, workspace1, workspace2}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		require.Equal(t, v2ClustersPath, r.URL.Path)
		_, err := w.Write(testutil.MustJSON(allClusters))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")("name", cty.StringVal("duplicate-name")).
					String(),
				ExpectError: regexp.MustCompile("Multiple workspaces with the name 'duplicate-name' were found"),
			},
		},
	})
}

func TestValidationErrorsForConflictingIdentifiersWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.False(t, true, "should not get here")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")("name", cty.StringVal("duplicate-name")).
					WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile("Only one of 'id' or 'name' can be specified, not both"),
			},
		},
	})
}

func TestValidationErrorsForMissingIdentifiersWorkspace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.False(t, true, "should not get here")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")("name", unset).
					WithWorkspaceGetDataSource("this")(config.IDAttribute, unset).
					String(),
				ExpectError: regexp.MustCompile("Either 'id' or 'name' must be specified"),
			},
		},
	})
}

func TestCaseInsensitiveNameMatchingWorkspace(t *testing.T) {
	workspaceGroup := management.Cluster{
		GroupID: util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		Name:    "group1",
	}

	workspace := management.Cluster{
		CreatedAt:  mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		Name:       "Test-Workspace-Name",
		State:      util.Ptr(management.ClusterStateACTIVE),
		ClusterID:  util.Ptr(uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")),
		GroupID:    workspaceGroup.GroupID,
		SizeConfig: &management.SizeConfig{Size: util.Ptr("S-00")},
	}

	allClusters := []management.Cluster{workspaceGroup, workspace}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		require.Equal(t, v2ClustersPath, r.URL.Path)
		_, err := w.Write(testutil.MustJSON(allClusters))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")("name", cty.StringVal("  test-workspace-name  ")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", config.IDAttribute, util.Deref(workspace.ClusterID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "name", workspace.Name),
				),
			},
		},
	})
}

func TestGetWorkspaceByNameNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")("name", cty.StringVal("non-existent-workspace-name-for-testing")).
					String(),
				ExpectError: regexp.MustCompile("No workspace"),
			},
		},
	})
}
