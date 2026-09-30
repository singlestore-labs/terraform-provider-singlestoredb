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

func TestCRUDWorkspace(t *testing.T) { //nolint:maintidx,cyclop
	newEndpoint := util.Ptr("svc-14a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com")

	workspaceGroupID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	projectID := uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	// Under /v2/clusters the workspace adopts the workspace_group starter cluster.
	clusterID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")

	cluster := management.Cluster{
		AllowAllTraffic: util.Ptr(false),
		CreatedAt:       mustParseTimePtr("2023-02-28T05:33:06.3003Z"),
		ExpiresAt:       util.Ptr(config.TestInitialWorkspaceGroupExpiresAt),
		FirewallRanges:  util.Ptr([]string{config.TestFirewallFirewallRangeAllTraffic}),
		Name:            config.TestInitialWorkspaceGroupName,
		Region:          util.Ptr("us-east-1"),
		Provider:        util.Ptr(management.CloudProviderAWS),
		State:           util.Ptr(management.ClusterStateACTIVE),
		GroupID:         util.Ptr(workspaceGroupID),
		ClusterID:       util.Ptr(clusterID),
		ProjectID:       projectID,
		DeploymentType:  util.Ptr(management.PRODUCTION),
		Endpoint:        util.Ptr("svc-94a328d2-8c3d-412d-91a0-c32a750673cb-dml.aws-oregon-3.svc.singlestore.com"),
		SizeConfig:      &management.SizeConfig{Size: util.Ptr("S-00"), ScaleFactor: util.Ptr[float32](1), CacheConfig: util.Ptr[float32](1)},
	}

	clusterExists := true
	postCount := 0
	updatePatches := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "json")
		clusterPath := strings.Join([]string{"/v2/clusters", clusterID.String()}, "/")

		switch {
		case r.URL.Path == "/v2/projects" && r.Method == http.MethodGet:
			_, err := w.Write(testutil.MustJSON([]management.Project{{
				Name: config.TestInitialProjectName, ProjectID: projectID, Edition: management.STANDARD, CreatedAt: time.Now().UTC(),
			}}))
			require.NoError(t, err)
		case r.URL.Path == "/v2/clusters" && r.Method == http.MethodGet:
			clusters := []management.Cluster{}
			if clusterExists {
				clusters = append(clusters, cluster)
			}
			_, err := w.Write(testutil.MustJSON(clusters))
			require.NoError(t, err)
		case r.URL.Path == "/v2/clusters" && r.Method == http.MethodPost:
			postCount++
			require.Equal(t, 1, postCount, "workspace create should adopt the starter cluster, not POST a second cluster")
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			require.Equal(t, config.TestInitialWorkspaceGroupName, input.Name)
			_, err = w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
				GroupID   uuid.UUID `json:"groupID"`   //nolint:tagliatelle // API uses groupID.
			}{ClusterID: clusterID, GroupID: workspaceGroupID}))
			require.NoError(t, err)
		case r.URL.Path == clusterPath && r.Method == http.MethodGet:
			if !clusterExists {
				w.WriteHeader(http.StatusNotFound)

				return
			}
			_, err := w.Write(testutil.MustJSON(cluster))
			require.NoError(t, err)
		case r.URL.Path == strings.Join([]string{clusterPath, "suspend"}, "/") && r.Method == http.MethodPost:
			cluster.State = util.Ptr(management.ClusterStateSUSPENDED)
			cluster.Endpoint = nil
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: clusterID}))
			require.NoError(t, err)
		case r.URL.Path == strings.Join([]string{clusterPath, "resume"}, "/") && r.Method == http.MethodPost:
			cluster.State = util.Ptr(management.ClusterStateACTIVE)
			cluster.Endpoint = newEndpoint
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: clusterID}))
			require.NoError(t, err)
		case r.URL.Path == clusterPath && r.Method == http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var input management.Cluster
			require.NoError(t, json.Unmarshal(body, &input))
			require.Nil(t, input.Kai, "PATCH must omit default kai=false to avoid mongoproxy teardown")
			updatePatches++
			if updatePatches == 1 {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}
			require.Equal(t, updatedWorkspaceSize, util.Deref(input.SizeConfig.Size))
			require.Equal(t, updatedCacheConfig, util.Deref(input.SizeConfig.CacheConfig))
			require.Equal(t, updatedScaleFactor, util.Deref(input.SizeConfig.ScaleFactor))
			require.Equal(t, updatedMaxScaleFactor, util.Deref(input.AutoScale.MaxScaleFactor))
			require.Equal(t, management.AutoScaleSensitivity(updatedSensitivity), util.Deref(input.AutoScale.Sensitivity))
			cluster.SizeConfig = &management.SizeConfig{
				Size:        util.Ptr(updatedWorkspaceSize),
				CacheConfig: util.Ptr(updatedCacheConfig),
				ScaleFactor: util.Ptr(updatedScaleFactor),
			}
			cluster.AutoScale = &management.AutoScale{
				MaxScaleFactor: util.Ptr(updatedMaxScaleFactor),
				Sensitivity:    util.Ptr(management.AutoScaleSensitivity(updatedSensitivity)),
			}
			cluster.AutoSuspend = &management.AutoSuspend{
				SuspendType:      util.Ptr(management.IDLE),
				IdleAfterSeconds: util.Ptr(int(updatedSuspendSeconds)),
			}
			_, err = w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: clusterID}))
			require.NoError(t, err)
		case r.URL.Path == clusterPath && r.Method == http.MethodDelete:
			clusterExists = false
			_, err := w.Write(testutil.MustJSON(struct {
				ClusterID uuid.UUID `json:"clusterID"` //nolint:tagliatelle // API uses clusterID.
			}{ClusterID: clusterID}))
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
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", config.IDAttribute, clusterID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "workspace_group_id", workspaceGroupID.String()),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "name", config.TestWorkspaceName),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "created_at", cluster.CreatedAt.Format(time.RFC3339)),
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

	require.GreaterOrEqual(t, updatePatches, 2)
	require.False(t, clusterExists)
}

func TestWorkspaceResourceIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "example",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.WorkspacesResource).
					WithWorkspaceResource("this")("auto_scale", cty.ObjectVal(map[string]cty.Value{
					"max_scale_factor": cty.NumberIntVal(int64(updatedMaxScaleFactor)),
					"sensitivity":      cty.StringVal(updatedSensitivity),
				})).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "name", config.TestWorkspaceName),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "size", config.TestInitialWorkspaceSize),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "suspended", "false"),
					// Prefer Data API over MySQL: GH runners often time out on :3306 even when
					// the workspace allowlist and admin password are correct.
					testutil.IsDataAPIReadyUsingGroupPassword("singlestoredb_workspace.this", "singlestoredb_workspace_group.example"),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_scale.max_scale_factor", fmt.Sprintf("%.0f", updatedMaxScaleFactor)),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_scale.sensitivity", updatedSensitivity),
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "auto_suspend.suspend_type", "DISABLED"),
				),
			},
		},
	})
}
