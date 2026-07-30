package workspacegroups

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestEffectiveFirewallRanges(t *testing.T) {
	t.Parallel()

	t.Run("allow all traffic", func(t *testing.T) {
		t.Parallel()
		got := effectiveFirewallRanges(management.WorkspaceGroup{
			AllowAllTraffic: util.Ptr(true),
			FirewallRanges:  nil,
		})
		require.Equal(t, []string{unrestrictedCIDR}, got)
	})

	t.Run("explicit ranges", func(t *testing.T) {
		t.Parallel()
		got := effectiveFirewallRanges(management.WorkspaceGroup{
			AllowAllTraffic: util.Ptr(false),
			FirewallRanges:  util.Ptr([]string{"10.0.0.0/8", "192.168.1.1/32"}),
		})
		require.Equal(t, []string{"10.0.0.0/8", "192.168.1.1/32"}, got)
	})

	t.Run("no inbound", func(t *testing.T) {
		t.Parallel()
		got := effectiveFirewallRanges(management.WorkspaceGroup{
			AllowAllTraffic: util.Ptr(false),
			FirewallRanges:  util.Ptr([]string{}),
		})
		require.Empty(t, got)
	})
}

func TestFirewallRangesConverged(t *testing.T) {
	t.Parallel()

	configured := []types.String{
		types.StringValue("192.168.1.1/32"),
		types.StringValue("10.0.0.0/8"),
	}

	t.Run("same set different order", func(t *testing.T) {
		t.Parallel()
		require.True(t, firewallRangesConverged(configured, management.WorkspaceGroup{
			FirewallRanges: util.Ptr([]string{"10.0.0.0/8", "192.168.1.1/32"}),
		}))
	})

	t.Run("unrestricted via allowAllTraffic", func(t *testing.T) {
		t.Parallel()
		require.True(t, firewallRangesConverged(
			[]types.String{types.StringValue(unrestrictedCIDR)},
			management.WorkspaceGroup{AllowAllTraffic: util.Ptr(true)},
		))
	})

	t.Run("mixed list with unrestricted collapses", func(t *testing.T) {
		t.Parallel()
		require.True(t, firewallRangesConverged(
			[]types.String{
				types.StringValue("10.0.0.0/8"),
				types.StringValue(unrestrictedCIDR),
			},
			management.WorkspaceGroup{AllowAllTraffic: util.Ptr(true)},
		))
	})

	t.Run("no inbound", func(t *testing.T) {
		t.Parallel()
		require.True(t, firewallRangesConverged(nil, management.WorkspaceGroup{
			AllowAllTraffic: util.Ptr(false),
			FirewallRanges:  util.Ptr([]string{}),
		}))
	})

	t.Run("genuine drift", func(t *testing.T) {
		t.Parallel()
		require.False(t, firewallRangesConverged(configured, management.WorkspaceGroup{
			FirewallRanges: util.Ptr([]string{"10.0.0.0/8"}),
		}))
	})
}

func TestFirewallRangesForState(t *testing.T) {
	t.Parallel()

	configured := []types.String{
		types.StringValue("192.168.1.1/32"),
		types.StringValue("10.0.0.0/8"),
	}

	t.Run("preserves configured order when set matches", func(t *testing.T) {
		t.Parallel()
		got := firewallRangesForState(configured, management.WorkspaceGroup{
			FirewallRanges: util.Ptr([]string{"10.0.0.0/8", "192.168.1.1/32"}),
		})
		require.Equal(t, configured, got)
	})

	t.Run("reports api ranges on drift", func(t *testing.T) {
		t.Parallel()
		got := firewallRangesForState(configured, management.WorkspaceGroup{
			FirewallRanges: util.Ptr([]string{"10.0.0.0/8"}),
		})
		require.Equal(t, []types.String{types.StringValue("10.0.0.0/8")}, got)
	})

	t.Run("preserves unrestricted spelling", func(t *testing.T) {
		t.Parallel()
		configuredUnrestricted := []types.String{types.StringValue(unrestrictedCIDR)}
		got := firewallRangesForState(configuredUnrestricted, management.WorkspaceGroup{
			AllowAllTraffic: util.Ptr(true),
		})
		require.Equal(t, configuredUnrestricted, got)
	})
}
