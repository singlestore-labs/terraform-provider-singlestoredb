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
	"github.com/singlestore-labs/terraform-provider-singlestore/examples"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestReadsWorkspaceGroups(t *testing.T) {
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
		{
			AllowAllTraffic:  util.Ptr(true),
			CreatedAt:        "2022-07-15T15:11:09.185048Z",
			ExpiresAt:        util.Ptr("2222-07-15T15:11:09.185048Z"),
			FirewallRanges:   nil,
			Name:             "bar",
			RegionID:         uuid.MustParse("1aa1aff3-5092-4a0c-bf36-da54e85a5fdf"),
			State:            management.WorkspaceGroupStatePENDING,
			TerminatedAt:     nil,
			UpdateWindow:     nil,
			WorkspaceGroupID: uuid.MustParse("f1a0a960-8691-4196-bb26-f53f1f8e35ce"),
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/workspaceGroups", r.URL.Path)
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(workspaceGroups))
		require.NoError(t, err)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspaceGroupsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", config.TestIDAttribute, config.TestIDValue),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.#", "2"),
					resource.TestCheckNoResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.allow_all_traffic"),
					resource.TestCheckNoResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.expires_at"),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.firewall_ranges.#",
						strconv.Itoa(len(util.Deref(workspaceGroups[0].FirewallRanges))),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.name", workspaceGroups[0].Name),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.region_id", workspaceGroups[0].RegionID.String()),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.state", string(workspaceGroups[0].State)),
					resource.TestCheckNoResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.terminated_at"),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.update_window.day",
						strconv.Itoa(int(workspaceGroups[0].UpdateWindow.Day)),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.0.update_window.hour",
						strconv.Itoa(int(workspaceGroups[0].UpdateWindow.Hour)),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.allow_all_traffic",
						strconv.FormatBool(util.Deref(workspaceGroups[1].AllowAllTraffic)),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.expires_at",
						util.Deref(workspaceGroups[1].ExpiresAt),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.firewall_ranges.#",
						strconv.Itoa(len(util.Deref(workspaceGroups[1].FirewallRanges))),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.name", workspaceGroups[1].Name),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.region_id", workspaceGroups[1].RegionID.String()),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.state", string(workspaceGroups[1].State)),
					resource.TestCheckNoResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.terminated_at"),
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", "workspace_groups.1.update_window.%", "0"), // Not present for legacy schedules.
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
				Config:      examples.WorkspaceGroupsListDataSource,
				ExpectError: r,
			},
		},
	})
}

func TestReadsWorkspaceGroupsIntegration(t *testing.T) {
	apiKey := os.Getenv(config.EnvTestAPIKey)

	testutil.IntegrationTest(t, apiKey, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspaceGroupsListDataSource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestore_workspace_groups.all", config.TestIDAttribute, config.TestIDValue),
					// Checking that at least no error.
				),
			},
		},
	})
}
