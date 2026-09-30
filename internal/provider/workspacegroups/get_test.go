package workspacegroups_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
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

var (
	testOutboundAllowList = "arn:aws:iam::1234567890:root"
	unset                 = cty.Value{}
)

func TestReadsWorkspaceGroupByID(t *testing.T) {
	workspaceGroup := management.Cluster{
		AllowAllTraffic: nil,
		CreatedAt:       mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		ExpiresAt:       nil,
		FirewallRanges:  util.Ptr([]string{"127.0.0.1/32"}),
		Name:            "foo",
		Provider:        util.Ptr(management.CloudProviderAWS),
		Region:          util.Ptr("us-west-2"),
		State:           util.Ptr(management.ClusterStateACTIVE),
		TerminatedAt:    nil,
		UpdateWindow: &management.UpdateWindow{
			Day:  3,
			Hour: 15,
		},
		GroupID:           util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		DeploymentType:    &defaultDeploymentType,
		OutboundAllowList: &testOutboundAllowList,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch r.URL.Path {
		case "/v2/clusters":
			_, err := w.Write(testutil.MustJSON([]management.Cluster{workspaceGroup}))
			require.NoError(t, err)
		case "/v2/projects":
			_, err := w.Write(testutil.MustJSON([]management.Project{}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")(config.IDAttribute, cty.StringVal(util.Deref(workspaceGroup.GroupID).String())).
					WithWorkspaceGroupGetDataSource("this")("name", unset).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", config.IDAttribute, util.Deref(workspaceGroup.GroupID).String()),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_group.this", "allow_all_traffic"),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_group.this", "expires_at"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "firewall_ranges.#",
						strconv.Itoa(len(util.Deref(workspaceGroup.FirewallRanges))),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "name", workspaceGroup.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "cloud_provider", string(util.Deref(workspaceGroup.Provider))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "region_name", "us-west-2"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "state", string(util.Deref(workspaceGroup.State))),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_group.this", "terminated_at"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "update_window.day",
						strconv.Itoa(int(workspaceGroup.UpdateWindow.Day)),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "update_window.hour",
						strconv.Itoa(int(workspaceGroup.UpdateWindow.Hour)),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "deployment_type", string(defaultDeploymentType)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "outbound_allow_list", testOutboundAllowList),
				),
			},
		},
	})
}

func TestWorkspaceGroupNotFoundByID(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithWorkspaceGroupGetDataSource("this")("name", unset).
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")(config.IDAttribute, cty.StringVal("valid-uuid")).
					WithWorkspaceGroupGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile("invalid UUID"),
			},
		},
	})
}

func TestGetWorkspaceGroupNotFoundByIDIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithWorkspaceGroupGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusNotFound)), // Checking that at least the expected error.
			},
		},
	})
}

func TestReadsWorkspaceGroupByName(t *testing.T) {
	workspaceGroup := management.Cluster{
		AllowAllTraffic: nil,
		CreatedAt:       mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		ExpiresAt:       nil,
		FirewallRanges:  util.Ptr([]string{"127.0.0.1/32"}),
		Name:            "test-workspace-group",
		Provider:        util.Ptr(management.CloudProviderAWS),
		Region:          util.Ptr("us-west-2"),
		State:           util.Ptr(management.ClusterStateACTIVE),
		TerminatedAt:    nil,
		UpdateWindow: &management.UpdateWindow{
			Day:  3,
			Hour: 15,
		},
		GroupID:           util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		DeploymentType:    &defaultDeploymentType,
		OutboundAllowList: &testOutboundAllowList,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch r.URL.Path {
		case "/v2/clusters":
			workspaceGroups := []management.Cluster{workspaceGroup}
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case "/v2/projects":
			_, err := w.Write(testutil.MustJSON([]management.Project{}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")("name", cty.StringVal(workspaceGroup.Name)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", config.IDAttribute, util.Deref(workspaceGroup.GroupID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "name", workspaceGroup.Name),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "cloud_provider", string(util.Deref(workspaceGroup.Provider))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "region_name", "us-west-2"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "state", string(util.Deref(workspaceGroup.State))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "deployment_type", string(defaultDeploymentType)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "outbound_allow_list", testOutboundAllowList),
				),
			},
		},
	})
}

func TestWorkspaceGroupByNameNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch r.URL.Path {
		case "/v2/clusters":
			workspaceGroups := []management.Cluster{}
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case "/v2/projects":
			_, err := w.Write(testutil.MustJSON([]management.Project{}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")("name", cty.StringVal("nonexistent-workspace-group")).
					String(),
				ExpectError: regexp.MustCompile("Workspace group not found"),
			},
		},
	})
}

func TestWorkspaceGroupByNameMultipleFound(t *testing.T) {
	workspaceGroup1 := management.Cluster{
		AllowAllTraffic:   nil,
		CreatedAt:         mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		ExpiresAt:         nil,
		FirewallRanges:    util.Ptr([]string{"127.0.0.1/32"}),
		Name:              "duplicate-name",
		Provider:          util.Ptr(management.CloudProviderAWS),
		Region:            util.Ptr("us-west-2"),
		State:             util.Ptr(management.ClusterStateACTIVE),
		TerminatedAt:      nil,
		GroupID:           util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		DeploymentType:    &defaultDeploymentType,
		OutboundAllowList: &testOutboundAllowList,
	}

	workspaceGroup2 := workspaceGroup1
	workspaceGroup2.GroupID = util.Ptr(uuid.MustParse("1aa1aff3-4092-4a0c-bf36-da54e85a4fdf"))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch r.URL.Path {
		case "/v2/clusters":
			workspaceGroups := []management.Cluster{workspaceGroup1, workspaceGroup2}
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case "/v2/projects":
			_, err := w.Write(testutil.MustJSON([]management.Project{}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        "bar",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")("name", cty.StringVal("duplicate-name")).
					String(),
				ExpectError: regexp.MustCompile("Multiple workspace groups found"),
			},
		},
	})
}

func TestMissingIdentifier(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")("name", unset).
					String(),
				ExpectError: regexp.MustCompile("Missing identifier"),
			},
		},
	})
}

func TestConflictingIdentifiers(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")(config.IDAttribute, cty.StringVal(uuid.New().String())).
					WithWorkspaceGroupGetDataSource("this")("name", cty.StringVal("test-name")).
					String(),
				ExpectError: regexp.MustCompile("Conflicting identifiers"),
			},
		},
	})
}

func TestWorkspaceGroupByNameCaseInsensitive(t *testing.T) {
	workspaceGroup := management.Cluster{
		AllowAllTraffic: nil,
		CreatedAt:       mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		ExpiresAt:       nil,
		FirewallRanges:  util.Ptr([]string{"127.0.0.1/32"}),
		Name:            "Test-Workspace-Group",
		Provider:        util.Ptr(management.CloudProviderAWS),
		Region:          util.Ptr("us-west-2"),
		State:           util.Ptr(management.ClusterStateACTIVE),
		TerminatedAt:    nil,
		UpdateWindow: &management.UpdateWindow{
			Day:  3,
			Hour: 15,
		},
		GroupID:           util.Ptr(uuid.MustParse("e1a0a960-8591-4196-bb26-f53f0f8e35ce")),
		DeploymentType:    &defaultDeploymentType,
		OutboundAllowList: &testOutboundAllowList,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch r.URL.Path {
		case "/v2/clusters":
			workspaceGroups := []management.Cluster{workspaceGroup}
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case "/v2/projects":
			_, err := w.Write(testutil.MustJSON([]management.Project{}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")("name", cty.StringVal("test-workspace-group")).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", config.IDAttribute, util.Deref(workspaceGroup.GroupID).String()),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_group.this", "name", workspaceGroup.Name),
				),
			},
		},
	})
}

func TestGetWorkspaceGroupNotFoundByNameIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithWorkspaceGroupGetDataSource("this")("name", cty.StringVal("no-such-group")).
					String(),
				ExpectError: regexp.MustCompile("No workspace group"), // Checking that at least the expected error.
			},
		},
	})
}
