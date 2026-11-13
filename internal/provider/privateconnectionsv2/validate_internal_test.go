package privateconnectionsv2

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestValidateInbound(t *testing.T) {
	t.Parallel()

	plan := inboundPlan()
	require.Nil(t, validateConfiguration(plan, true))

	plan.AllowList = types.StringNull()
	err := validateConfiguration(plan, true)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "allow_list configuration is required")

	plan = inboundPlan()
	plan.KaiEndpointID = types.StringValue("vpce-123")
	plan.AllowList = types.StringNull()
	require.Nil(t, validateConfiguration(plan, true))

	plan.AllowList = types.StringValue("123")
	err = validateConfiguration(plan, true)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "not allowed for SingleStore Kai")

	plan = inboundPlan()
	plan.ServiceName = types.StringValue("named")
	require.Nil(t, validateConfiguration(plan, true))
}

func TestValidateOutbound(t *testing.T) {
	t.Parallel()

	plan := inboundPlan()
	plan.Type = types.StringValue("OUTBOUND")
	plan.AllowList = types.StringNull()
	plan.ServiceName = types.StringValue("com.amazonaws.vpce.svc")
	require.Nil(t, validateConfiguration(plan, true))

	plan.AllowList = types.StringValue("123")
	err := validateConfiguration(plan, true)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "not allowed for OUTBOUND")

	plan.AllowList = types.StringNull()
	plan.ServiceName = types.StringNull()
	err = validateConfiguration(plan, true)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "service_name configuration is required")
}

func TestValidateUnknownSkipsRulesUntilApply(t *testing.T) {
	t.Parallel()

	plan := inboundPlan()
	plan.AllowList = types.StringUnknown()
	require.Nil(t, validateConfiguration(plan, false))

	err := validateConfiguration(plan, true)
	require.NotNil(t, err)
	require.Contains(t, err.Summary, "still unknown")

	plan = inboundPlan()
	plan.Type = types.StringValue("OUTBOUND")
	plan.AllowList = types.StringNull()
	plan.ServiceName = types.StringUnknown()
	require.Nil(t, validateConfiguration(plan, false))
	err = validateConfiguration(plan, true)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "service_name configuration is required")
}

func TestValidateServiceNameConfig(t *testing.T) {
	t.Parallel()

	plan := inboundPlan()
	config := plan
	config.ServiceName = types.StringNull()
	require.Nil(t, validateServiceNameConfig(config, plan))

	config.ServiceName = types.StringValue("set")
	err := validateServiceNameConfig(config, plan)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "not allowed for INBOUND")

	plan.Type = types.StringValue("OUTBOUND")
	config.Type = plan.Type
	config.ServiceName = types.StringNull()
	err = validateServiceNameConfig(config, plan)
	require.NotNil(t, err)
	require.Contains(t, err.Detail, "service_name configuration is required")

	config.ServiceName = types.StringUnknown()
	require.Nil(t, validateServiceNameConfig(config, plan))
}

func inboundPlan() privateConnectionModel {
	return privateConnectionModel{
		ClusterID:             types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
		Type:                  types.StringValue("INBOUND"),
		AllowList:             types.StringValue("651246146166"),
		KaiEndpointID:         types.StringNull(),
		ServiceName:           types.StringNull(),
		SQLPort:               types.Float32Value(defaultSQLPort),
		WebsocketsPort:        types.Float32Value(defaultWebsocketPort),
		SupportedCrossRegions: types.ListNull(types.StringType),
	}
}
