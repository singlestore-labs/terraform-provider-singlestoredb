package workspacegroups_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestore/examples"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/testutil"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/util"
	"github.com/stretchr/testify/require"
)

var (
	updatedWorkspaceGroupName = strings.Join([]string{"updated", config.TestInitialWorkspaceGroupName}, "-") //nolint
	updatedAdminPassword      = "buzzBAR123$"                                                                //nolint
	emptyFirewallRanges       = ""                                                                           //nolint
)

func TestCRUDWorkspaceGroup(t *testing.T) {
	regions := []management.Region{
		{
			RegionID: uuid.MustParse("2ca3d358-021d-45ed-86cb-38b8d14ac507"),
			Region:   "GS - US West 2 (Oregon) - aws-oregon-gs1",
			Provider: management.AWS,
		},
	}
	workspaceGroupCreateResponse := struct {
		WorkspaceGroupID uuid.UUID
	}{
		WorkspaceGroupID: uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507"),
	}
	workspaceGroup := management.WorkspaceGroup{
		AllowAllTraffic:  util.Ptr(false),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:        util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:   util.Ptr([]string{config.TestInitialFirewallRange}),
		Name:             config.TestInitialWorkspaceGroupName,
		RegionID:         regions[0].RegionID,
		State:            management.WorkspaceGroupStatePENDING, // During the first poll, the status will still be PENDING.
		TerminatedAt:     nil,
		UpdateWindow:     nil,
		WorkspaceGroupID: workspaceGroupCreateResponse.WorkspaceGroupID,
	}
	workspaceGroupTerminateResponse := struct {
		WorkspaceGroupID uuid.UUID
	}{
		WorkspaceGroupID: uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507"),
	}
	updatedExpiresAt := time.Now().UTC().Add(time.Hour * 2).Format(time.RFC3339)
	workspaceGroupUpdateResponse := struct {
		WorkspaceGroupID uuid.UUID
	}{
		WorkspaceGroupID: uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507"),
	}

	call := 0
	expectedCalls := 19

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() { call++ }()

		switch call {
		case 0, 1, 6, 7, 9, 10, 12, 14, 15, 17:
			require.Equal(t, "/v1/regions", r.URL.Path, strconv.Itoa(call))

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(regions))
			require.NoError(t, err)
		case 2: // Create.
			require.Equal(t, "/v1/workspaceGroups", r.URL.Path, strconv.Itoa(call))
			require.Equal(t, http.MethodPost, r.Method)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.WorkspaceGroupCreate
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, config.TestInitialAdminPassword, util.Deref(input.AdminPassword))
			require.Equal(t, false, util.Deref(input.AllowAllTraffic))
			require.Equal(t, config.TestInitialWorkspaceGroupExpiresAt, util.Deref(input.ExpiresAt))
			require.Equal(t, []string{config.TestInitialFirewallRange}, input.FirewallRanges)
			require.Equal(t, config.TestInitialWorkspaceGroupName, input.Name)
			require.Equal(t, regions[0].RegionID, input.RegionID)

			w.Header().Add("Content-Type", "json")
			_, err = w.Write(testutil.MustJSON(workspaceGroupCreateResponse))
			require.NoError(t, err)
		case 3: // Read.
			require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupCreateResponse.WorkspaceGroupID.String()}, "/"), r.URL.Path, strconv.Itoa(call))
			require.Equal(t, http.MethodGet, r.Method)

			w.Header().Add("Content-Type", "json")
			w.WriteHeader(http.StatusNotFound)
		case 4, 5, 8, 11, 16: // Read.
			require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupCreateResponse.WorkspaceGroupID.String()}, "/"), r.URL.Path, strconv.Itoa(call))
			require.Equal(t, http.MethodGet, r.Method)

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(workspaceGroup))
			require.NoError(t, err)
			workspaceGroup.State = management.WorkspaceGroupStateACTIVE // Marking the state as ACTIVE to end polling.
		case 13: // Update.
			require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupCreateResponse.WorkspaceGroupID.String()}, "/"), r.URL.Path, strconv.Itoa(call))
			require.Equal(t, http.MethodPatch, r.Method)
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.WorkspaceGroupUpdate
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, updatedAdminPassword, util.Deref(input.AdminPassword))
			require.True(t, util.Deref(input.AllowAllTraffic))
			require.Equal(t, updatedExpiresAt, util.Deref(input.ExpiresAt))
			require.Empty(t, util.Deref(input.FirewallRanges))
			require.Equal(t, updatedWorkspaceGroupName, util.Deref(input.Name))

			w.Header().Add("Content-Type", "json")
			_, err = w.Write(testutil.MustJSON(workspaceGroupUpdateResponse))
			require.NoError(t, err)
			workspaceGroup.ExpiresAt = &updatedExpiresAt
			workspaceGroup.Name = updatedWorkspaceGroupName
			workspaceGroup.AllowAllTraffic = util.Ptr(true)
			workspaceGroup.FirewallRanges = util.Ptr([]string{}) // Updating for the next reads.
		case 18: // Delete.
			require.Equal(t, strings.Join([]string{"/v1/workspaceGroups", workspaceGroupCreateResponse.WorkspaceGroupID.String()}, "/"), r.URL.Path, strconv.Itoa(call))
			require.Equal(t, http.MethodDelete, r.Method)

			w.Header().Add("Content-Type", "json")
			_, err := w.Write(testutil.MustJSON(workspaceGroupTerminateResponse))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			require.Fail(t, "Management API should be called not more than %d times, but is called the %d time now with the URL %s", expectedCalls, call, r.URL.Path)
		}
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.Config{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspaceGroupsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", config.IDAttribute, workspaceGroupCreateResponse.WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "name", config.TestInitialWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "created_at", workspaceGroup.CreatedAt),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "expires_at", *workspaceGroup.ExpiresAt),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "region_id", regions[0].RegionID.String()),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "admin_password", config.TestInitialAdminPassword),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithOverride(
						config.TestInitialWorkspaceGroupName,
						updatedWorkspaceGroupName,
					).
					WithOverride(
						config.TestInitialAdminPassword,
						updatedAdminPassword,
					).
					WithOverride(
						config.TestInitialWorkspaceGroupExpiresAt,
						updatedExpiresAt,
					).
					WithOverride(
						config.TestInitialWorkspaceGroupExpiresAt,
						updatedExpiresAt,
					).
					WithOverride(
						strconv.Quote(config.TestInitialFirewallRange),
						emptyFirewallRanges,
					).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", config.IDAttribute, workspaceGroupCreateResponse.WorkspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "created_at", workspaceGroup.CreatedAt),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "expires_at", updatedExpiresAt),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "region_id", regions[0].RegionID.String()),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "admin_password", updatedAdminPassword),
				),
			},
		},
	})

	require.Equal(t, expectedCalls, call, "Management API should be called %d times, but is called %d times", expectedCalls, call)
}

func TestWorkspaceGroupResourceIntegration(t *testing.T) {
	apiKey := os.Getenv(config.EnvTestAPIKey)

	testutil.IntegrationTest(t, apiKey, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspaceGroupsResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestore_workspace_group.example", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "name", config.TestInitialWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "admin_password", config.TestInitialAdminPassword),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspaceGroupsResource).
					WithOverride(
						config.TestInitialWorkspaceGroupName,
						updatedWorkspaceGroupName,
					).
					WithOverride(
						config.TestInitialAdminPassword,
						updatedAdminPassword,
					).
					WithOverride(
						strconv.Quote(config.TestInitialFirewallRange),
						emptyFirewallRanges,
					). // Not testing updating expires at because of the limitations of testutil.IntegrationTest that ensures garbage collection.
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestore_workspace_group.example", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "name", updatedWorkspaceGroupName),
					resource.TestCheckResourceAttr("singlestore_workspace_group.example", "admin_password", updatedAdminPassword),
				),
			},
		},
	})
}
