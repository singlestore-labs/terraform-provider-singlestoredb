package clusters

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestWaitConditions(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	size := "S-00"
	cluster := management.Cluster{
		ClusterID: &clusterID,
		SizeConfig: &management.SizeConfig{
			Size: &size,
		},
		FirewallRanges: util.Ptr([]string{"10.0.0.0/8"}),
	}

	require.NoError(t, waitConditionSize(size)(cluster))
	require.Error(t, waitConditionSize("S-1")(cluster))
	require.Error(t, waitConditionSize(size)(management.Cluster{ClusterID: &clusterID}))

	require.NoError(t, waitConditionFirewallRanges([]types.String{types.StringValue("10.0.0.0/8")})(cluster))
	require.Error(t, waitConditionFirewallRanges([]types.String{types.StringValue("192.168.0.0/16")})(cluster))

	require.NoError(t, waitConditionTakesAtLeast(-time.Second)(cluster))
	require.Error(t, waitConditionTakesAtLeast(time.Hour)(cluster))
}

func TestWaitClusterFailures(t *testing.T) {
	clusterID := uuid.MustParse("3ca3d359-021d-45ed-86cb-38b8d14ac507")
	failed := management.ClusterStateFAILED

	failedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body, err := json.Marshal(management.Cluster{ClusterID: &clusterID, State: &failed})
		require.NoError(t, err)
		_, err = w.Write(body)
		require.NoError(t, err)
	}))
	t.Cleanup(failedServer.Close)

	failedClient, err := management.NewClientWithResponses(failedServer.URL)
	require.NoError(t, err)
	_, serr := wait(t.Context(), failedClient, clusterID, time.Second, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "failed")

	statusServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(statusServer.Close)
	statusClient, err := management.NewClientWithResponses(statusServer.URL)
	require.NoError(t, err)
	_, serr = wait(t.Context(), statusClient, clusterID, time.Millisecond, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)

	down, err := management.NewClientWithResponses("http://127.0.0.1:1")
	require.NoError(t, err)
	_, serr = wait(t.Context(), down, clusterID, time.Second, waitConditionState(management.ClusterStateACTIVE))
	require.NotNil(t, serr)
	require.Contains(t, serr.Detail, "failed to get cluster")
}
