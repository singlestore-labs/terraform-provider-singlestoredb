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
		if err != nil {
			// The HTTP client may return errors due to 5xx responses after exhausting its retries.
			// We should continue retrying here since the Flow instance may still be initializing.
			ferr := fmt.Errorf("failed to get Flow instance %s: %w", id, err)

			return retry.RetryableError(ferr)
		}

		if code := flow.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get Flow instance %s: status code %s", id, http.StatusText(code))

			return retry.RetryableError(err)
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
			return fmt.Errorf("flow instance %s endpoint is not yet available", f.FlowID)
		}

		if !util.CheckLastN(endpointHistory, config.FlowInstanceConsistencyThreshold, true) {
			return fmt.Errorf("flow instance %s endpoint is available but the Management API did not return it consistently for %d iterations yet",
				f.FlowID, config.FlowInstanceConsistencyThreshold,
			)
		}

		return nil
	}
}
