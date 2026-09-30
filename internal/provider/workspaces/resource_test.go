package workspaces_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

var (
	updatedWorkspaceSize          = "S-0"
	updatedCacheConfig    float32 = 4
	updatedScaleFactor    float32 = 2
	updatedMaxScaleFactor float32 = 4
	updatedSensitivity            = "LOW"
	updatedSuspendSeconds float32 = 1200
	updatedSuspendType            = "IDLE"
)

func TestCRUDWorkspace(t *testing.T) { //nolint:maintidx
	newEndpoint := util.Ptr("svc-14a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com")

	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	projectID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	starterClusterID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	workspaceID := uuid.MustParse("f2a1a960-8591-4156-bb26-f53f0f8e35ce")

	starterCluster := management.Cluster{
		AllowAllTraffic: util.Ptr(false),
		CreatedAt:       util.Ptr(time.Now().UTC()),
		ExpiresAt:       util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:  util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:            config.TestInitialWorkspaceGroupName,
		Region:          util.Ptr("us-east-1"),
		Provider:        util.Ptr(management.CloudProviderAWS),
		State:           util.Ptr(management.ClusterStateACTIVE),
		GroupID:         util.Ptr(workspaceGroupID),
		ClusterID:       util.Ptr(starterClusterID),
		ProjectID:       projectID,
		DeploymentType:  util.Ptr(management.PRODUCTION),
		SizeConfig:      &management.SizeConfig{Size: util.Ptr("S-00")},
	}

	workspace := management.Cluster{
		CreatedAt:  mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		Name:       config.TestWorkspaceName,
		State:      util.Ptr(management.ClusterStateACTIVE),
		ClusterID:  util.Ptr(workspaceID),
		GroupID:    util.Ptr(workspaceGroupID),
		ProjectID:  projectID,
		Endpoint:   util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		SizeConfig: &management.SizeConfig{Size: util.Ptr(config.TestInitialWorkspaceSize), ScaleFactor: util.Ptr[float32](1), CacheConfig: util.Ptr[float32](1)},
	}

	workspaceExists := false
	starterExists := true
	postCount := 0
	patchAttempts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		workspacePath := strings.Join([]string{"/v2/clusters", workspaceID.String()}, "/")
		starterPath := strings.Join([]string{"/v2/clusters", starterClusterID.String()}, "/")

		switch {
		case r.URL.Path == "/v2/projects" && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{
				Name: config.TestInitialProjectName, ProjectID: projectID, Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}}))
			require.NoError(t, err)
		case r.URL.Path == "/v2/clusters" && r.Method == http.MethodGet:
			clusters := make([]management.Cluster, 0, 2)
			if starterExists {
				clusters = append(clusters, starterCluster)
			}
			if workspaceExists {
				clusters = append(clusters, workspace)
			}
			_, err := w.Write(testutil.MustJSON(clusters))
			require.NoError(t, err)
		case r.URL.Path == "/v2/clusters" && r.Method == http.MethodPost:
			postCount++
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			if postCount == 1 {
				_, err = w.Write(testutil.MustJSON(struct {
					ClusterID uuid.UUID `json:"clusterID"`
					GroupID   uuid.UUID `json:"groupID"`
				}{ClusterID: starterClusterID, GroupID: workspaceGroupID}))
			} else {
				require.Nil(t, input.AutoScale, "AutoScale should be nil when max_scale_factor defaults to 1")
				workspaceExists = true
				_, err = w.Write(testutil.MustJSON(struct {
					ClusterID uuid.UUID `json:"clusterID"`
					GroupID   uuid.UUID `json:"groupID"`
				}{ClusterID: workspaceID, GroupID: workspaceGroupID}))
			}
			require.NoError(t, err)
		case r.URL.Path == workspacePath && r.Method == http.MethodGet:
			if !workspaceExists {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			_, err := w.Write(testutil.MustJSON(workspace))
			require.NoError(t, err)
		case r.URL.Path == starterPath && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON(starterCluster))
			require.NoError(t, err)
		case r.URL.Path == strings.Join([]string{workspacePath, "suspend"}, "/") && r.Method == http.MethodPost:
			workspace.State = util.Ptr(management.ClusterStateSUSPENDED)
			workspace.Endpoint = nil
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: workspaceID}))
			require.NoError(t, err)
		case r.URL.Path == strings.Join([]string{workspacePath, "resume"}, "/") && r.Method == http.MethodPost:
			workspace.State = util.Ptr(management.ClusterStateACTIVE)
			workspace.Endpoint = newEndpoint
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: workspaceID}))
			require.NoError(t, err)
		case r.URL.Path == workspacePath && r.Method == http.MethodPatch:
			patchAttempts++
			if patchAttempts == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, updatedWorkspaceSize, util.Deref(input.SizeConfig.Size))
			require.Equal(t, updatedCacheConfig, util.Deref(input.SizeConfig.CacheConfig))
			require.Equal(t, updatedScaleFactor, util.Deref(input.SizeConfig.ScaleFactor))
			require.Equal(t, updatedMaxScaleFactor, util.Deref(input.AutoScale.MaxScaleFactor))
			require.Equal(t, management.AutoScaleSensitivity(updatedSensitivity), util.Deref(input.AutoScale.Sensitivity))
			workspace.SizeConfig = &management.SizeConfig{
				Size:        util.Ptr(updatedWorkspaceSize),
				CacheConfig: util.Ptr(updatedCacheConfig),
				ScaleFactor: util.Ptr(updatedScaleFactor),
			}
			workspace.AutoScale = &management.AutoScale{
				MaxScaleFactor: util.Ptr(updatedMaxScaleFactor),
				Sensitivity:    util.Ptr(management.AutoScaleSensitivity(updatedSensitivity)),
			}
			workspace.AutoSuspend = &management.AutoSuspend{
				SuspendType:      util.Ptr(management.IDLE),
				IdleAfterSeconds: util.Ptr(int(updatedSuspendSeconds)),
			}
			_, err = w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: workspaceID}))
			require.NoError(t, err)
		case r.URL.Path == workspacePath && r.Method == http.MethodDelete:
			workspaceExists = false
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: workspaceID}))
			require.NoError(t, err)
		case r.URL.Path == starterPath && r.Method == http.MethodDelete:
			starterExists = false
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"`
			}{ClusterID: starterClusterID}))
			require.NoError(t, err)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.WorkspacesResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", config.IDAttribute, workspaceID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "workspace_group_id", workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "name", workspace.Name),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "created_at", workspace.CreatedAt.Format(time.RFC3339)),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "endpoint", "svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "kai_enabled", "false"),
					resource.TestCheckNoResourceAttr("singlestoredb_workspace.this", "last_resumed_at"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_suspend.suspend_type", "DISABLED"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(true)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "true"),
					resource.TestCheckNoResourceAttr("singlestoredb_workspace.this", "endpoint"),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(false)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "endpoint", *newEndpoint),
				),
			},
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceResource("this")("suspended", cty.BoolVal(false)).
					WithWorkspaceResource("this")("size", cty.StringVal(updatedWorkspaceSize)).
					WithWorkspaceResource("this")("cache_config", cty.NumberIntVal(int64(updatedCacheConfig))).
					WithWorkspaceResource("this")("scale_factor", cty.NumberIntVal(int64(updatedScaleFactor))).
					WithWorkspaceResource("this")("auto_scale", cty.ObjectVal(map[string]cty.Value{
					"max_scale_factor": cty.NumberIntVal(int64(updatedMaxScaleFactor)),
					"sensitivity":      cty.StringVal(updatedSensitivity),
				})).
					WithWorkspaceResource("this")("auto_suspend", cty.ObjectVal(map[string]cty.Value{
					"suspend_after_seconds": cty.NumberIntVal(int64(updatedSuspendSeconds)),
					"suspend_type":          cty.StringVal(updatedSuspendType),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", updatedWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "cache_config", fmt.Sprintf("%.0f", updatedCacheConfig)),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "scale_factor", fmt.Sprintf("%.0f", updatedScaleFactor)),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_scale.max_scale_factor", fmt.Sprintf("%.0f", updatedMaxScaleFactor)),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_scale.sensitivity", updatedSensitivity),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_suspend.suspend_after_seconds", fmt.Sprintf("%.0f", updatedSuspendSeconds)),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_suspend.suspend_type", updatedSuspendType),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "endpoint", *newEndpoint),
				),
			},
		},
	})

	require.GreaterOrEqual(t, patchAttempts, 2)
	require.False(t, workspaceExists)
	require.False(t, starterExists)
}

func TestWorkspaceResourceIntegration(t *testing.T) {
	adminPassword := "sfkjDIJ423d44w1sfooBar1$" //nolint:gosec
	isConnectable := testutil.IsConnectableWithAdminPassword(adminPassword)

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "example",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
					WithWorkspaceResource("this")("auto_scale", cty.ObjectVal(map[string]cty.Value{
					"max_scale_factor": cty.NumberIntVal(int64(updatedMaxScaleFactor)),
					"sensitivity":      cty.StringVal(updatedSensitivity),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "name", config.TestWorkspaceName),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", isConnectable),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_scale.max_scale_factor", fmt.Sprintf("%.0f", updatedMaxScaleFactor)),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_scale.sensitivity", updatedSensitivity),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_suspend.suspend_type", "DISABLED"),
				),
			},
		},
	})
}
