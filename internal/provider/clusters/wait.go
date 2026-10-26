package clusters

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
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
		cluster, err := c.GetV2ClustersClusterIDWithResponse(ctx, id, &management.GetV2ClustersClusterIDParams{})
		if err != nil { // Not status code OK does not get here, not retrying for that reason.
			ferr := fmt.Errorf("failed to get cluster %s: %w", id, err)

			return retry.NonRetryableError(ferr)
		}

		if code := cluster.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get cluster %s: status code %s", id, http.StatusText(code))

			return retry.RetryableError(err)
		}

		if util.Deref(cluster.JSON200.State) == management.ClusterStateFAILED {
			err := fmt.Errorf("cluster %s failed; %s", util.Deref(cluster.JSON200.ClusterID), config.ContactSupportErrorDetail)

			return retry.NonRetryableError(err)
		}

		for _, c := range conditions {
			if err := c(*cluster.JSON200); err != nil {
				return retry.RetryableError(err)
			}
		}

		result = *cluster.JSON200

		return nil
	}); err != nil {
		return result, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to wait for a cluster %s", id),
			Detail:  fmt.Sprintf("Cluster is not ready: %s", err.Error()),
		}
	}

	return result, nil
}

func waitConditionState(states ...management.ClusterState) func(management.Cluster) error {
	clusterStateHistory := make([]management.ClusterState, 0, config.WorkspaceConsistencyThreshold)

	return func(c management.Cluster) error {
		state := util.Deref(c.State)
		clusterStateHistory = append(clusterStateHistory, state)

		if !util.Any(states, state) {
			return fmt.Errorf("cluster %s state is %s, but should be %s", util.Deref(c.ClusterID), state, util.Join(states, ", "))
		}

		if !util.CheckLastN(clusterStateHistory, config.WorkspaceConsistencyThreshold, states...) {
			return fmt.Errorf("cluster %s state is %s but the Management API did not return the same state for the consequent %d iterations yet",
				util.Deref(c.ClusterID), state, config.WorkspaceConsistencyThreshold,
			)
		}

		return nil
	}
}

func waitConditionSize(desiredSize string) func(management.Cluster) error {
	return func(c management.Cluster) error {
		got := ""
		if c.SizeConfig != nil {
			got = util.Deref(c.SizeConfig.Size)
		}
		if got != desiredSize {
			return fmt.Errorf("cluster %s size is %s, but should be %s", util.Deref(c.ClusterID), got, desiredSize)
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

// waitConditionFirewallRanges holds until the Management API reports the configured
// allowlist. Firewall changes are applied asynchronously, so for a while after a
// create or update the API still reports the previous ranges.
func waitConditionFirewallRanges(firewallRanges []types.String) waitCondition {
	return func(c management.Cluster) error {
		if firewallRangesConverged(firewallRanges, c) {
			return nil
		}

		return fmt.Errorf("cluster %s firewall ranges are [%s] but should be [%s]",
			util.Deref(c.ClusterID),
			util.Join(effectiveFirewallRanges(c), ", "),
			util.Join(util.StringFirewallRanges(firewallRanges), ", "),
		)
	}
}
