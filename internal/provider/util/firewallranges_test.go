package util_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func ranges(ss ...string) []types.String {
	result := make([]types.String, 0, len(ss))
	for _, s := range ss {
		result = append(result, types.StringValue(s))
	}

	return result
}

func TestEffectiveFirewallRanges(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		allowAll *bool
		ranges   *[]string
		want     []string
	}{
		"allow all traffic": {allowAll: util.Ptr(true), want: []string{util.UnrestrictedCIDR}},
		"explicit ranges":   {allowAll: util.Ptr(false), ranges: util.Ptr([]string{"10.0.0.0/8", "192.168.1.1/32"}), want: []string{"10.0.0.0/8", "192.168.1.1/32"}},
		"no inbound":        {allowAll: util.Ptr(false), ranges: util.Ptr([]string{}), want: []string{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, util.EffectiveFirewallRanges(tc.allowAll, tc.ranges))
		})
	}
}

func TestFirewallRangesConverged(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		configured []types.String
		allowAll   *bool
		ranges     *[]string
		want       bool
	}{
		"same set different order":           {configured: ranges("192.168.1.1/32", "10.0.0.0/8"), ranges: util.Ptr([]string{"10.0.0.0/8", "192.168.1.1/32"}), want: true},
		"unrestricted via allowAllTraffic":   {configured: ranges(util.UnrestrictedCIDR), allowAll: util.Ptr(true), want: true},
		"mixed list with unrestricted":       {configured: ranges("10.0.0.0/8", util.UnrestrictedCIDR), allowAll: util.Ptr(true), want: true},
		"no inbound":                         {allowAll: util.Ptr(false), ranges: util.Ptr([]string{}), want: true},
		"missing range":                      {configured: ranges("192.168.1.1/32", "10.0.0.0/8"), ranges: util.Ptr([]string{"10.0.0.0/8"}), want: false},
		"different range of the same length": {configured: ranges("10.0.0.0/8"), ranges: util.Ptr([]string{"10.1.0.0/8"}), want: false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, util.FirewallRangesConverged(tc.configured, tc.allowAll, tc.ranges))
		})
	}
}

func TestFirewallRangesForState(t *testing.T) {
	t.Parallel()

	configured := ranges("192.168.1.1/32", "10.0.0.0/8")

	for name, tc := range map[string]struct {
		configured []types.String
		allowAll   *bool
		ranges     *[]string
		want       []types.String
	}{
		"preserves configured order when set matches": {configured: configured, ranges: util.Ptr([]string{"10.0.0.0/8", "192.168.1.1/32"}), want: configured},
		"reports api ranges on drift":                 {configured: configured, ranges: util.Ptr([]string{"10.0.0.0/8"}), want: ranges("10.0.0.0/8")},
		"preserves unrestricted spelling":             {configured: ranges(util.UnrestrictedCIDR), allowAll: util.Ptr(true), want: ranges(util.UnrestrictedCIDR)},
		"imported unrestricted allowlist":             {allowAll: util.Ptr(true), want: ranges(util.UnrestrictedCIDR)},
		"imported empty allowlist is non-nil":         {allowAll: util.Ptr(false), ranges: util.Ptr([]string{}), want: []types.String{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := util.FirewallRangesForState(tc.configured, tc.allowAll, tc.ranges)
			require.NotNil(t, got)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSameFirewallRanges(t *testing.T) {
	t.Parallel()

	ab := ranges("10.0.0.0/8", "192.168.0.0/16")

	require.True(t, util.SameFirewallRanges(nil, nil))
	require.True(t, util.SameFirewallRanges(ab, ranges("192.168.0.0/16", "10.0.0.0/8")))
	require.False(t, util.SameFirewallRanges(ab, ab[:1]))
	require.False(t, util.SameFirewallRanges(ab, []types.String{types.StringValue("10.0.0.0/8"), types.StringUnknown()}))
}
