package workspaces

import (
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// unrestrictedCIDR is how a configuration spells "allow traffic from anywhere".
// The Management API reports that state back as allowAllTraffic=true with an
// empty/omitted firewallRanges list rather than echoing the range.
const unrestrictedCIDR = "0.0.0.0/0"

// effectiveFirewallRanges returns the allowlist to send on create, spelled the
// way POST /v2/clusters expects it. Copying raw FirewallRanges from a sibling
// that has AllowAllTraffic=true yields an empty list, which the API treats as
// deny-all.
func effectiveFirewallRanges(cluster management.Cluster) []string {
	if util.Deref(cluster.AllowAllTraffic) {
		return []string{unrestrictedCIDR}
	}

	return util.Deref(cluster.FirewallRanges)
}
