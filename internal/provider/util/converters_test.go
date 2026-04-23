package util_test

import (
	"context"
	"testing"

	otypes "github.com/deepmap/oapi-codegen/pkg/types"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

const (
	testUUID1    = "9966fccf-5116-437e-a34f-008ee32e8d94"
	testUUID2    = "458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"
	testUUID3    = "283d4b0d-b0d6-485a-bc2d-a763c523c68a"
	testEmail1   = "alice@example.com"
	testEmail2   = "bob@example.com"
	testEmail3   = "carol@example.com"
	invalidUUID  = "not-a-uuid"
	invalidEmail = "not-an-email"
)

func TestMaybeString(t *testing.T) {
	require.Nil(t, util.MaybeString(types.StringNull()))
	require.Nil(t, util.MaybeString(types.StringUnknown()))
	s := "bar"
	require.Equal(t, &s, util.MaybeString(types.StringValue(s)))
}

func TestToString(t *testing.T) {
	require.Empty(t, util.ToString(types.StringNull()))
	require.Empty(t, util.ToString(types.StringUnknown()))
	s := "buzz"
	require.Equal(t, s, util.ToString(types.StringValue(s)))
}

func TestMaybeStringValue(t *testing.T) {
	require.Equal(t, types.StringNull(), util.MaybeStringValue(nil))
	s := "fizz"
	require.Equal(t, types.StringValue(s), util.MaybeStringValue(&s))
}

func TestMaybeBool(t *testing.T) {
	require.Nil(t, util.MaybeBool(types.BoolNull()))
	require.Nil(t, util.MaybeBool(types.BoolUnknown()))
	require.True(t, util.Deref(util.MaybeBool(types.BoolValue(true))))
}

func TestMaybeBoolValue(t *testing.T) {
	require.Equal(t, types.BoolNull(), util.MaybeBoolValue(nil))
	require.Equal(t, types.BoolValue(true), util.MaybeBoolValue(util.Ptr(true)))
}

func TestUUIDStringValue(t *testing.T) {
	require.Equal(t, types.StringValue(testUUID1), util.UUIDStringValue(uuid.MustParse(testUUID1)))
}

func TestStringFirewallRanges(t *testing.T) {
	a := "192.168.5.10/24"
	b := "192.168.5.10/32"
	result := util.StringFirewallRanges([]types.String{types.StringValue(a), types.StringValue(b)})
	require.Equal(t, []string{a, b}, result)
}

func TestFirewallRanges(t *testing.T) {
	a := "192.168.5.10/24"
	b := "192.168.5.10/32"
	result := util.FirewallRanges(nil)
	require.Empty(t, result)
	result = util.FirewallRanges(util.Ptr([]string{a, b}))
	require.Equal(t, []types.String{types.StringValue(a), types.StringValue(b)}, result)
}

func TestWorkspaceGroupStateStringValue(t *testing.T) {
	state := management.WorkspaceGroupStateACTIVE
	require.Equal(t, string(state), util.WorkspaceGroupStateStringValue(state).ValueString())
}

func TestWorkspaceStateString(t *testing.T) {
	require.Nil(t, util.WorkspaceStateString(types.StringValue("something")))
	active := string(management.WorkspaceStateACTIVE)
	require.Equal(t, management.WorkspaceStateACTIVE, util.Deref(util.WorkspaceStateString(types.StringValue(active))))
}

func TestWorkspaceStateStringValue(t *testing.T) {
	state := management.WorkspaceStateACTIVE
	require.Equal(t, string(state), util.WorkspaceStateStringValue(state).ValueString())
}

func mustUUIDSet(t *testing.T, ids ...string) types.Set {
	t.Helper()

	elems := make([]attr.Value, len(ids))
	for i, id := range ids {
		elems[i] = types.StringValue(id)
	}

	set, diags := types.SetValue(types.StringType, elems)
	require.False(t, diags.HasError(), "failed to build UUID set: %v", diags)

	return set
}

func mustEmailSet(t *testing.T, emails ...string) types.Set {
	t.Helper()

	elems := make([]attr.Value, len(emails))
	for i, e := range emails {
		elems[i] = types.StringValue(e)
	}

	set, diags := types.SetValue(types.StringType, elems)
	require.False(t, diags.HasError(), "failed to build email set: %v", diags)

	return set
}

func TestParseUUIDSets_AllNew(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustUUIDSet(t, testUUID1, testUUID2)
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.ElementsMatch(t, []otypes.UUID{uuid.MustParse(testUUID1), uuid.MustParse(testUUID2)}, got)
}

func TestParseUUIDSets_DifferenceFiltersOverlap(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustUUIDSet(t, testUUID1, testUUID2, testUUID3)
	b := mustUUIDSet(t, testUUID2)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.ElementsMatch(t, []otypes.UUID{uuid.MustParse(testUUID1), uuid.MustParse(testUUID3)}, got)
}

func TestParseUUIDSets_FullOverlapReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustUUIDSet(t, testUUID1, testUUID2)
	b := mustUUIDSet(t, testUUID1, testUUID2)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Empty(t, got)
}

