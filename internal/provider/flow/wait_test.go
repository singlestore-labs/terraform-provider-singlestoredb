package flow_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/flow"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func readyFlowInstance() management.Flow {
	return management.Flow{
		FlowID:       uuid.New(),
		Status:       util.Ptr("Running"),
		UserName:     util.Ptr("admin"),
		DatabaseName: util.Ptr("my_database"),
		Endpoint:     util.Ptr("example.com"),
	}
}

func TestWaitConditionReady(t *testing.T) {
	t.Parallel()

	t.Run("passes after consistent ready polls", func(t *testing.T) {
		t.Parallel()

		condition := flow.WaitConditionReadyForTest()
		instance := readyFlowInstance()

		for i := range config.FlowInstanceConsistencyThreshold {
			err := condition(instance)
			if i < config.FlowInstanceConsistencyThreshold-1 {
				require.ErrorContains(t, err, "readiness check did not pass consistently")
			} else {
				require.NoError(t, err)
			}
		}
	})

	t.Run("status not running", func(t *testing.T) {
		t.Parallel()

		condition := flow.WaitConditionReadyForTest()
		instance := readyFlowInstance()
		instance.Status = util.Ptr("Pending")

		err := condition(instance)
		require.ErrorContains(t, err, `status is "Pending", expected "Running"`)
	})

	t.Run("user name not available", func(t *testing.T) {
		t.Parallel()

		condition := flow.WaitConditionReadyForTest()
		instance := readyFlowInstance()
		instance.UserName = util.Ptr("Unknown")

		err := condition(instance)
		require.ErrorContains(t, err, "user name is not yet available")
	})

	t.Run("database name not available", func(t *testing.T) {
		t.Parallel()

		condition := flow.WaitConditionReadyForTest()
		instance := readyFlowInstance()
		instance.DatabaseName = util.Ptr("unknown")

		err := condition(instance)
		require.ErrorContains(t, err, "database name is not yet available")
	})

	t.Run("endpoint not available", func(t *testing.T) {
		t.Parallel()

		condition := flow.WaitConditionReadyForTest()
		instance := readyFlowInstance()
		instance.Endpoint = util.Ptr("")

		err := condition(instance)
		require.ErrorContains(t, err, "endpoint is not yet available")
	})
}

func TestMergeFlowCreateOnlyPlanFields(t *testing.T) {
	t.Parallel()

	t.Run("preserves plan config when state fields are null", func(t *testing.T) {
		t.Parallel()

		result := flow.MergeFlowCreateOnlyPlanFieldsForTest(
			flow.FlowCreateOnlyPlanFields{
				UserName:     types.StringValue("admin"),
				DatabaseName: types.StringValue("my_database"),
			},
			flow.FlowCreateOnlyPlanFields{
				UserName:     types.StringNull(),
				DatabaseName: types.StringNull(),
			},
		)

		require.Equal(t, "admin", result.UserName.ValueString())
		require.Equal(t, "my_database", result.DatabaseName.ValueString())
	})

	t.Run("adopts state values for drift suppression", func(t *testing.T) {
		t.Parallel()

		result := flow.MergeFlowCreateOnlyPlanFieldsForTest(
			flow.FlowCreateOnlyPlanFields{
				UserName:     types.StringValue("different-user"),
				DatabaseName: types.StringValue("different-db"),
			},
			flow.FlowCreateOnlyPlanFields{
				UserName:     types.StringValue("admin"),
				DatabaseName: types.StringValue("my_database"),
			},
		)

		require.Equal(t, "admin", result.UserName.ValueString())
		require.Equal(t, "my_database", result.DatabaseName.ValueString())
	})
}
