package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

type mockSQLExecutor struct {
	execStatements []string
	queryStatement string
	queryResults   []map[string]string
	execErr        error
	queryErr       error
}

func (m *mockSQLExecutor) Exec(_ context.Context, query string) error {
	m.execStatements = append(m.execStatements, query)

	return m.execErr
}

func (m *mockSQLExecutor) Query(_ context.Context, query string) ([]map[string]string, error) {
	m.queryStatement = query
	if m.queryErr != nil {

		return nil, m.queryErr
	}

	return m.queryResults, nil
}

func (m *mockSQLExecutor) Close() error {

	return nil
}

func TestRowsToList(t *testing.T) {
	t.Parallel()

	list, diags := rowsToList([]map[string]string{
		{"database_name": "my_app_db"},
	})
	require.False(t, diags.HasError())
	require.Equal(t, 1, len(list.Elements()))
}

func TestReadQueryResultsSuccess(t *testing.T) {
	t.Parallel()

	r := &sqlResource{}
	mock := &mockSQLExecutor{
		queryResults: []map[string]string{{"database_name": "my_app_db"}},
	}

	results, diags := r.readQueryResults(t.Context(), mock, types.StringValue("SHOW DATABASES LIKE 'my_app_db'"))
	require.False(t, diags.HasError())
	require.Equal(t, "SHOW DATABASES LIKE 'my_app_db'", mock.queryStatement)
	require.Equal(t, 1, len(results.Elements()))
}

func TestReadQueryResultsWarningOnFailure(t *testing.T) {
	t.Parallel()

	r := &sqlResource{}
	mock := &mockSQLExecutor{queryErr: errors.New("syntax error")}

	_, diags := r.readQueryResults(t.Context(), mock, types.StringValue("INVALID"))
	require.Positive(t, diags.WarningsCount())
	require.False(t, diags.HasError())
}

func TestReadQueryResultsEmptyQuery(t *testing.T) {
	t.Parallel()

	r := &sqlResource{}
	mock := &mockSQLExecutor{}

	results, diags := r.readQueryResults(t.Context(), mock, types.StringNull())
	require.False(t, diags.HasError())
	require.True(t, results.IsNull())
	require.Empty(t, mock.queryStatement)
}

func TestConnectionConfigFromModel(t *testing.T) {
	t.Parallel()

	cfg := connectionConfigFromModel(sqlResourceModel{
		Endpoint: types.StringValue("workspace.example.com"),
		Username: types.StringValue("admin"),
		Password: types.StringValue("secret"),
		Database: types.StringValue("my_app_db"),
		Port:     types.Int64Value(3306),
		TLS:      types.StringValue("preferred"),
	})

	require.Equal(t, ConnectionConfig{
		Endpoint: "workspace.example.com",
		Username: "admin",
		Password: "secret",
		Database: "my_app_db",
		Port:     3306,
		TLS:      "preferred",
	}, cfg)
}
