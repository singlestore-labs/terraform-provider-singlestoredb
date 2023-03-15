package workspacegroups_test

import (
	"fmt"
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

func TestReadsWorkspaceGroup(t *testing.T) {
	workspaceGroup := management.WorkspaceGroup{
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
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, fmt.Sprintf("/v1/workspaceGroups/%s", workspaceGroup.WorkspaceGroupID), r.URL.Path)
		w.Header().Add("Content-Type", "json") // Necessary to make the library parse the resulting JSON.
		_, err := w.Write(testutil.MustJSON(workspaceGroup))
		require.NoError(t, err)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithOverride(config.TestInitialWorkspaceGroupID, workspaceGroup.WorkspaceGroupID.String()).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.singlestore_workspace_group.example", config.IDAttribute, workspaceGroup.WorkspaceGroupID.String()),
					resource.TestCheckNoResourceAttr("data.singlestore_workspace_group.example", "allow_all_traffic"),
					resource.TestCheckNoResourceAttr("data.singlestore_workspace_group.example", "expires_at"),
					resource.TestCheckResourceAttr("data.singlestore_workspace_group.example", "firewall_ranges.#",
						strconv.Itoa(len(util.Deref(workspaceGroup.FirewallRanges))),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_group.example", "name", workspaceGroup.Name),
					resource.TestCheckResourceAttr("data.singlestore_workspace_group.example", "region_id", workspaceGroup.RegionID.String()),
					resource.TestCheckResourceAttr("data.singlestore_workspace_group.example", "state", string(workspaceGroup.State)),
					resource.TestCheckNoResourceAttr("data.singlestore_workspace_group.example", "terminated_at"),
					resource.TestCheckResourceAttr("data.singlestore_workspace_group.example", "update_window.day",
						strconv.Itoa(int(workspaceGroup.UpdateWindow.Day)),
					),
					resource.TestCheckResourceAttr("data.singlestore_workspace_group.example", "update_window.hour",
						strconv.Itoa(int(workspaceGroup.UpdateWindow.Hour)),
					),
				),
			},
		},
	})
}

func TestWorkspaceGroupNotFound(t *testing.T) {
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithOverride(config.TestInitialWorkspaceGroupID, uuid.New().String()).
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
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsGetDataSource).
					WithOverride(config.TestInitialWorkspaceGroupID, "invalid-uuid").
					String(),
				ExpectError: r,
			},
		},
	})
}

func TestGetWorkspaceGroupNotFoundIntegration(t *testing.T) {
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
