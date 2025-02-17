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

const scheduledAfterSeconds float32 = 1200

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

func TestWorkspaceNotFound(t *testing.T) {
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
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestGetWorkspaceNotFoundIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesGetDataSource).
					WithWorkspaceGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)), // Checking that at least the expected error.
			},
		},
	})
}
