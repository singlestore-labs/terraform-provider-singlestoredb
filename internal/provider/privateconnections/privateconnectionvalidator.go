package privateconnections

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

var allowListRequiredMsg = "allow_list configuration is required for INBOUND private connections."

type validationRule struct {
	condition bool
	message   string
}

func ValidatePrivateConnection(plan PrivateConnectionModel, isUpdate bool) *util.SummaryWithDetailError {
	privateConnectionType, err := util.PrivateConnectionTypeString(plan.Type)
	if err != nil {
		return &util.SummaryWithDetailError{
			Summary: err.Error(),
			Detail:  err.Error(),
		}
	}

	var rules []validationRule

	switch privateConnectionType {
	case management.PrivateConnectionCreateTypeINBOUND:
		rules = getInboundValidationRules(plan, isUpdate)
	case management.PrivateConnectionCreateTypeOUTBOUND:
		rules = getOutboundValidationRules(plan, isUpdate)
	default:
		return &util.SummaryWithDetailError{
			Summary: "Unknown private connection type.",
			Detail:  fmt.Sprintf("Invalid private connection type %s while it should be %s or %s", privateConnectionType, management.PrivateConnectionCreateTypeINBOUND, management.PrivateConnectionCreateTypeOUTBOUND),
		}
	}

	for _, rule := range rules {
		if rule.condition {
			return &util.SummaryWithDetailError{
				Summary: "Failed to validate private connection configuration",
				Detail:  rule.message,
			}
		}
	}

	return nil
}

func getInboundValidationRules(plan PrivateConnectionModel, isUpdate bool) []validationRule {
	return []validationRule{
		{isUpdate && isUndefined(plan.AllowList), allowListRequiredMsg},
		{isDefined(plan.ServiceName), "service_name configuration is not allowed for INBOUND private connections."},
		{isDefined(plan.KaiEndpointID) && isUndefined(plan.WorkspaceID), "workspace_id configuration is required for SingleStore Kai INBOUND private connections."},
		{isDefined(plan.KaiEndpointID) && isDefined(plan.AllowList), "allow_list configuration is not allowed for SingleStore Kai INBOUND private connections."},
		{isUndefined(plan.KaiEndpointID) && isUndefined(plan.AllowList), allowListRequiredMsg},
	}
}

func getOutboundValidationRules(plan PrivateConnectionModel, isUpdate bool) []validationRule {
	return []validationRule{
		{isUpdate, "OUTBOUND private connections update is not allowed."},
		{isDefined(plan.AllowList), "allow_list configuration is not allowed for OUTBOUND private connections."},
		{isUndefined(plan.ServiceName), "service_name configuration is required for OUTBOUND private connections."},
	}
}

func ValidatePrivateConnectionModifyPlan(plan, state PrivateConnectionModel) *util.SummaryWithDetailError {
	rules := []validationRule{
		{hasChanged(plan.Type, state.Type), "Changing the type configuration is currently not supported."},
		{hasChanged(plan.ServiceName, state.ServiceName), "Changing the service_name configuration is currently not supported."},
		{hasChanged(plan.KaiEndpointID, state.KaiEndpointID), "Changing the kai_endpoint_id configuration is currently not supported."},
		{hasChanged(plan.SQLPort, state.SQLPort), "Changing the sql_port configuration is currently not supported."},
		{hasChanged(plan.WebsocketsPort, state.WebsocketsPort), "Changing the web_socket_port configuration is currently not supported."},
		{hasChanged(plan.WorkspaceGroupID, state.WorkspaceGroupID), "Changing the workspace_group_id configuration is not supported."},
		{hasChanged(plan.WorkspaceID, state.WorkspaceID), "Changing the workspace_id configuration is not supported."},
	}

	for _, rule := range rules {
		if rule.condition {
			return &util.SummaryWithDetailError{
				Summary: "Failed to validate private connection configuration",
				Detail:  rule.message,
			}
		}
	}

	return nil
}

func isDefined(field attr.Value) bool {
	return !field.IsNull() && !field.IsUnknown()
}

func isUndefined(field attr.Value) bool {
	return !isDefined(field)
}

func hasChanged(plan, state attr.Value) bool {
	return !plan.IsNull() && !plan.IsUnknown() && !plan.Equal(state)
}
