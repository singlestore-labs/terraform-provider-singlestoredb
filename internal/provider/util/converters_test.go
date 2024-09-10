package util_test

import (
	"testing"

	"github.com/google/uuid"
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
	id := "9966fccf-5116-437e-a34f-008ee32e8d94"
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
