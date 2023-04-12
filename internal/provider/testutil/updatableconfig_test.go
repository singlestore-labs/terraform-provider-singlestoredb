package testutil_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestUpdatableConfigWithWorkspace(t *testing.T) {
	uc := testutil.UpdatableConfig(`resource "singlestoredb_workspace" "example" {
	}`)
	require.NotContains(t, uc, "suspended")
	uc = uc.WithWorkspace("example")("suspended", cty.BoolVal(true))
	require.Contains(t, uc, "suspended")
	require.Contains(t, uc, "true")
	uc = uc.WithWorkspace("example")("suspended", cty.BoolVal(false))
	require.NotContains(t, uc, "true")
	require.Contains(t, uc, "false")
}
