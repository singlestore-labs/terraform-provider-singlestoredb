package workspaces

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
)

func EffectiveFirewallRangesForTest(c management.Cluster) []string {
	return effectiveFirewallRanges(c)
}

func KaiPatchIfChangedForTest(plan types.Bool, current *bool) *bool {
	return kaiPatchIfChanged(plan, current)
}
