package util

import (
	"context"
	"fmt"
	"strings"
	"time"

	otypes "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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

func IsConfiguredString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func MaybeStringValue(s *string) types.String {
	return maybeElse(s, types.StringValue, types.StringNull)
}

func MaybeTimeValue(s *time.Time) types.String {
	if s == nil {
		return types.StringNull()
	}

	return types.StringValue(s.Format(time.RFC3339))
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

func MaybeUUIDStringListValue(ids *[]otypes.UUID) []types.String {
	if ids == nil {
		return []types.String{}
	}

	result := make([]types.String, len(*ids))
	for i, id := range *ids {
		idCopy := id
		result[i] = MaybeUUIDStringValue(&idCopy)
	}

	return result
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

func PrivateConnectionTypeString(wgs types.String) (management.PrivateConnectionCreateType, error) {
	for _, s := range []management.PrivateConnectionCreateType{
		management.PrivateConnectionCreateTypeINBOUND,
		management.PrivateConnectionCreateTypeOUTBOUND,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return s, nil
		}
	}

	return "", fmt.Errorf("invalid private connection type '%s'", wgs)
}

func MaybeFloat32(f types.Float32) *float32 {
	if f.IsNull() || f.IsUnknown() {
		return nil
	}

	return Ptr(f.ValueFloat32())
}

func WorkspaceAutoScaleSensitivityString(wgs types.String) *management.AutoScaleSensitivity {
	for _, s := range []management.AutoScaleSensitivity{
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

func WorkspaceCreateAutoSuspendSuspendTypeString(wgs types.String) *management.WorkspaceCreateAutoSuspendSuspendType {
	for _, s := range []management.WorkspaceCreateAutoSuspendSuspendType{
		management.WorkspaceCreateAutoSuspendSuspendTypeIDLE,
		management.WorkspaceCreateAutoSuspendSuspendTypeDISABLED,
		management.WorkspaceCreateAutoSuspendSuspendTypeSCHEDULED,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return &s
		}
	}

	return nil
}

func WorkspaceUpdateAutoSuspendSuspendTypeString(wgs types.String) *management.WorkspaceUpdateAutoSuspendSuspendType {
	for _, s := range []management.WorkspaceUpdateAutoSuspendSuspendType{
		management.IDLE,
		management.DISABLED,
		management.SCHEDULED,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return &s
		}
	}

	return nil
}

func WorkspaceGroupCreateDeploymentTypeString(wgs types.String) *management.WorkspaceGroupCreateDeploymentType {
	for _, s := range []management.WorkspaceGroupCreateDeploymentType{
		management.WorkspaceGroupCreateDeploymentTypePRODUCTION,
		management.WorkspaceGroupCreateDeploymentTypeNONPRODUCTION,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return &s
		}
	}

	return nil
}

func WorkspaceGroupUpdateDeploymentTypeString(wgs types.String) *management.WorkspaceGroupUpdateDeploymentType {
	for _, s := range []management.WorkspaceGroupUpdateDeploymentType{
		management.WorkspaceGroupUpdateDeploymentTypePRODUCTION,
		management.WorkspaceGroupUpdateDeploymentTypeNONPRODUCTION,
	} {
		if strings.EqualFold(wgs.ValueString(), string(s)) {
			return &s
		}
	}

	return nil
}

func WorkspaceGroupCloudProviderString(provider string) *management.CloudProvider {
	for _, s := range []management.CloudProvider{
		management.CloudProviderAWS,
		management.CloudProviderAzure,
		management.CloudProviderGCP,
	} {
		if strings.EqualFold(provider, string(s)) {
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

// SetDifference computes the set difference (a - b) of two string sets.
// Null or unknown a yields an empty result; null or unknown b is treated as empty.
func SetDifference(ctx context.Context, a, b types.Set, diags *diag.Diagnostics) []string {
	if a.IsNull() || a.IsUnknown() {
		return []string{}
	}

	bSet := make(map[string]struct{}, len(b.Elements()))
	if !b.IsNull() && !b.IsUnknown() {
		bElems := make([]types.String, 0, len(b.Elements()))
		diags.Append(b.ElementsAs(ctx, &bElems, false)...)
		for _, bElem := range bElems {
			bSet[bElem.ValueString()] = struct{}{}
		}
	}

	result := make([]string, 0, len(a.Elements()))
	for _, aElem := range a.Elements() {
		s := aElem.(types.String).ValueString()
		if _, exists := bSet[s]; exists {
			continue
		}
		result = append(result, s)
	}

	return result
}

// ValidateSet applies convert to each element in a, returning the mapped values or the first
// conversion error encountered.
func ValidateSet[T any](a []string, convert func(string) (T, error)) ([]T, error) {
	result := make([]T, 0, len(a))
	for _, elem := range a {
		v, err := convert(elem)
		if err != nil {
			return []T{}, err
		}
		result = append(result, v)
	}

	return result, nil
}

// ValidateUUIDDiff computes the set difference (a - b) of UUID string sets and
// parses each resulting value into a UUID, returning the parsed UUIDs or the first parsing error encountered.
func ValidateUUIDDiff(ctx context.Context, a, b types.Set, diags *diag.Diagnostics) ([]otypes.UUID, error) {
	diff := SetDifference(ctx, a, b, diags)

	return ValidateSet(diff, uuid.Parse)
}

// ValidateUserEmailDiff computes the set difference (a - b) of email sets and
// validates each resulting email, returning the validated emails or the first
// validation error encountered.
func ValidateUserEmailDiff(ctx context.Context, a, b types.Set, diags *diag.Diagnostics) ([]string, error) {
	diff := SetDifference(ctx, a, b, diags)

	return ValidateSet(diff, IsValidEmail)
}
