package workspacegroups

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// unrestrictedCIDR is how a configuration spells "allow traffic from anywhere".
// The Management API reports that state back as the allowAllTraffic flag with an
// omitted firewallRanges list rather than echoing the range.
const unrestrictedCIDR = "0.0.0.0/0"

// effectiveFirewallRanges returns the allowlist the Management API reports,
// spelled the way a configuration spells it.
func effectiveFirewallRanges(workspaceGroup management.WorkspaceGroup) []string {
	if util.Deref(workspaceGroup.AllowAllTraffic) {
		return []string{unrestrictedCIDR}
	}

	return util.Deref(workspaceGroup.FirewallRanges)
}

// firewallRangesConverged reports whether the Management API already reflects the
// configured allowlist.
//
// Ranges are compared as a set: they are applied to a firewall that has no notion
// of order or duplicates, and the API reads them back sorted.
func firewallRangesConverged(configured []types.String, workspaceGroup management.WorkspaceGroup) bool {
	want := stringSet(util.StringFirewallRanges(configured))

	// an allowlist containing the unrestricted range is applied as plain
	// unrestricted access, so the API never reports the other ranges back
	if _, ok := want[unrestrictedCIDR]; ok && util.Deref(workspaceGroup.AllowAllTraffic) {
		return true
	}

	got := stringSet(effectiveFirewallRanges(workspaceGroup))
	if len(want) != len(got) {
		return false
	}

	for firewallRange := range want {
		if _, ok := got[firewallRange]; !ok {
			return false
		}
	}

	return true
}

// firewallRangesForState keeps the configured ordering and spelling whenever the
// Management API reports an equivalent allowlist, so that the order it returns
// does not surface as a diff. An allowlist that genuinely differs is reported as
// the API gives it, so that drift is still detected.
func firewallRangesForState(configured []types.String, workspaceGroup management.WorkspaceGroup) []types.String {
	if firewallRangesConverged(configured, workspaceGroup) {
		return configured
	}

	return util.FirewallRanges(util.Ptr(effectiveFirewallRanges(workspaceGroup)))
}

func stringSet(ss []string) map[string]struct{} {
	result := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		result[s] = struct{}{}
	}

	return result
}
