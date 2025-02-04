package privateconnections_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/privateconnections"
	"github.com/stretchr/testify/require"
)

func TestValidatePrivateConnection(t *testing.T) {
	plan := privateconnections.PrivateConnectionModel{
		Type:             types.StringValue("INBOUND"),
		WorkspaceGroupID: types.StringValue("721cdcaf-3555-4434-8f2e-d4e77d9a5d25"),
	}

	// INBOUND

	err := privateconnections.ValidatePrivateConnection(plan, false)
	require.NotNil(t, err)
	require.Equal(t, "allow_list configuration is required for INBOUND private connections.", err.Detail)

	err = privateconnections.ValidatePrivateConnection(plan, true)
	require.NotNil(t, err)
	require.Equal(t, "allow_list configuration is required for INBOUND private connections.", err.Detail)

	plan.AllowList = types.StringValue("12345")

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.Nil(t, err)

	err = privateconnections.ValidatePrivateConnection(plan, true)
	require.Nil(t, err)

	plan.ServiceName = types.StringValue("test service name")

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.NotNil(t, err)
	require.Equal(t, "service_name configuration is not allowed for INBOUND private connections.", err.Detail)

	plan.ServiceName = types.StringNull()
	plan.KaiEndpointID = types.StringValue("vce-test")
	plan.WorkspaceID = types.StringValue("vce-testfa3d7868-fd40-40b5-8001-4b73de5d94c1")

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.NotNil(t, err)
	require.Equal(t, "allow_list configuration is not allowed for SingleStore Kai INBOUND private connections.", err.Detail)

	plan.WorkspaceID = types.StringNull()

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.NotNil(t, err)
	require.Equal(t, "workspace_id configuration is required for SingleStore Kai INBOUND private connections.", err.Detail)

	// OUTBOUND

	plan = privateconnections.PrivateConnectionModel{
		Type:             types.StringValue("OUTBOUND"),
		WorkspaceGroupID: types.StringValue("721cdcaf-3555-4434-8f2e-d4e77d9a5d25"),
	}

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.NotNil(t, err)
	require.Equal(t, "service_name configuration is required for OUTBOUND private connections.", err.Detail)

	plan.ServiceName = types.StringValue("test service name")

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.Nil(t, err)

	plan.AllowList = types.StringValue("12345")

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.NotNil(t, err)
	require.Equal(t, "allow_list configuration is not allowed for OUTBOUND private connections.", err.Detail)

	plan.AllowList = types.StringNull()

	err = privateconnections.ValidatePrivateConnection(plan, false)
	require.Nil(t, err)

	err = privateconnections.ValidatePrivateConnection(plan, true)
	require.NotNil(t, err)
	require.Equal(t, "OUTBOUND private connections update is not allowed.", err.Detail)
}

func TestValidatePrivateConnectionModifyPlan(t *testing.T) {
	plan := privateconnections.PrivateConnectionModel{
		Type:             types.StringValue("INBOUND"),
		WorkspaceGroupID: types.StringValue("721cdcaf-3555-4434-8f2e-d4e77d9a5d25"),
		AllowList:        types.StringValue("123345"),
	}

	state := privateconnections.PrivateConnectionModel{
		Type:             types.StringValue("INBOUND"),
		AllowList:        types.StringValue("111111"),
		ServiceName:      types.StringValue("service name"),
		KaiEndpointID:    types.StringValue("kai-vcpe"),
		SQLPort:          types.Float32Value(3306),
		WebsocketsPort:   types.Float32Value(443.0),
		WorkspaceGroupID: types.StringValue("721cdcaf-3555-4434-8f2e-d4e77d9a5d25"),
		WorkspaceID:      types.StringValue("41c1c310-9a5f-4a7a-ba8e-088af6056d8d"),
	}

	err := privateconnections.ValidatePrivateConnectionModifyPlan(plan, state)
	require.Nil(t, err)

	plan.Type = types.StringValue("OUTBOUND")

	err = privateconnections.ValidatePrivateConnectionModifyPlan(plan, state)
	require.NotNil(t, err)
	require.Equal(t, "Changing the type configuration is currently not supported.", err.Detail)

	plan.Type = types.StringValue("INBOUND")

	plan.ServiceName = types.StringValue("updated service name")

	err = privateconnections.ValidatePrivateConnectionModifyPlan(plan, state)
	require.NotNil(t, err)
	require.Equal(t, "Changing the service_name configuration is currently not supported.", err.Detail)

	plan.ServiceName = types.StringNull()
	plan.WorkspaceID = types.StringValue("06291fc6-1dd5-495f-b9aa-90bd5961dd65")

	err = privateconnections.ValidatePrivateConnectionModifyPlan(plan, state)
	require.NotNil(t, err)
	require.Equal(t, "Changing the workspace_id configuration is not supported.", err.Detail)

	plan.WorkspaceID = types.StringNull()
	plan.WorkspaceGroupID = types.StringValue("06291fc6-1dd5-495f-b9aa-90bd5961dd65")

	err = privateconnections.ValidatePrivateConnectionModifyPlan(plan, state)
	require.NotNil(t, err)
	require.Equal(t, "Changing the workspace_group_id configuration is not supported.", err.Detail)

	plan.WorkspaceGroupID = types.StringNull()
	err = privateconnections.ValidatePrivateConnectionModifyPlan(plan, state)
	require.Nil(t, err)
}
