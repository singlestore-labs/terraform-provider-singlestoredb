package sql_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/sql"
	"github.com/stretchr/testify/require"
)

func TestQueryDataSourceID(t *testing.T) {
	t.Parallel()

	id1 := sql.QueryDataSourceIDForTest("host.example.com", "SELECT 1", []string{"a"})
	id2 := sql.QueryDataSourceIDForTest("host.example.com", "SELECT 1", []string{"b"})
	id3 := sql.QueryDataSourceIDForTest("host.example.com", "SELECT 2", []string{"a"})
	id4 := sql.QueryDataSourceIDForTest("other.example.com", "SELECT 1", []string{"a"})

	require.NotEqual(t, id1, id2)
	require.NotEqual(t, id1, id3)
	require.NotEqual(t, id1, id4)
	require.Equal(t, id1, sql.QueryDataSourceIDForTest("host.example.com", "SELECT 1", []string{"a"}))

	// Equivalent endpoints that normalize to the same Data API URL must produce
	// the same ID so a port suffix does not churn state.
	require.Equal(t, id1, sql.QueryDataSourceIDForTest("host.example.com:3306", "SELECT 1", []string{"a"}))
	require.Equal(t, id1, sql.QueryDataSourceIDForTest("  host.example.com:443  ", "SELECT 1", []string{"a"}))
}
