package workspacegroups

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceGroupPatchAdminPassword(t *testing.T) {
	t.Parallel()

	pw := types.StringValue("NewValidPassword193!")
	empty := types.StringValue("")
	same := types.StringValue("same-password-here!!")

	t.Run("unchanged omits", func(t *testing.T) {
		t.Parallel()
		plan := workspaceGroupResourceModel{AdminPassword: same}
		state := workspaceGroupResourceModel{AdminPassword: same}
		require.Nil(t, workspaceGroupPatchAdminPassword(plan, state))
	})

	t.Run("changed to non-empty sends", func(t *testing.T) {
		t.Parallel()
		plan := workspaceGroupResourceModel{AdminPassword: pw}
		state := workspaceGroupResourceModel{AdminPassword: empty}
		got := workspaceGroupPatchAdminPassword(plan, state)
		require.NotNil(t, got)
		require.Equal(t, pw.ValueString(), *got)
	})

	t.Run("changed to empty omits", func(t *testing.T) {
		t.Parallel()
		plan := workspaceGroupResourceModel{AdminPassword: empty}
		state := workspaceGroupResourceModel{AdminPassword: pw}
		require.Nil(t, workspaceGroupPatchAdminPassword(plan, state))
	})

	t.Run("both empty omits", func(t *testing.T) {
		t.Parallel()
		plan := workspaceGroupResourceModel{AdminPassword: empty}
		state := workspaceGroupResourceModel{AdminPassword: empty}
		require.Nil(t, workspaceGroupPatchAdminPassword(plan, state))
	})
}
