package sql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildDSN(t *testing.T) {
	t.Parallel()

	dsn := buildDSN(ConnectionConfig{
		Endpoint: "workspace.example.com",
		Username: "admin",
		Password: "secret",
		Database: "my_app_db",
		Port:     3306,
		TLS:      "preferred",
	})

	require.Contains(t, dsn, "admin:secret@tcp(workspace.example.com:3306)/my_app_db?")
	require.Contains(t, dsn, "multiStatements=true")
	require.Contains(t, dsn, "tls=preferred")
}

func TestValueToString(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", valueToString(nil))
	require.Equal(t, "hello", valueToString("hello"))
	require.Equal(t, "world", valueToString([]byte("world")))
	require.Equal(t, "42", valueToString(42))
}
