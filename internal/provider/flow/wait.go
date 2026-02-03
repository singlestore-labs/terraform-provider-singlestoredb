package flow

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

// waitCondition returns nil if it is satisfied.
type waitCondition func(management.Flow) error

func wait(ctx context.Context, c management.ClientWithResponsesInterface, id management.FlowID, timeout time.Duration, conditions ...waitCondition) (management.Flow, *util.SummaryWithDetailError) {
	result := management.Flow{}

	if err := retry.RetryContext(ctx, timeout, func() *retry.RetryError {
		flow, err := c.GetV1FlowFlowIDWithResponse(ctx, id)
		if err != nil { // Not status code OK does not get here, not retrying for that reason.
			ferr := fmt.Errorf("failed to get Flow instance %s: %w", id, err)

			return retry.NonRetryableError(ferr)
		}

		if code := flow.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get Flow instance %s: status code %s", id, http.StatusText(code))

			return retry.RetryableError(err)
		}

		// Check if the Flow instance has been terminated
		if flow.JSON200.DeletedAt != nil {
			err := fmt.Errorf("Flow instance %s has been terminated; %s", flow.JSON200.FlowID, config.ContactSupportErrorDetail)

			return retry.NonRetryableError(err)
		}

		for _, c := range conditions {
			if err := c(*flow.JSON200); err != nil {
				return retry.RetryableError(err)
			}
		}

		result = *flow.JSON200

		return nil
	}); err != nil {
		return result, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to wait for Flow instance %s creation", id),
			Detail:  fmt.Sprintf("Flow instance is not ready: %s", err.Error()),
		}
	}

	return result, nil
}

func waitConditionEndpointReady() func(management.Flow) error {
	endpointHistory := make([]bool, 0, config.FlowInstanceConsistencyThreshold)

	return func(f management.Flow) error {
		hasEndpoint := f.Endpoint != nil && *f.Endpoint != ""
		endpointHistory = append(endpointHistory, hasEndpoint)

		if !hasEndpoint {
			return fmt.Errorf("Flow instance %s endpoint is not yet available", f.FlowID)
		}

		// Check that endpoint has been consistently available
		if !checkLastNTrue(endpointHistory, config.FlowInstanceConsistencyThreshold) {
			return fmt.Errorf("Flow instance %s endpoint is available but the Management API did not return it consistently for %d iterations yet",
				f.FlowID, config.FlowInstanceConsistencyThreshold,
			)
		}

		return nil
	}
}

// checkLastNTrue checks if the last n elements in the slice are all true.
func checkLastNTrue(history []bool, n int) bool {
	if len(history) < n {
		return false
	}

	for i := len(history) - n; i < len(history); i++ {
		if !history[i] {
			return false
		}
	}

	return true
}
