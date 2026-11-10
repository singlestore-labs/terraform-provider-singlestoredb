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

// clusterPollError classifies a Management API transport failure during a wait.
// Context cancellation stops immediately. Other transport errors are retried
// until the wait timeout.
func clusterPollError(ctx context.Context, id management.ClusterID, err error) *retry.RetryError {
	ferr := fmt.Errorf("failed to get cluster %s: %w", id, err)
	if ctx.Err() != nil {
		return retry.NonRetryableError(ferr)
	}

	return retry.RetryableError(ferr)
}

// clusterNotFoundGrace is how long a wait tolerates consecutive 404 responses.
// A cluster can be briefly missing right after create, but a cluster deleted
// mid-wait must fail fast instead of polling until the full timeout. The grace
// is time-based because the poll interval backs off: a fixed count of 404s
// spans a few seconds at the start of a wait and almost a minute later on.
// It is a variable so tests can shorten it.
var clusterNotFoundGrace = time.Minute

// notFoundTracker measures how long the Management API has been returning 404
// for a cluster without an intervening successful read.
type notFoundTracker struct {
	grace time.Duration
	now   func() time.Time
	since time.Time
}

func newNotFoundTracker() *notFoundTracker {
	return &notFoundTracker{grace: clusterNotFoundGrace, now: time.Now}
}

// notFound records a 404 and reports whether 404s have lasted at least the grace.
func (t *notFoundTracker) notFound() bool {
	now := t.now()
	if t.since.IsZero() {
		t.since = now
	}

	return now.Sub(t.since) >= t.grace
}

// found records a response other than 404.
func (t *notFoundTracker) found() {
	t.since = time.Time{}
}

// errMissingClusterBody reports a 200 response without a JSON cluster, for example
// an HTML page from a proxy. It is retried like other transient responses.
func errMissingClusterBody(id management.ClusterID, contentType string) error {
	return fmt.Errorf("failed to get cluster %s: status code OK but the response is not a JSON cluster (Content-Type %q)", id, contentType)
}

// waitErrorDetail describes why a wait ended: canceled, stopped on a terminal
// condition, or timed out. RetryContext returns the last poll error instead of
// its timeout or cancellation error, so the reason is reconstructed here.
func waitErrorDetail(ctx context.Context, timeout time.Duration, stopped bool, err error, timedOut string) string {
	switch {
	case ctx.Err() != nil:
		return fmt.Sprintf("The wait was canceled (%s); last error: %s", ctx.Err(), err)
	case stopped:
		return err.Error()
	default:
		return fmt.Sprintf("%s within %s: %s", timedOut, timeout, err)
	}
}

// wait polls the cluster until every condition holds or the timeout elapses.
func wait(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, timeout time.Duration, conditions ...waitCondition) (management.Cluster, *util.SummaryWithDetailError) {
	result := management.Cluster{}
	notFound := newNotFoundTracker()
	stopped := false

	// RetryContext does not cancel a poll in flight when it times out, and returns
	// after a 30s grace while the poll may still run and write to result. Bound
	// every request by the wait timeout so polls end with the wait.
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := retry.RetryContext(ctx, timeout, func() *retry.RetryError {
		cluster, err := c.GetV2ClustersClusterIDWithResponse(pollCtx, id, &management.GetV2ClustersClusterIDParams{})
		if err != nil {
			// Status codes are handled below. A reset, DNS failure, or client
			// timeout is retried until the wait times out. Create stores the
			// cluster before this wait, so one blip must not fail it and taint
			// a cluster that is still provisioning. Context cancellation stops
			// the wait so Terraform can shut the plugin down.
			return clusterPollError(ctx, id, err)
		}

		if code := cluster.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get cluster %s: status code %s", id, http.StatusText(code))
			if code != http.StatusNotFound {
				notFound.found()

				return retry.RetryableError(err)
			}

			if notFound.notFound() {
				stopped = true

				return retry.NonRetryableError(fmt.Errorf("%w for %s; it may have been deleted outside Terraform", err, notFound.grace))
			}

			return retry.RetryableError(err)
		}
		notFound.found()

		if cluster.JSON200 == nil {
			return retry.RetryableError(errMissingClusterBody(id, cluster.HTTPResponse.Header.Get("Content-Type")))
		}

		// FAILED and TERMINATED will not become ready. Stop instead of polling
		// until the wait timeout after the cluster fails, expires, or is deleted.
		if err := terminalClusterError(cluster.JSON200); err != nil {
			stopped = true

			return retry.NonRetryableError(err)
		}

		for _, cond := range conditions {
			if err := cond(*cluster.JSON200); err != nil {
				return retry.RetryableError(err)
			}
		}

		result = *cluster.JSON200

		return nil
	}); err != nil {
		return result, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to wait for cluster %s", id),
			Detail:  waitErrorDetail(ctx, timeout, stopped, err, "Cluster did not reach the expected state"),
		}
	}

	return result, nil
}

// isTerminatedCluster reports whether the cluster is terminated. Management API
// reports a cluster terminated before its endpoint became ready as FAILED or
// PENDING, but still sets terminatedAt, so terminatedAt is checked as well.
func isTerminatedCluster(cluster management.Cluster) bool {
	return util.Deref(cluster.State) == management.ClusterStateTERMINATED || cluster.TerminatedAt != nil
}

