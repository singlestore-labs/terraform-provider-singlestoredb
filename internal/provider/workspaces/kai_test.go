package workspaces_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspaces"
	"github.com/stretchr/testify/require"
)

func TestKaiPatchIfChanged(t *testing.T) {
	require.Nil(t, workspaces.KaiPatchIfChangedForTest(types.BoolNull(), nil))
	require.Nil(t, workspaces.KaiPatchIfChangedForTest(types.BoolUnknown(), nil))
	require.Nil(t, workspaces.KaiPatchIfChangedForTest(types.BoolValue(false), nil))
	require.Nil(t, workspaces.KaiPatchIfChangedForTest(types.BoolValue(false), util.Ptr(false)))
	require.Nil(t, workspaces.KaiPatchIfChangedForTest(types.BoolValue(true), util.Ptr(true)))

	require.Equal(t, util.Ptr(true), workspaces.KaiPatchIfChangedForTest(types.BoolValue(true), nil))
	require.Equal(t, util.Ptr(true), workspaces.KaiPatchIfChangedForTest(types.BoolValue(true), util.Ptr(false)))
	require.Equal(t, util.Ptr(false), workspaces.KaiPatchIfChangedForTest(types.BoolValue(false), util.Ptr(true)))
}
