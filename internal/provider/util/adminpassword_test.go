package util_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestAdminPasswordForState(t *testing.T) {
	require.Equal(t, "generated", util.AdminPasswordForState("configured", "generated"))
	require.Equal(t, "configured", util.AdminPasswordForState("configured", ""))
	require.Equal(t, "generated", util.AdminPasswordForState("", "generated"))
}
