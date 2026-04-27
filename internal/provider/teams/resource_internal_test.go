package teams

import (
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/stretchr/testify/require"
)

func TestToUsersEmailSet_Deduplicates(t *testing.T) {
	users := []management.UserInfo{
		{Email: "alice@example.com"},
		{Email: "alice@example.com"},
		{Email: "bob@example.com"},
	}

	set := toUsersEmailSet(&users)
	require.False(t, set.IsNull())
	require.False(t, set.IsUnknown())
	require.Equal(t, 2, len(set.Elements()))
	require.Contains(t, set.Elements(), types.StringValue("alice@example.com"))
	require.Contains(t, set.Elements(), types.StringValue("bob@example.com"))
}

func TestToTeamsUUIDSet_Deduplicates(t *testing.T) {
	id1 := uuid.MustParse("9966fccf-5116-437e-a34f-008ee32e8d94")
	id2 := uuid.MustParse("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f")
	teams := []management.TeamInfo{
		{TeamID: id1},
		{TeamID: id1},
		{TeamID: id2},
	}

	set := toTeamsUUIDSet(&teams)
	require.False(t, set.IsNull())
	require.False(t, set.IsUnknown())
	require.Equal(t, 2, len(set.Elements()))
	require.Contains(t, set.Elements(), types.StringValue(id1.String()))
	require.Contains(t, set.Elements(), types.StringValue(id2.String()))
}
