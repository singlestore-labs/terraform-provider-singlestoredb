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
	"github.com/zclconf/go-cty/cty"
)

const (
	scheduledAfterSeconds float32 = 1200
	v1WorkspaceGroups             = "/v1/workspaceGroups"
	v1Workspaces                  = "/v1/workspaces"
)

var unset = cty.Value{}

func TestReadsWorkspaceByID(t *testing.T) {
	workspace := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             "foo",
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
		LastResumedAt:    util.Ptr("2023-03-14T17:28:32.430878Z"),
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		Size:             "S-00",
		AutoSuspend: &struct {
			IdleAfterSeconds      *float32                                   `json:"idleAfterSeconds,omitempty"`
			IdleChangedAt         *string                                    `json:"idleChangedAt,omitempty"`
			ScheduledAfterSeconds *float32                                   `json:"scheduledAfterSeconds,omitempty"`
			ScheduledChangedAt    *string                                    `json:"scheduledChangedAt,omitempty"`
			ScheduledSuspendAt    *string                                    `json:"scheduledSuspendAt,omitempty"`
			SuspendType           management.WorkspaceAutoSuspendSuspendType `json:"suspendType"`
			SuspendTypeChangedAt  *string                                    `json:"suspendTypeChangedAt,omitempty"`
		}{
			SuspendType:           management.WorkspaceAutoSuspendSuspendTypeSCHEDULED,
			ScheduledAfterSeconds: util.Ptr(scheduledAfterSeconds),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/workspaces/%s", workspace.WorkspaceID), r.URL.Path)
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
					WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal(workspace.WorkspaceID.String())).
					WithWorkspaceGetDataSource("this")("name", unset).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", config.IDAttribute, workspace.WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "workspace_group_id", workspace.WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "name", workspace.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "state", string(workspace.State)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "size", workspace.Size),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "created_at", workspace.CreatedAt),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "endpoint", *workspace.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "last_resumed_at", *workspace.LastResumedAt),
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
	workspaceGroup1 := management.WorkspaceGroup{
		WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
		Name:             "group1",
	}

	workspaceGroup2 := management.WorkspaceGroup{
		WorkspaceGroupID: uuid.MustParse("f2b1b970-8591-4196-bb26-f53f0f8e35ce"),
		Name:             "group2",
	}

	workspace1 := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             "target-workspace",
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: workspaceGroup1.WorkspaceGroupID,
		LastResumedAt:    util.Ptr("2023-03-14T17:28:32.430878Z"),
		Endpoint:         util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		Size:             "S-00",
		AutoSuspend: &struct {
			IdleAfterSeconds      *float32                                   `json:"idleAfterSeconds,omitempty"`
			IdleChangedAt         *string                                    `json:"idleChangedAt,omitempty"`
			ScheduledAfterSeconds *float32                                   `json:"scheduledAfterSeconds,omitempty"`
			ScheduledChangedAt    *string                                    `json:"scheduledChangedAt,omitempty"`
			ScheduledSuspendAt    *string                                    `json:"scheduledSuspendAt,omitempty"`
			SuspendType           management.WorkspaceAutoSuspendSuspendType `json:"suspendType"`
			SuspendTypeChangedAt  *string                                    `json:"suspendTypeChangedAt,omitempty"`
		}{
			SuspendType:           management.WorkspaceAutoSuspendSuspendTypeSCHEDULED,
			ScheduledAfterSeconds: util.Ptr(scheduledAfterSeconds),
		},
	}

	workspace2 := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             "other-workspace",
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      uuid.MustParse("a3c2d980-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: workspaceGroup1.WorkspaceGroupID,
		Size:             "S-1",
	}

	workspaceGroups := []management.WorkspaceGroup{workspaceGroup1, workspaceGroup2}
	workspacesGroup1 := []management.Workspace{workspace1, workspace2}
	workspacesGroup2 := []management.Workspace{} // Empty group

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")

		switch r.URL.Path {
		case v1WorkspaceGroups:
			// Return all workspace groups
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case v1Workspaces:
			// Check which workspace group is being queried
			workspaceGroupID := r.URL.Query().Get("workspaceGroupID")
			switch workspaceGroupID {
			case workspaceGroup1.WorkspaceGroupID.String():
				_, err := w.Write(testutil.MustJSON(workspacesGroup1))
				require.NoError(t, err)
			case workspaceGroup2.WorkspaceGroupID.String():
				_, err := w.Write(testutil.MustJSON(workspacesGroup2))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", config.IDAttribute, workspace1.WorkspaceID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "workspace_group_id", workspace1.WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "name", workspace1.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "state", string(workspace1.State)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "size", workspace1.Size),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "created_at", workspace1.CreatedAt),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "endpoint", *workspace1.Endpoint),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "last_resumed_at", *workspace1.LastResumedAt),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "auto_suspend.suspend_type", "SCHEDULED"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", "auto_suspend.suspend_after_seconds", fmt.Sprintf("%.0f", scheduledAfterSeconds)),
				),
			},
		},
	})
}

