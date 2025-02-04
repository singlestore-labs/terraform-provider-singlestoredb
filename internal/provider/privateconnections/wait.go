package privateconnections

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

type waitCondition func(management.PrivateConnection) error

func WaitPrivateConnectionStatus(ctx context.Context, c management.ClientWithResponsesInterface, id management.ConnectionID, conditions ...waitCondition) (management.PrivateConnection, *util.SummaryWithDetailError) {
	result := management.PrivateConnection{}

	if err := retry.RetryContext(ctx, config.PrivateConnectionCreationTimeout, func() *retry.RetryError {
		privateConnection, err := c.GetV1PrivateConnectionsConnectionIDWithResponse(ctx, id, &management.GetV1PrivateConnectionsConnectionIDParams{})
		if err != nil { // Not status code OK does not get here, not retrying for that reason.
			ferr := fmt.Errorf("failed to get private connection %s: %w", id, err)

			return retry.NonRetryableError(ferr)
		}

		if code := privateConnection.StatusCode(); code != http.StatusOK {
			err := fmt.Errorf("failed to get private connection %s: status code %s", id, http.StatusText(code))

			return retry.RetryableError(err)
		}

		if privateConnection.JSON200.Status != nil && *privateConnection.JSON200.Status == management.PrivateConnectionStatusDELETED {
			var result struct {
				Error *string `json:"error"`
			}
			perr := json.Unmarshal(privateConnection.Body, &result)
			if perr != nil {
				err = fmt.Errorf("private connection %s status is %s", id, string(*privateConnection.JSON200.Status))
			} else {
				err = fmt.Errorf("private connection %s status is %s, API error is '%s'", id, string(*privateConnection.JSON200.Status), *result.Error)
			}

			return retry.NonRetryableError(err)
		}

		for _, c := range conditions {
			if err := c(*privateConnection.JSON200); err != nil {
				return retry.RetryableError(err)
			}
		}

		result = *privateConnection.JSON200

		return nil
	}); err != nil {
		return management.PrivateConnection{}, &util.SummaryWithDetailError{
			Summary: fmt.Sprintf("Failed to wait for a private connection %s creation", id),
			Detail:  fmt.Sprintf("Private connection is not ready: %s", err),
		}
	}

	return result, nil
}

func waitConditionAllowList(desiredAllowList string) func(management.PrivateConnection) error {
	return func(c management.PrivateConnection) error {
		if c.AllowList == nil || *c.AllowList != desiredAllowList {
			return fmt.Errorf("private connection %s allow_list is %s, but should be %s", c.PrivateConnectionID, *c.AllowList, desiredAllowList)
		}

		return nil
	}
}

func waitConditionStatus(statuses ...management.PrivateConnectionStatus) func(management.PrivateConnection) error {
	privateConnectionStatusHistory := make([]management.PrivateConnectionStatus, 0, config.PrivateConnectionConsistencyThreshold)

	return func(c management.PrivateConnection) error {
		privateConnectionStatusHistory = append(privateConnectionStatusHistory, *c.Status)

		if !util.Any(statuses, *c.Status) {
			return fmt.Errorf("private connection %s status is %s, but should be %s", c.PrivateConnectionID, *c.Status, util.Join(statuses, ", "))
		}

		if !util.CheckLastN(privateConnectionStatusHistory, config.PrivateConnectionConsistencyThreshold, statuses...) {
			return fmt.Errorf("private connection %s status is %s but the Management API did not return the same status for the consequent %d iterations yet",
				c.PrivateConnectionID, *c.Status, config.PrivateConnectionConsistencyThreshold,
			)
		}

		return nil
	}
}
