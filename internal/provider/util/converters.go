package util

import (
	"strings"

	otypes "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
)

func MaybeString(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}

	return Ptr(s.ValueString())
}

func ToString(s types.String) string {
	return s.ValueString()
}

func MaybeStringValue(s *string) types.String {
	return maybeElse(s, types.StringValue, types.StringNull)
}

func MaybeBool(b types.Bool) *bool {
	if b.IsNull() || b.IsUnknown() {
		return nil
	}

	return Ptr(b.ValueBool())
}

func MaybeBoolValue(b *bool) types.Bool {
	return maybeElse(b, types.BoolValue, types.BoolNull)
}

func UUIDStringValue(id otypes.UUID) types.String {
	return types.StringValue(id.String())
}

func MaybeUUIDStringValue(id *otypes.UUID) types.String {
	if id == nil {
		return types.StringNull()
	}

	return types.StringValue(id.String())
}

func StringFirewallRanges(frs []types.String) []string {
	return Map(frs, ToString)
}

func FirewallRanges(frs *[]string) []types.String {
	return Map(Deref(frs), types.StringValue)
}

func WorkspaceGroupStateStringValue(wgs management.WorkspaceGroupState) types.String {
	return types.StringValue(string(wgs))
}

func WorkspaceStateString(wgs types.String) *management.WorkspaceState {
	for _, s := range []management.WorkspaceState{
		management.WorkspaceStateACTIVE,
		management.WorkspaceStateFAILED,
		management.WorkspaceStatePENDING,
		management.WorkspaceStateSUSPENDED,
		management.WorkspaceStateTERMINATED,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return &s
		}
	}

	return nil
}

func WorkspaceStateStringValue(ws management.WorkspaceState) types.String {
	return types.StringValue(string(ws))
}

func maybeElse[A, B any](input *A, convert func(A) B, create func() B) B {
	if input == nil {
		return create()
	}

	return convert(*input)
}

func PrivateConnectionTypeString(wgs types.String) *management.PrivateConnectionCreateType {
	for _, s := range []management.PrivateConnectionCreateType{
		management.PrivateConnectionCreateTypeINBOUND,
		management.PrivateConnectionCreateTypeOUTBOUND,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return &s
		}
	}

	return nil
}

func StringValueOrNull[T ~string](value *T) types.String {
	if value == nil {
		return types.StringNull()
	}

	return types.StringValue(string(*value))
}

func MaybeFloat32(f types.Float32) *float32 {
	if f.IsNull() || f.IsUnknown() {
		return nil
	}

	return Ptr(f.ValueFloat32())
}

func WorkspaceAutoScaleSensitivityString(wgs types.String) *management.WorkspaceUpdateAutoScaleSensitivity {
	for _, s := range []management.WorkspaceUpdateAutoScaleSensitivity{
		management.LOW,
		management.NORMAL,
		management.HIGH,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return &s
		}
	}

	return nil
}

func StringValueOrNull[T ~string](value *T) types.String {
	if value == nil {
		return types.StringNull()
	}

	return types.StringValue(string(*value))
}
