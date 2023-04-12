package workspaces_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspaces"
	"github.com/stretchr/testify/require"
)

func TestParseSize(t *testing.T) {
	_, err := workspaces.ParseSize("whatever")
	require.Error(t, err, "zero size is no longer supported")

	size, err := workspaces.ParseSize("S-00")
	require.Nil(t, err)
	require.Equal(t, "0.25", size.String())

	size, err = workspaces.ParseSize("S-0")
	require.Nil(t, err)
	require.Equal(t, "0.5", size.String())

	size, err = workspaces.ParseSize("S-1")
	require.Nil(t, err)
	require.Equal(t, "1", size.String())

	size, err = workspaces.ParseSize("S-2")
	require.Nil(t, err)
	require.Equal(t, "2", size.String())

	size, err = workspaces.ParseSize("0.25")
	require.Nil(t, err)
	require.Equal(t, "0.25", size.String())

	size, err = workspaces.ParseSize("0.5")
	require.Nil(t, err)
	require.Equal(t, "0.5", size.String())

	size, err = workspaces.ParseSize("1")
	require.Nil(t, err)
	require.Equal(t, "1", size.String())

	size, err = workspaces.ParseSize("2")
	require.Nil(t, err)
	require.Equal(t, "2", size.String())
}

func TestWorkspaceSizeEq(t *testing.T) {
	suspended, err := workspaces.ParseSize("")
	require.Error(t, err)

	s00, err := workspaces.ParseSize("0.25")
	require.Nil(t, err)

	s0, err := workspaces.ParseSize("0.5")
	require.Nil(t, err)

	s1, err := workspaces.ParseSize("1")
	require.Nil(t, err)

	s2, err := workspaces.ParseSize("2")
	require.Nil(t, err)

	require.True(t, suspended.Eq(suspended))
	require.True(t, s00.Eq(s00))
	require.True(t, s0.Eq(s0))
	require.True(t, s1.Eq(s1))
	require.True(t, s2.Eq(s2))
	require.False(t, suspended.Eq(s0))
	require.False(t, s00.Eq(s0))
	require.False(t, s0.Eq(s1))
	require.False(t, s1.Eq(s2))
}
