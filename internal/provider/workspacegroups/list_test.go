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
)

func TestReadsWorkspaceGroups(t *testing.T) {
	workspaceGroups := []management.Cluster{
		{
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
		},
		{
			AllowAllTraffic: util.Ptr(true),
			CreatedAt:       mustParseTimePtr("2022-07-15T15:11:09.185048Z"),
			ExpiresAt:       util.Ptr("2222-07-15T15:11:09.185048Z"),
			FirewallRanges:  nil,
			Name:            "bar",
			Provider:        util.Ptr(management.CloudProviderGCP),
			Region:          util.Ptr("us-west-1"),
			State:           util.Ptr(management.ClusterStatePENDING),
			TerminatedAt:    nil,
			UpdateWindow:    nil,
			GroupID:         util.Ptr(uuid.MustParse("f1a0a960-8691-4196-bb26-f53f1f8e35ce")),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		switch r.URL.Path {
		case pathV2Clusters:
			_, err := w.Write(testutil.MustJSON(workspaceGroups))
			require.NoError(t, err)
		case pathV2Projects:
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
				Config: examples.WorkspaceGroupsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", config.IDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.#", "2"),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.allow_all_traffic"),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.expires_at"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.firewall_ranges.#",
						strconv.Itoa(len(util.Deref(workspaceGroups[0].FirewallRanges))),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.name", workspaceGroups[0].Name),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.region_id"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.cloud_provider", string(util.Deref(workspaceGroups[0].Provider))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.region_name", util.Deref(workspaceGroups[0].Region)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.state", string(util.Deref(workspaceGroups[0].State))),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.terminated_at"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.update_window.day",
						strconv.Itoa(int(workspaceGroups[0].UpdateWindow.Day)),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.update_window.hour",
						strconv.Itoa(int(workspaceGroups[0].UpdateWindow.Hour)),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.deployment_type", string(defaultDeploymentType)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.0.outbound_allow_list", testOutboundAllowList),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.allow_all_traffic",
						strconv.FormatBool(util.Deref(workspaceGroups[1].AllowAllTraffic)),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.expires_at",
						util.NormalizeTimestampString(util.Deref(workspaceGroups[1].ExpiresAt)),
					),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.firewall_ranges.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.firewall_ranges.0", "0.0.0.0/0"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.name", workspaceGroups[1].Name),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.region_id"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.cloud_provider", string(util.Deref(workspaceGroups[1].Provider))),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.region_name", util.Deref(workspaceGroups[1].Region)),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.state", string(util.Deref(workspaceGroups[1].State))),
					resource.TestCheckNoResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.terminated_at"),
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", "workspace_groups.1.update_window.%", "0"), // Not present for legacy schedules.
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
				Config:      examples.WorkspaceGroupsListDataSource,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestReadsWorkspaceGroupsIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspaceGroupsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestoredb_workspace_groups.all", config.IDAttribute, config.TestIDValue),
					// Checking that at least no error.
				),
			},
		},
	})
}
