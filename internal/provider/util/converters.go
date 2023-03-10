package util

import (
	otypes "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
)

func MaybeString(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}

	result := s.ValueString()
	return &result
}

func MaybeStringValue(s *string) types.String {
	return maybeElse(s, types.StringValue, types.StringNull)
}

func MaybeBoolValue(b *bool) types.Bool {
	return maybeElse(b, types.BoolValue, types.BoolNull)
}

func UUIDStringValue(id otypes.UUID) types.String {
	return types.StringValue(id.String())
}

func FirewallRanges(frs *[]string) []types.String {
	if frs == nil {
		return nil
	}

	result := []types.String{}
	for _, fr := range *frs {
		result = append(result, types.StringValue(fr))
	}

	return result
}

func WorkspaceGroupStateStringValue(wgs management.WorkspaceGroupState) types.String {
	return types.StringValue(string(wgs))
}

func maybeElse[A, B any](input *A, convert func(A) B, create func() B) B {
	if input == nil {
		return create()
	}

	return convert(*input)
}
