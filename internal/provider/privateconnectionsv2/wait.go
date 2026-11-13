package privateconnectionsv2

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// pollGrace is how long an in-flight poll may run after the wait timeout.
// RetryContext still accepts a poll that reaches the expected state during that
// window. Canceling the poll at the wait deadline drops that result.
const pollGrace = 30 * time.Second

// notFoundGrace is how long a wait or refresh tolerates consecutive 404 responses.
// A connection can be briefly missing right after create. A connection deleted
// outside Terraform stops once this elapses instead of polling until the timeout.
// The grace is time-based because the poll interval backs off: a fixed count of
// 404s is a few seconds at the start of a wait and much longer later on.
// Tests shorten it.
var notFoundGrace = time.Minute

type waitMode int

const (
	// waitUntilActive polls until the connection is active and every condition holds.
	// DELETED stops the wait.
	waitUntilActive waitMode = iota
	// waitUntilDeleted polls until the connection is DELETED or no longer returned.
	waitUntilDeleted
	// readExisting returns the connection, including a DELETED one, or reports 404 after the grace.
	readExisting
)

type waitCondition func(management.ClusterPrivateConnection) error

// notFoundTracker measures how long the API has been returning 404 without an
// intervening response.
type notFoundTracker struct {
	grace time.Duration
	now   func() time.Time
	since time.Time
}

func newNotFoundTracker() *notFoundTracker {
	return &notFoundTracker{grace: notFoundGrace, now: time.Now}
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

// readConnection polls until the connection is returned or 404s last for the grace.
// notFound is true when the polls ended on an uninterrupted run of 404s, which
// means the connection is gone. A transport error, another status, or caller
// cancellation after a 404 is reported as an error instead, so Read does not
// drop a connection that may still exist.
// Authentication failures stop immediately. Other errors are retried for the grace.
func readConnection(ctx context.Context, c management.ClientWithResponsesInterface, id management.ConnectionID) (*management.ClusterPrivateConnection, bool, *util.SummaryWithDetailError) {
	tracker := newNotFoundTracker()
	var found *management.ClusterPrivateConnection
	// notFoundStreak is true while every poll since the last other outcome got 404.
	// A transport error or another status ends the streak.
	notFoundStreak := false

	pollCtx, cancel := context.WithTimeout(ctx, notFoundGrace+pollGrace)
	defer cancel()

	err := retry.RetryContext(ctx, notFoundGrace, func() *retry.RetryError {
		resp, err := c.GetV2PrivateConnectionsConnectionIDWithResponse(pollCtx, id, &management.GetV2PrivateConnectionsConnectionIDParams{})
		if err != nil {
			notFoundStreak = false

			return pollTransportError(ctx, pollCtx, id, err, nil)
		}

		notFoundStreak = resp.StatusCode() == http.StatusNotFound
		decision := classifyGet(id, resp.StatusCode(), resp.JSON200, responseContentType(resp.HTTPResponse), tracker, readExisting)
		if decision.ready != nil {
			found = decision.ready
		}

		return decision.retry
	})
	// A canceled caller is an error even after 404s: the streak did not finish.
	if err != nil && notFoundStreak && ctx.Err() == nil {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to read private connection %s", id),
			Detail:  waitErrorDetail(ctx, notFoundGrace, err, "Private connection could not be read"),
		}
	}

	return found, false, nil
}

// wait polls until the connection matches mode and every condition, or the timeout elapses.
func wait(ctx context.Context, c management.ClientWithResponsesInterface, id management.ConnectionID, timeout time.Duration, mode waitMode, conditions ...waitCondition) (management.ClusterPrivateConnection, *util.SummaryWithDetailError) {
	result := management.ClusterPrivateConnection{}
	tracker := newNotFoundTracker()
	stopped := false
	var lastCondition error

	pollCtx, cancel := context.WithTimeout(ctx, timeout+pollGrace)
	defer cancel()

	if err := retry.RetryContext(ctx, timeout, func() *retry.RetryError {
		resp, err := c.GetV2PrivateConnectionsConnectionIDWithResponse(pollCtx, id, &management.GetV2PrivateConnectionsConnectionIDParams{})
		if err != nil {
			return pollTransportError(ctx, pollCtx, id, err, lastCondition)
		}

		decision := classifyGet(id, resp.StatusCode(), resp.JSON200, responseContentType(resp.HTTPResponse), tracker, mode)
		if decision.retry != nil {
			stopped = !decision.retry.Retryable
			if decision.retry.Retryable {
				lastCondition = decision.retry.Err
			}

			return decision.retry
		}

		if decision.ready == nil {
			// Delete treats a 404 as gone. Any other empty body is tried again.
			if mode == waitUntilDeleted {
				return nil
			}

			return retry.RetryableError(fmt.Errorf("failed to get private connection %s", id))
		}

		for _, condition := range conditions {
			if cerr := condition(*decision.ready); cerr != nil {
				lastCondition = cerr

				return retry.RetryableError(cerr)
			}
		}

		result = *decision.ready

		return nil
	}); err != nil {
		summary := fmt.Sprintf("Failed to wait for private connection %s", id)
		detail := waitErrorDetail(ctx, timeout, err, "Private connection did not reach the expected state")
		if stopped {
			detail = err.Error()
		}

		return result, &util.SummaryWithDetailError{Summary: summary, Detail: detail}
	}

	return result, nil
}

type pollDecision struct {
	retry *retry.RetryError
	ready *management.ClusterPrivateConnection
}

