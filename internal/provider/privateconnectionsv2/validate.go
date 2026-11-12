package privateconnectionsv2

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const validationSummary = "Failed to validate private connection configuration"

const stillUnknownDetail = "Run terraform plan again so every argument is known before apply."

// validateConfiguration checks type-specific arguments.
// Unknown values are skipped: unknown is neither set nor empty. atApply reports
// those values because apply is the last time they can be checked before the API call.
func validateConfiguration(plan privateConnectionModel, atApply bool) *util.SummaryWithDetailError {
	if configurationIsUnknown(plan) {
		if atApply {
			return unknownConfiguration()
		}

		return nil
	}

	connectionType, err := connectionTypeValue(plan.Type)
	if err != nil {
		return err
	}

	switch connectionType {
	case management.PrivateConnectionCreateV2TypeINBOUND:
		return validateInbound(plan, atApply)
	case management.PrivateConnectionCreateV2TypeOUTBOUND:
		return validateOutbound(plan, atApply)
	default:
		return &util.SummaryWithDetailError{
			Summary: "Unknown private connection type.",
			Detail: fmt.Sprintf("Invalid private connection type %s while it should be %s or %s",
				plan.Type.ValueString(), management.PrivateConnectionCreateV2TypeINBOUND, management.PrivateConnectionCreateV2TypeOUTBOUND),
		}
	}
}

func configurationIsUnknown(plan privateConnectionModel) bool {
	return plan.Type.IsUnknown() || plan.ClusterID.IsUnknown() || plan.AllowList.IsUnknown() ||
		plan.KaiEndpointID.IsUnknown() || plan.SQLPort.IsUnknown() || plan.WebsocketsPort.IsUnknown() ||
		plan.SupportedCrossRegions.IsUnknown()
}

func validateInbound(plan privateConnectionModel, atApply bool) *util.SummaryWithDetailError {
	// service_name is checked against configuration. The plan also carries the name
	// the API assigned, and that value is not a user setting.
	switch {
	case isDefined(plan.KaiEndpointID) && isDefined(plan.AllowList):
		return invalidConfig("allow_list configuration is not allowed for SingleStore Kai INBOUND private connections.")
	case isDefined(plan.KaiEndpointID):
		return nil
	case isDefined(plan.AllowList):
		return nil
	case plan.KaiEndpointID.IsUnknown() || plan.AllowList.IsUnknown():
		if atApply {
			return unknownConfiguration()
		}

		return nil
	default:
		return invalidConfig("allow_list configuration is required for INBOUND private connections.")
	}
}

func validateOutbound(plan privateConnectionModel, atApply bool) *util.SummaryWithDetailError {
	if isDefined(plan.AllowList) {
		return invalidConfig("allow_list configuration is not allowed for OUTBOUND private connections.")
	}
	if plan.ServiceName.IsUnknown() {
		if atApply {
			return invalidConfig("service_name configuration is required for OUTBOUND private connections.")
		}

		return nil
	}
	if !isDefined(plan.ServiceName) {
		return invalidConfig("service_name configuration is required for OUTBOUND private connections.")
	}

	return nil
}

func connectionTypeValue(value types.String) (management.PrivateConnectionCreateV2Type, *util.SummaryWithDetailError) {
	parsed := management.PrivateConnectionCreateV2Type(value.ValueString())
	if parsed.Valid() {
		return parsed, nil
	}

	return "", &util.SummaryWithDetailError{
		Summary: "Unknown private connection type.",
		Detail: fmt.Sprintf("Invalid private connection type %s while it should be %s or %s",
			value.ValueString(), management.PrivateConnectionCreateV2TypeINBOUND, management.PrivateConnectionCreateV2TypeOUTBOUND),
	}
}

func invalidConfig(detail string) *util.SummaryWithDetailError {
	return &util.SummaryWithDetailError{Summary: validationSummary, Detail: detail}
}

func unknownConfiguration() *util.SummaryWithDetailError {
	return &util.SummaryWithDetailError{
		Summary: "Private connection configuration is still unknown",
		Detail:  stillUnknownDetail,
	}
}

// validateServiceNameConfig uses the configuration, not the plan. An omitted
// service_name is null in config and unknown in the plan because the attribute is computed.
func validateServiceNameConfig(config, plan privateConnectionModel) *util.SummaryWithDetailError {
	if !util.IsConfiguredString(plan.Type) {
		return nil
	}

	connectionType, err := connectionTypeValue(plan.Type)
	if err != nil {
		return err
	}

	switch connectionType {
	case management.PrivateConnectionCreateV2TypeINBOUND:
		if isDefined(config.ServiceName) {
			return invalidConfig("service_name configuration is not allowed for INBOUND private connections.")
		}
	case management.PrivateConnectionCreateV2TypeOUTBOUND:
		if config.ServiceName.IsNull() {
			return invalidConfig("service_name configuration is required for OUTBOUND private connections.")
		}
	default:
		return nil
	}

	return nil
}

func isDefined(field attr.Value) bool {
	return !field.IsNull() && !field.IsUnknown()
}