func TestParseUUIDSets_EmptyAReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustUUIDSet(t)
	b := mustUUIDSet(t, testUUID1)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Empty(t, got)
}

func TestParseUUIDSets_ANullReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := types.SetNull(types.StringType)
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Empty(t, got)
}

func TestParseUUIDSets_AUnknownReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := types.SetUnknown(types.StringType)
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Empty(t, got)
}

func TestParseUUIDSets_BNullTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustUUIDSet(t, testUUID1)
	b := types.SetNull(types.StringType)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Equal(t, []otypes.UUID{uuid.MustParse(testUUID1)}, got)
}

func TestParseUUIDSets_BUnknownTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustUUIDSet(t, testUUID1)
	b := types.SetUnknown(types.StringType)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Equal(t, []otypes.UUID{uuid.MustParse(testUUID1)}, got)
}

func TestParseUUIDSets_InvalidUUIDReturnsError(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustUUIDSet(t, testUUID1, invalidUUID)
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.Error(t, err)
	require.Empty(t, got)
	require.Contains(t, err.Error(), "invalid UUID")
}

func TestParseUUIDSets_InvalidUUIDIgnoredWhenInB(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	// Malformed UUIDs in a should be skipped (not parsed) when present in b,
	// because they are being filtered out of the set difference.
	a := mustUUIDSet(t, invalidUUID, testUUID1)
	b := mustUUIDSet(t, invalidUUID)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Equal(t, []otypes.UUID{uuid.MustParse(testUUID1)}, got)
}

func TestValidateAndMapUserEmails_AllNew(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, testEmail1, testEmail2)
	b := mustEmailSet(t)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.ElementsMatch(t, []string{testEmail1, testEmail2}, got)
}

func TestValidateAndMapUserEmails_DifferenceFiltersOverlap(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, testEmail1, testEmail2, testEmail3)
	b := mustEmailSet(t, testEmail2)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.ElementsMatch(t, []string{testEmail1, testEmail3}, got)
}

func TestValidateAndMapUserEmails_FullOverlapReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, testEmail1, testEmail2)
	b := mustEmailSet(t, testEmail1, testEmail2)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Empty(t, got)
}

func TestValidateAndMapUserEmails_EmptyAReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t)
	b := mustEmailSet(t, testEmail2)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Empty(t, got)
}

func TestValidateAndMapUserEmails_BNullTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, testEmail1)
	b := types.SetNull(types.StringType)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Equal(t, []string{testEmail1}, got)
}

func TestValidateAndMapUserEmails_BUnknownTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, testEmail1)
	b := types.SetUnknown(types.StringType)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Equal(t, []string{testEmail1}, got)
}

func TestValidateAndMapUserEmails_InvalidEmailReturnsError(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, testEmail1, invalidEmail)
	b := mustEmailSet(t)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.Error(t, err)
	require.Empty(t, got)
	require.Contains(t, err.Error(), "invalid email address")
	require.Contains(t, err.Error(), invalidEmail)
}

func TestValidateAndMapUserEmails_InvalidEmailIgnoredWhenInB(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	// Even if the email is malformed, it should be skipped (not validated)
	// when present in b, because it is being filtered out of the difference.
	a := mustEmailSet(t, invalidEmail, testEmail1)
	b := mustEmailSet(t, invalidEmail)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.Equal(t, []string{testEmail1}, got)
}
