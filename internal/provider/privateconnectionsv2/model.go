package privateconnectionsv2

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

const (
	defaultSQLPort       float32 = 3306
	defaultWebsocketPort float32 = 443
)

// privateConnectionModel maps a v2 private connection.
// Ports stay float32 because the Management API schema types them as JSON numbers.
type privateConnectionModel struct {
	ID                    types.String  `tfsdk:"id"`
	ActiveAt              types.String  `tfsdk:"active_at"`
	AllowList             types.String  `tfsdk:"allow_list"`
	KaiEndpointID         types.String  `tfsdk:"kai_endpoint_id"`
	CreatedAt             types.String  `tfsdk:"created_at"`
	DeletedAt             types.String  `tfsdk:"deleted_at"`
	Endpoint              types.String  `tfsdk:"endpoint"`
	OutboundAllowList     types.String  `tfsdk:"outbound_allow_list"`
	ServiceName           types.String  `tfsdk:"service_name"`
	Status                types.String  `tfsdk:"status"`
	SQLPort               types.Float32 `tfsdk:"sql_port"`
	Type                  types.String  `tfsdk:"type"`
	WebsocketsPort        types.Float32 `tfsdk:"web_socket_port"`
	UpdatedAt             types.String  `tfsdk:"updated_at"`
	ClusterID             types.String  `tfsdk:"cluster_id"`
	SupportedCrossRegions types.List    `tfsdk:"supported_cross_regions"`
}

func toPrivateConnectionModel(ctx context.Context, conn management.ClusterPrivateConnection) privateConnectionModel {
	return privateConnectionModel{
		ID:                    util.UUIDStringValue(conn.PrivateConnectionID),
		ActiveAt:              util.MaybeStringValue(conn.ActiveAt),
		AllowList:             util.MaybeStringValue(conn.AllowList),
		KaiEndpointID:         kaiEndpointID(conn),
		CreatedAt:             util.MaybeStringValue(conn.CreatedAt),
		DeletedAt:             util.MaybeStringValue(conn.DeletedAt),
		Endpoint:              util.MaybeStringValue(conn.Endpoint),
		OutboundAllowList:     util.MaybeStringValue(conn.OutboundAllowList),
		ServiceName:           util.MaybeStringValue(conn.ServiceName),
		Status:                util.StringValueOrNull(conn.Status),
		SQLPort:               portValue(conn.SqlPort, defaultSQLPort),
		Type:                  util.StringValueOrNull(conn.Type),
		WebsocketsPort:        portValue(conn.WebsocketsPort, defaultWebsocketPort),
		UpdatedAt:             util.MaybeStringValue(conn.UpdatedAt),
		ClusterID:             util.MaybeUUIDStringValue(conn.ClusterID),
		SupportedCrossRegions: regionsList(ctx, conn.SupportedCrossRegions),
	}
}

// fillOmitted keeps identifiers the v2 API can leave out of a response.
// allow_list, status, and supported_cross_regions are taken from the response
// on every read because they can change outside Terraform.
func fillOmitted(result *privateConnectionModel, prior privateConnectionModel) {
	if result.ClusterID.IsNull() {
		result.ClusterID = nullIfUnknownString(prior.ClusterID)
	}
	if result.KaiEndpointID.IsNull() {
		result.KaiEndpointID = nullIfUnknownString(prior.KaiEndpointID)
	}
	if result.ServiceName.IsNull() {
		result.ServiceName = nullIfUnknownString(prior.ServiceName)
	}
}

// clearUnknown replaces unknown values so a partially stored connection can be saved.
// Terraform rejects unknown values in state.
func clearUnknown(model *privateConnectionModel) {
	model.ActiveAt = nullIfUnknownString(model.ActiveAt)
	model.AllowList = nullIfUnknownString(model.AllowList)
	model.KaiEndpointID = nullIfUnknownString(model.KaiEndpointID)
	model.CreatedAt = nullIfUnknownString(model.CreatedAt)
	model.DeletedAt = nullIfUnknownString(model.DeletedAt)
	model.Endpoint = nullIfUnknownString(model.Endpoint)
	model.OutboundAllowList = nullIfUnknownString(model.OutboundAllowList)
	model.ServiceName = nullIfUnknownString(model.ServiceName)
	model.Status = nullIfUnknownString(model.Status)
	model.Type = nullIfUnknownString(model.Type)
	model.UpdatedAt = nullIfUnknownString(model.UpdatedAt)
	model.ClusterID = nullIfUnknownString(model.ClusterID)
	if model.SQLPort.IsUnknown() {
		model.SQLPort = types.Float32Null()
	}
	if model.WebsocketsPort.IsUnknown() {
		model.WebsocketsPort = types.Float32Null()
	}
	if model.SupportedCrossRegions.IsUnknown() {
		model.SupportedCrossRegions = types.ListNull(types.StringType)
	}
}

func nullIfUnknownString(value types.String) types.String {
	if value.IsUnknown() {
		return types.StringNull()
	}

	return value
}

func kaiEndpointID(conn management.ClusterPrivateConnection) types.String {
	if conn.AllowedPrivateLinkIDs == nil || len(*conn.AllowedPrivateLinkIDs) == 0 {
		return types.StringNull()
	}

	return types.StringValue((*conn.AllowedPrivateLinkIDs)[0])
}

// portValue uses the schema default when the API omits a port.
// An omitted port is the default, and storing null would disagree with the plan.
func portValue(value *float32, fallback float32) types.Float32 {
	if value == nil {
		return types.Float32Value(fallback)
	}

	return types.Float32Value(*value)
}

// regionsList stores no regions as null. An empty API list and an omitted list
// are the same configuration, and a null plan matches an omitted argument.
// The list is sorted so the API's order is not a change.
func regionsList(ctx context.Context, values *[]string) types.List {
	if values == nil || len(*values) == 0 {
		return types.ListNull(types.StringType)
	}

	regions := append([]string(nil), (*values)...)
	sort.Strings(regions)
	list, diags := types.ListValueFrom(ctx, types.StringType, regions)
	if diags.HasError() {
		return types.ListNull(types.StringType)
	}

	return list
}

// normalizeRegions sorts a configured list and stores an empty list as null.
func normalizeRegions(ctx context.Context, list types.List) (types.List, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return list, nil
	}

	var regions []string
	diags := list.ElementsAs(ctx, &regions, false)
	if diags.HasError() {
		return list, diags
	}
	if len(regions) == 0 {
		return types.ListNull(types.StringType), nil
	}

	sort.Strings(regions)
	normalized, listDiags := types.ListValueFrom(ctx, types.StringType, regions)
	diags.Append(listDiags...)

	return normalized, diags
}

// regionStrings returns the configured regions. unknown is true when the list
// cannot be sent to the API yet.
func regionStrings(ctx context.Context, list types.List) ([]string, bool) {
	if list.IsUnknown() {
		return nil, true
	}
	if list.IsNull() {
		return nil, false
	}

	var regions []string
	diags := list.ElementsAs(ctx, &regions, false)
	if diags.HasError() {
		return nil, true
	}

	return regions, false
}
