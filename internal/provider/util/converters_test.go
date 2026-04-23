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
	id := "9966fccf-5116-437e-a34f-008ee32e8d94" //nolint:goconst // test ID
	require.Equal(t, types.StringValue(id), util.UUIDStringValue(uuid.MustParse(id)))
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
	require.False(t, diags.HasError(), "failed to build UUID set: %s", diags)

	return set
}

func mustEmailSet(t *testing.T, emails ...string) types.Set {
	t.Helper()

	elems := make([]attr.Value, len(emails))
	for i, e := range emails {
		elems[i] = types.StringValue(e)
	}

	set, diags := types.SetValue(types.StringType, elems)
	require.False(t, diags.HasError(), "failed to build email set: %s", diags)

	return set
}

func TestParseUUIDSets_AllNew(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"
	id2 := "458d14e6-fcc4-4985-a2a6-f1f1f15cef2f" //nolint:goconst

	a := mustUUIDSet(t, id1, id2)
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.ElementsMatch(t, []otypes.UUID{uuid.MustParse(id1), uuid.MustParse(id2)}, *got)
}

func TestParseUUIDSets_DifferenceFiltersOverlap(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"
	id2 := "458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"
	id3 := "283d4b0d-b0d6-485a-bc2d-a763c523c68a"

	a := mustUUIDSet(t, id1, id2, id3)
	b := mustUUIDSet(t, id2)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.ElementsMatch(t, []otypes.UUID{uuid.MustParse(id1), uuid.MustParse(id3)}, *got)
}

func TestParseUUIDSets_FullOverlapReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"
	id2 := "458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"

	a := mustUUIDSet(t, id1, id2)
	b := mustUUIDSet(t, id1, id2)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Empty(t, *got)
}

func TestParseUUIDSets_EmptyAReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"

	a := mustUUIDSet(t)
	b := mustUUIDSet(t, id1)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Empty(t, *got)
}

func TestParseUUIDSets_ANullReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := types.SetNull(types.StringType)
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Empty(t, *got)
}

func TestParseUUIDSets_AUnknownReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := types.SetUnknown(types.StringType)
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Empty(t, *got)
}

func TestParseUUIDSets_BNullTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"

	a := mustUUIDSet(t, id1)
	b := types.SetNull(types.StringType)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Equal(t, []otypes.UUID{uuid.MustParse(id1)}, *got)
}

func TestParseUUIDSets_BUnknownTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"

	a := mustUUIDSet(t, id1)
	b := types.SetUnknown(types.StringType)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Equal(t, []otypes.UUID{uuid.MustParse(id1)}, *got)
}

func TestParseUUIDSets_InvalidUUIDReturnsError(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"

	a := mustUUIDSet(t, id1, "not-a-uuid")
	b := mustUUIDSet(t)

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "invalid UUID")
}

func TestParseUUIDSets_InvalidUUIDIgnoredWhenInB(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	id1 := "9966fccf-5116-437e-a34f-008ee32e8d94"

	// Malformed UUIDs in a should be skipped (not parsed) when present in b,
	// because they are being filtered out of the set difference.
	a := mustUUIDSet(t, "not-a-uuid", id1)
	b := mustUUIDSet(t, "not-a-uuid")

	got, err := util.ParseUUIDSets(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Equal(t, []otypes.UUID{uuid.MustParse(id1)}, *got)
}

func TestValidateAndMapUserEmails_AllNew(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, "alice@example.com", "bob@example.com")
	b := mustEmailSet(t)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.ElementsMatch(t, []string{"alice@example.com", "bob@example.com"}, *got)
}

func TestValidateAndMapUserEmails_DifferenceFiltersOverlap(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, "alice@example.com", "bob@example.com", "carol@example.com")
	b := mustEmailSet(t, "bob@example.com")

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.ElementsMatch(t, []string{"alice@example.com", "carol@example.com"}, *got)
}

func TestValidateAndMapUserEmails_FullOverlapReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, "alice@example.com", "bob@example.com")
	b := mustEmailSet(t, "alice@example.com", "bob@example.com")

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Empty(t, *got)
}

func TestValidateAndMapUserEmails_EmptyAReturnsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t)
	b := mustEmailSet(t, "bob@example.com")

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Empty(t, *got)
}

func TestValidateAndMapUserEmails_BNullTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, "alice@example.com")
	b := types.SetNull(types.StringType)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Equal(t, []string{"alice@example.com"}, *got)
}

func TestValidateAndMapUserEmails_BUnknownTreatedAsEmpty(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, "alice@example.com")
	b := types.SetUnknown(types.StringType)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Equal(t, []string{"alice@example.com"}, *got)
}

func TestValidateAndMapUserEmails_InvalidEmailReturnsError(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	a := mustEmailSet(t, "alice@example.com", "not-an-email")
	b := mustEmailSet(t)

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.Error(t, err)
	require.Nil(t, got)
	require.Contains(t, err.Error(), "invalid email address")
	require.Contains(t, err.Error(), "not-an-email")
}

func TestValidateAndMapUserEmails_InvalidEmailIgnoredWhenInB(t *testing.T) {
	ctx := context.Background()
	diags := diag.Diagnostics{}

	// Even if the email is malformed, it should be skipped (not validated)
	// when present in b, because it is being filtered out of the difference.
	a := mustEmailSet(t, "not-an-email", "alice@example.com")
	b := mustEmailSet(t, "not-an-email")

	got, err := util.ValidateAndMapUserEmails(ctx, a, b, &diags)
	require.NoError(t, err)
	require.False(t, diags.HasError())
	require.NotNil(t, got)
	require.Equal(t, []string{"alice@example.com"}, *got)
}