func TestWorkspaceNotFoundByName(t *testing.T) {
	workspaceGroup := management.WorkspaceGroup{
		WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
		Name:             "group1",
	}

	workspace := management.Workspace{
		Name:             "existing-workspace",
		WorkspaceID:      uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: workspaceGroup.WorkspaceGroupID,
	}

	workspaceGroups := []management.WorkspaceGroup{workspaceGroup}
	workspaces := []management.Workspace{workspace}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")

		switch r.URL.Path {
		case v1WorkspaceGroups:
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case v1Workspaces:
			_, err := w.Write(testutil.MustJSON(workspaces))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
	workspaceGroup1 := management.WorkspaceGroup{
		WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
		Name:             "group1",
	}

	workspaceGroup2 := management.WorkspaceGroup{
		WorkspaceGroupID: uuid.MustParse("f2b1b970-8591-4196-bb26-f53f0f8e35ce"),
		Name:             "group2",
	}

	workspace1 := management.Workspace{
		Name:             "duplicate-name",
		WorkspaceID:      uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: workspaceGroup1.WorkspaceGroupID,
	}

	workspace2 := management.Workspace{
		Name:             "duplicate-name",
		WorkspaceID:      uuid.MustParse("a3c2d980-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: workspaceGroup2.WorkspaceGroupID,
	}

	workspaceGroups := []management.WorkspaceGroup{workspaceGroup1, workspaceGroup2}
	workspacesGroup1 := []management.Workspace{workspace1}
	workspacesGroup2 := []management.Workspace{workspace2}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")

		switch r.URL.Path {
		case v1WorkspaceGroups:
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case v1Workspaces:
			workspaceGroupID := r.URL.Query().Get("workspaceGroupID")
			switch workspaceGroupID {
			case workspaceGroup1.WorkspaceGroupID.String():
				_, err := w.Write(testutil.MustJSON(workspacesGroup1))
				require.NoError(t, err)
			case workspaceGroup2.WorkspaceGroupID.String():
				_, err := w.Write(testutil.MustJSON(workspacesGroup2))
				require.NoError(t, err)
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
	workspaceGroup := management.WorkspaceGroup{
		WorkspaceGroupID: uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce"),
		Name:             "group1",
	}

	workspace := management.Workspace{
		CreatedAt:        "2023-02-28T05:33:06.3003Z",
		Name:             "Test-Workspace-Name",
		State:            management.WorkspaceStateACTIVE,
		WorkspaceID:      uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce"),
		WorkspaceGroupID: workspaceGroup.WorkspaceGroupID,
		Size:             "S-00",
	}

	workspaceGroups := []management.WorkspaceGroup{workspaceGroup}
	workspaces := []management.Workspace{workspace}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")

		switch r.URL.Path {
		case v1WorkspaceGroups:
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case v1Workspaces:
			_, err := w.Write(testutil.MustJSON(workspaces))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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
					resource.TestCheckResourceAttr("data.singlestoredb_workspace.this", config.IDAttribute, workspace.WorkspaceID.String()),
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
