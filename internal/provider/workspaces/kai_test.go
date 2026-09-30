package workspaces

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestKaiPatchIfChanged(t *testing.T) {
	require.Nil(t, kaiPatchIfChanged(types.BoolNull(), nil))
	require.Nil(t, kaiPatchIfChanged(types.BoolUnknown(), nil))
	require.Nil(t, kaiPatchIfChanged(types.BoolValue(false), nil))
	require.Nil(t, kaiPatchIfChanged(types.BoolValue(false), util.Ptr(false)))
	require.Nil(t, kaiPatchIfChanged(types.BoolValue(true), util.Ptr(true)))

	require.Equal(t, util.Ptr(true), kaiPatchIfChanged(types.BoolValue(true), nil))
	require.Equal(t, util.Ptr(true), kaiPatchIfChanged(types.BoolValue(true), util.Ptr(false)))
	require.Equal(t, util.Ptr(false), kaiPatchIfChanged(types.BoolValue(false), util.Ptr(true)))
}
