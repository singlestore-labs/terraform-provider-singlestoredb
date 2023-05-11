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
type waitCondition func(management.Workspace) error

func wait(ctx context.Context, c management.ClientWithResponsesInterface, id management.WorkspaceID, timeout time.Duration, conditions ...waitCondition) (management.Workspace, *util.SummaryWithDetailError) {
	result := management.Workspace{}

	if err := retry.RetryContext(ctx, timeout, func() *retry.RetryError {
		workspace, err := c.GetV1WorkspacesWorkspaceIDWithResponse(ctx, id, &management.GetV1WorkspacesWorkspaceIDParams{})
		if err != nil { // Not status code OK does not get here, not retrying for that reason.
			ferr := fmt.Errorf("failed to get workspace %s: %w", id, err)

			return retry.NonRetryableError(ferr)
		}

		if code := workspace.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get workspace %s: status code %s", id, http.StatusText(code))

			return retry.RetryableError(err)
		}

		if workspace.JSON200.State == management.WorkspaceStateFAILED {
			err := fmt.Errorf("workspace %s failed; %s", workspace.JSON200.WorkspaceID, config.ContactSupportErrorDetail)

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

func waitConditionState(states ...management.WorkspaceState) func(management.Workspace) error {
	workspaceStateHistory := make([]management.WorkspaceState, 0, config.WorkspaceConsistencyThreshold)

	return func(w management.Workspace) error {
		workspaceStateHistory = append(workspaceStateHistory, w.State)

		if !util.Any(states, w.State) {
			return fmt.Errorf("workspace %s state is %s, but should be %s", w.WorkspaceID, w.State, util.Join(states, ", "))
		}

		if !util.CheckLastN(workspaceStateHistory, config.WorkspaceConsistencyThreshold, states...) {
			return fmt.Errorf("workspace %s state is %s but the Management API did not return the same state for the consequent %d iterations yet",
				w.WorkspaceID, w.State, config.WorkspaceConsistencyThreshold,
			)
		}

		return nil
	}
}

func waitConditionSize(desiredSize string) func(management.Workspace) error {
	return func(w management.Workspace) error {
		if w.Size != desiredSize {
			return fmt.Errorf("workspace %s size is %s, but should be %s", w.WorkspaceID, w.Size, desiredSize)
		}

		return nil
	}
}

func waitConditionTakesAtLeast(d time.Duration) func(management.Workspace) error {
	begin := time.Now()
	atLeast := begin.Add(d)

	return func(_ management.Workspace) error {
		if time.Now().Before(atLeast) {
			return fmt.Errorf("should wait at least until %s (%s starting from %s)", atLeast.UTC(), d, begin)
		}

		return nil
	}
}
