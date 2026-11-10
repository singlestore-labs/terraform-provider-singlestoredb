package util

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// UnrestrictedCIDR is how a configuration spells "allow traffic from anywhere".
// The Management API reports that state back as the allowAllTraffic flag with an
// omitted firewallRanges list rather than echoing the range.
const UnrestrictedCIDR = "0.0.0.0/0"

// EffectiveFirewallRanges returns the allowlist the Management API reports through
// allowAllTraffic and firewallRanges, spelled the way a configuration spells it.
func EffectiveFirewallRanges(allowAllTraffic *bool, firewallRanges *[]string) []string {
	if Deref(allowAllTraffic) {
		return []string{UnrestrictedCIDR}
	}

	return Deref(firewallRanges)
}

// FirewallRangesConverged reports whether the Management API already reflects the
// configured allowlist.
//
// Ranges are compared as a set: they are applied to a firewall that has no notion
// of order or duplicates, and the API reads them back sorted.
func FirewallRangesConverged(configured []types.String, allowAllTraffic *bool, firewallRanges *[]string) bool {
	want := stringSet(StringFirewallRanges(configured))

	// an allowlist containing the unrestricted range is applied as plain
	// unrestricted access, so the API never reports the other ranges back
	if _, ok := want[UnrestrictedCIDR]; ok && Deref(allowAllTraffic) {
		return true
	}

	return sameSet(want, stringSet(EffectiveFirewallRanges(allowAllTraffic, firewallRanges)))
}

// FirewallRangesForState keeps the configured ordering and spelling whenever the
// Management API reports an equivalent allowlist, so that the order it returns
// does not surface as a diff. An allowlist that genuinely differs is reported as
// the API gives it, so that drift is still detected.
func FirewallRangesForState(configured []types.String, allowAllTraffic *bool, firewallRanges *[]string) []types.String {
	if configured != nil && FirewallRangesConverged(configured, allowAllTraffic, firewallRanges) {
		return configured
	}

	return FirewallRanges(Ptr(EffectiveFirewallRanges(allowAllTraffic, firewallRanges)))
}

// SameFirewallRanges reports whether two allowlists hold the same ranges in any
// order. The Management API does not keep the order, so a reordered list is not a
// change to send. A list with an unknown range is treated as changed.
func SameFirewallRanges(a, b []types.String) bool {
	for _, r := range append(append([]types.String(nil), a...), b...) {
		if r.IsUnknown() {
			return false
		}
	}

	return sameSet(stringSet(StringFirewallRanges(a)), stringSet(StringFirewallRanges(b)))
}

// stringSet collects firewall ranges so allowlists can be compared without regard to order.
func stringSet(ss []string) map[string]struct{} {
	result := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		result[s] = struct{}{}
	}

	return result
}

func sameSet(a, b map[string]struct{}) bool {
	if len(a) != len(b) {
		return false
	}

	for s := range a {
		if _, ok := b[s]; !ok {
			return false
		}
	}

	return true
}
