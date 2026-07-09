package sql_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/sql"
	"github.com/stretchr/testify/require"
)

func TestResolvePassword_ExplicitWins(t *testing.T) {
	t.Setenv(config.EnvSQLUserPassword, "from-env")

	got, err := sql.ResolvePasswordForTest(types.StringValue("explicit"))
	require.Nil(t, err)
	require.Equal(t, "explicit", got)
}

func TestResolvePassword_FromEnv(t *testing.T) {
	t.Setenv(config.EnvSQLUserPassword, "from-env")

	got, err := sql.ResolvePasswordForTest(types.StringNull())
	require.Nil(t, err)
	require.Equal(t, "from-env", got)
}

func TestResolvePassword_Missing(t *testing.T) {
	require.NoError(t, os.Unsetenv(config.EnvSQLUserPassword))

	got, err := sql.ResolvePasswordForTest(types.StringNull())
	require.Empty(t, got)
	require.NotNil(t, err)
	require.Contains(t, err.Summary, "Missing SQL credentials")
}

func TestPasswordForState(t *testing.T) {
	t.Parallel()

	require.True(t, sql.PasswordForStateForTest(types.StringValue("secret")).Equal(types.StringValue("secret")))
	require.True(t, sql.PasswordForStateForTest(types.StringNull()).IsNull())
}
