package workspaces_test

import (
	"testing"

	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspaces"
	"github.com/stretchr/testify/require"
)

func TestEffectiveFirewallRanges(t *testing.T) {
	require.Equal(t, []string{"0.0.0.0/0"}, workspaces.EffectiveFirewallRangesForTest(management.Cluster{
		AllowAllTraffic: util.Ptr(true),
		FirewallRanges:  util.Ptr([]string{}),
	}))
	require.Equal(t, []string{"10.0.0.0/8"}, workspaces.EffectiveFirewallRangesForTest(management.Cluster{
		AllowAllTraffic: util.Ptr(false),
		FirewallRanges:  util.Ptr([]string{"10.0.0.0/8"}),
	}))
}