// classifyGet decides what one GET means.
// 404 is success for delete and, after the grace, a missing connection for a read or create.
// 401 and 403 stop immediately. DELETED stops a wait for ACTIVE and is returned to a read.
func classifyGet(id management.ConnectionID, code int, conn *management.ClusterPrivateConnection, contentType string, tracker *notFoundTracker, mode waitMode) pollDecision {
	if code == http.StatusNotFound {
		if mode == waitUntilDeleted {
			return pollDecision{}
		}
		err := fmt.Errorf("failed to get private connection %s: status code %s", id, http.StatusText(code))
		if tracker.notFound() {
			return pollDecision{retry: retry.NonRetryableError(fmt.Errorf("%w for %s; it may have been deleted outside Terraform", err, tracker.grace))}
		}

		return pollDecision{retry: retry.RetryableError(err)}
	}

	tracker.found()
	if code != http.StatusOK {
		return pollDecision{retry: statusError(id, code)}
	}
	if conn == nil {
		return pollDecision{retry: retry.RetryableError(fmt.Errorf("failed to get private connection %s: status code OK but the response is not a JSON private connection (Content-Type %q)", id, contentType))}
	}

	return classifyBody(conn, mode)
}

func classifyBody(conn *management.ClusterPrivateConnection, mode waitMode) pollDecision {
	switch status := util.Deref(conn.Status); status {
	case management.ClusterPrivateConnectionStatusDELETED:
		if mode == waitUntilActive {
			return pollDecision{retry: retry.NonRetryableError(fmt.Errorf("private connection %s status is %s; it may have been deleted outside Terraform; run terraform apply to create it again, or terraform destroy to remove it from state",
				conn.PrivateConnectionID, status))}
		}

		return pollDecision{ready: conn}
	case management.ClusterPrivateConnectionStatusACTIVE, management.ClusterPrivateConnectionStatusPENDING:
		if mode == waitUntilDeleted {
			return pollDecision{retry: retry.RetryableError(fmt.Errorf("private connection %s status is %s, but should be %s", conn.PrivateConnectionID, status, management.ClusterPrivateConnectionStatusDELETED))}
		}

		return pollDecision{ready: conn}
	default:
		// A missing status is not terminal. The next poll may report one.
		if mode == waitUntilDeleted {
			return pollDecision{retry: retry.RetryableError(fmt.Errorf("private connection %s status is %s, but should be %s", conn.PrivateConnectionID, status, management.ClusterPrivateConnectionStatusDELETED))}
		}

		return pollDecision{ready: conn}
	}
}

func statusError(id management.ConnectionID, code int) *retry.RetryError {
	err := fmt.Errorf("failed to get private connection %s: status code %s", id, http.StatusText(code))
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return retry.NonRetryableError(fmt.Errorf("%w; authentication or authorization failure", err))
	}

	return retry.RetryableError(err)
}

// pollTransportError retries a reset, DNS failure, or client timeout.
// Cancellation of the caller's context stops the wait. Cancellation of the poll
// context means the deadline passed, so the last condition is kept.
func pollTransportError(ctx, pollCtx context.Context, id management.ConnectionID, err error, lastCondition error) *retry.RetryError {
	if ctx.Err() != nil {
		return retry.NonRetryableError(fmt.Errorf("failed to get private connection %s: %w", id, err))
	}
	if pollCtx.Err() != nil {
		if lastCondition != nil {
			return retry.NonRetryableError(lastCondition)
		}

		return retry.NonRetryableError(fmt.Errorf("failed to get private connection %s: %w", id, err))
	}

	return retry.RetryableError(fmt.Errorf("failed to get private connection %s: %w", id, err))
}

func waitErrorDetail(ctx context.Context, timeout time.Duration, err error, timedOut string) string {
	if ctx.Err() != nil {
		return fmt.Sprintf("The wait was canceled (%s); last error: %s", ctx.Err(), err)
	}

	return fmt.Sprintf("%s within %s: %s", timedOut, timeout, err)
}

func responseContentType(resp *http.Response) string {
	if resp == nil {
		return ""
	}

	return resp.Header.Get("Content-Type")
}

func waitConditionStatus(desired management.ClusterPrivateConnectionStatus) waitCondition {
	history := make([]management.ClusterPrivateConnectionStatus, 0, config.PrivateConnectionConsistencyThreshold)

	return func(conn management.ClusterPrivateConnection) error {
		status := util.Deref(conn.Status)
		history = append(history, status)
		if status != desired {
			return fmt.Errorf("private connection %s status is %s, but should be %s", conn.PrivateConnectionID, status, desired)
		}
		// Only the current status counts. A mix of accepted statuses is still a transition.
		if !util.CheckLastN(history, config.PrivateConnectionConsistencyThreshold, status) {
			return fmt.Errorf("private connection %s status is %s but the Management API did not return the same status for the consequent %d iterations yet",
				conn.PrivateConnectionID, status, config.PrivateConnectionConsistencyThreshold)
		}

		return nil
	}
}

func waitConditionAllowList(desired string) waitCondition {
	return func(conn management.ClusterPrivateConnection) error {
		if util.Deref(conn.AllowList) != desired {
			return fmt.Errorf("private connection %s allow_list is %s, but should be %s", conn.PrivateConnectionID, util.Deref(conn.AllowList), desired)
		}

		return nil
	}
}

func waitConditionRegions(desired []string) waitCondition {
	return func(conn management.ClusterPrivateConnection) error {
		if sameRegions(conn.SupportedCrossRegions, desired) {
			return nil
		}

		return fmt.Errorf("private connection %s supported_cross_regions is %v, but should be %v", conn.PrivateConnectionID, util.Deref(conn.SupportedCrossRegions), desired)
	}
}

func sameRegions(got *[]string, want []string) bool {
	left := append([]string(nil), util.Deref(got)...)
	right := append([]string(nil), want...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}

	return true
}