// terminalClusterError reports a failed or terminated cluster so a wait stops
// instead of polling until the timeout.
func terminalClusterError(cluster *management.Cluster) error {
	id := util.Deref(cluster.ClusterID)
	if cluster.TerminatedAt != nil {
		return fmt.Errorf("cluster %s was terminated at %s; it may have expired or been deleted outside Terraform; run terraform apply -replace=ADDRESS to replace it, or terraform destroy to remove it",
			id, cluster.TerminatedAt.UTC().Format(time.RFC3339))
	}

	switch util.Deref(cluster.State) {
	case management.ClusterStateFAILED:
		return fmt.Errorf("cluster %s failed; run terraform apply -replace=ADDRESS to replace it, or terraform destroy to remove it; %s",
			id, config.ContactSupportErrorDetail)
	case management.ClusterStateTERMINATED:
		return fmt.Errorf("cluster %s is terminated; it may have expired or been deleted outside Terraform; run terraform apply -replace=ADDRESS to replace it, or terraform destroy to remove it",
			id)
	case management.ClusterStateACTIVE, management.ClusterStatePENDING, management.ClusterStateSUSPENDED, management.ClusterStateUNKNOWN:
		// Listed so the exhaustive linter flags a new state that may be terminal.
	}

	return nil
}

// waitConditionState holds once the cluster reports one of states for several consecutive polls.
func waitConditionState(states ...management.ClusterState) func(management.Cluster) error {
	clusterStateHistory := make([]management.ClusterState, 0, config.WorkspaceConsistencyThreshold)

	return func(c management.Cluster) error {
		state := util.Deref(c.State)
		clusterStateHistory = append(clusterStateHistory, state)

		if !util.Any(states, state) {
			return fmt.Errorf("cluster %s state is %s, but should be %s", util.Deref(c.ClusterID), state, util.Join(states, ", "))
		}

		if !util.CheckLastN(clusterStateHistory, config.WorkspaceConsistencyThreshold, states...) {
			return fmt.Errorf("cluster %s state is %s but the Management API did not return the same state for %d consecutive polls yet",
				util.Deref(c.ClusterID), state, config.WorkspaceConsistencyThreshold,
			)
		}

		return nil
	}
}

// waitConditionSize holds once the Management API reports the desired cluster size.
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

// waitConditionTakesAtLeast holds only after d has elapsed, so a scale is not observed too early.
func waitConditionTakesAtLeast(d time.Duration) func(management.Cluster) error {
	begin := time.Now()
	atLeast := begin.Add(d)

	return func(_ management.Cluster) error {
		if time.Now().Before(atLeast) {
			return fmt.Errorf("should wait at least until %s (%s starting from %s)", atLeast.UTC(), d, begin.UTC())
		}

		return nil
	}
}

// waitConditionAutoScale holds until the Management API reports the max scale factor.
func waitConditionAutoScale(maxScaleFactor float32) waitCondition {
	return func(c management.Cluster) error {
		var got float32 = scaleX1
		if c.AutoScale != nil && c.AutoScale.MaxScaleFactor != nil {
			got = *c.AutoScale.MaxScaleFactor
		}
		if got != maxScaleFactor {
			return fmt.Errorf("cluster %s max scale factor is %v, but should be %v", util.Deref(c.ClusterID), got, maxScaleFactor)
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

// waitConditionSettled holds while the cluster is changing state, for example
// while a suspend, resume, or scale that timed out earlier is still running.
func waitConditionSettled() waitCondition {
	return func(c management.Cluster) error {
		if state := util.Deref(c.State); !isSettledClusterState(state) {
			return fmt.Errorf("cluster %s state is %s, but should be %s or %s",
				util.Deref(c.ClusterID), state, management.ClusterStateACTIVE, management.ClusterStateSUSPENDED)
		}

		return nil
	}
}

// waitTerminated polls until the Management API reports the cluster as terminated
// (TERMINATED state or a terminatedAt timestamp) or no longer returns it.
func waitTerminated(ctx context.Context, c management.ClientWithResponsesInterface, id management.ClusterID, timeout time.Duration) *util.SummaryWithDetailError {
	// See wait for why requests are bounded by the wait timeout.
	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := retry.RetryContext(ctx, timeout, func() *retry.RetryError {
		cluster, err := c.GetV2ClustersClusterIDWithResponse(pollCtx, id, &management.GetV2ClustersClusterIDParams{})
		if err != nil {
			return clusterPollError(ctx, id, err)
		}

		switch code := cluster.StatusCode(); {
		case code == http.StatusNotFound:
			return nil
		case code != http.StatusOK:
			return retry.RetryableError(fmt.Errorf("failed to get cluster %s: status code %s", id, http.StatusText(code)))
		}

		if cluster.JSON200 == nil {
			return retry.RetryableError(errMissingClusterBody(id, cluster.HTTPResponse.Header.Get("Content-Type")))
		}

		if !isTerminatedCluster(*cluster.JSON200) {
			state := util.Deref(cluster.JSON200.State)

			return retry.RetryableError(fmt.Errorf("cluster %s state is %s, but should be %s", id, state, management.ClusterStateTERMINATED))
		}

		return nil
	}); err != nil {
		return &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to wait for cluster %s to terminate", id),
			Detail:  "The delete request was accepted. " + waitErrorDetail(ctx, timeout, false, err, "The cluster did not terminate"),
		}
	}

	return nil
}
