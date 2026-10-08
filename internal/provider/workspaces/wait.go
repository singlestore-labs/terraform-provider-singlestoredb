package workspaces

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// waitCondition return nil if it is satisfied.
type waitCondition func(management.Cluster) error

func wait(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, timeout time.Duration, conditions ...waitCondition) (management.Cluster, *util.SummaryWithDetailError) {
	result := management.Cluster{}

	if err := retry.RetryContext(ctx, timeout, func() *retry.RetryError {
		workspace, err := c.GetV2ClustersClusterIDWithResponse(ctx, id, &management.GetV2ClustersClusterIDParams{})
		if err != nil { // Not status code OK does not get here, not retrying for that reason.
			ferr := fmt.Errorf("failed to get workspace %s: %w", id, err)

			return retry.NonRetryableError(ferr)
		}

		if code := workspace.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get workspace %s: status code %s", id, http.StatusText(code))

			return retry.RetryableError(err)
		}

		if clusterState(*workspace.JSON200) == management.ClusterStateFAILED {
			err := fmt.Errorf("workspace %s failed; %s", util.Deref(workspace.JSON200.ClusterID), config.ContactSupportErrorDetail)

			return retry.NonRetryableError(err)
		}

		for _, c := range conditions {
			if err := c(*workspace.JSON200); err != nil {
				return retry.RetryableError(err)
			}
		}

		result = *workspace.JSON200

		return nil
	}); err != nil {
		return result, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to wait for a workspace %s creation", id),
			Detail:  fmt.Sprintf("Workspace is not ready: %s", err.Error()),
		}
	}

	return result, nil
}

func waitConditionState(states ...management.ClusterState) func(management.Cluster) error {
	workspaceStateHistory := make([]management.ClusterState, 0, config.WorkspaceConsistencyThreshold)

	return func(w management.Cluster) error {
		state := clusterState(w)
		workspaceStateHistory = append(workspaceStateHistory, state)

		if !util.Any(states, state) {
			return fmt.Errorf("workspace %s state is %s, but should be %s", util.Deref(w.ClusterID), state, util.Join(states, ", "))
		}

		if !util.CheckLastN(workspaceStateHistory, config.WorkspaceConsistencyThreshold, states...) {
			return fmt.Errorf("workspace %s state is %s but the Management API did not return the same state for the consequent %d iterations yet",
				util.Deref(w.ClusterID), state, config.WorkspaceConsistencyThreshold,
			)
		}

		return nil
	}
}

func waitConditionSize(desiredSize string) func(management.Cluster) error {
	return func(w management.Cluster) error {
		size := clusterSize(w)
		if size != desiredSize {
			return fmt.Errorf("workspace %s size is %s, but should be %s", util.Deref(w.ClusterID), size, desiredSize)
		}

		return nil
	}
}

func waitConditionTakesAtLeast(d time.Duration) func(management.Cluster) error {
	begin := time.Now()
	atLeast := begin.Add(d)

	return func(_ management.Cluster) error {
		if time.Now().Before(atLeast) {
			return fmt.Errorf("should wait at least until %s (%s starting from %s)", atLeast.UTC(), d, begin)
		}

		return nil
	}
}
