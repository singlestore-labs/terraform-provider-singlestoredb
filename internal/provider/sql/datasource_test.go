package sql_test

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func sqlQueryConfig(args string) string {
	return fmt.Sprintf(`
provider "singlestoredb" {
}

data "singlestoredb_sql_query" "this" {
  endpoint = %q
  username = "admin"
  password = "secret"
  database = "my_app_db"
  query    = "SELECT id, email FROM users WHERE created_at > ?"
  args     = %s
}
`, testWorkspaceEndpoint, args)
}

func TestSQLQueryReadReturnsRows(t *testing.T) {
	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v2/query/rows", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "admin", user)
		require.Equal(t, "secret", pass)

		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.JSONEq(t, `{"sql":"SELECT id, email FROM users WHERE created_at > ?","args":["2025-01-01"],"database":"my_app_db"}`, string(body))

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"results":[{"rows":[{"id":1,"email":"alice@example.com"},{"id":2,"email":"bob@example.com"}]}]}`))
		require.NoError(t, err)
	}))

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: sqlQueryConfig(`["2025-01-01"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.singlestoredb_sql_query.this", config.IDAttribute),
					resource.TestCheckResourceAttr("data.singlestoredb_sql_query.this", "rows.#", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_sql_query.this", "rows.0.id", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_sql_query.this", "rows.0.email", "alice@example.com"),
					resource.TestCheckResourceAttr("data.singlestoredb_sql_query.this", "rows.1.id", "2"),
					resource.TestCheckResourceAttr("data.singlestoredb_sql_query.this", "rows.1.email", "bob@example.com"),
				),
			},
		},
	})
}

func TestSQLQueryHardErrorOnQueryFailure(t *testing.T) {
	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"error":{"code":1064,"message":"You have an error in your SQL syntax"}}`))
		require.NoError(t, err)
	}))

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      sqlQueryConfig(`["2025-01-01"]`),
				ExpectError: regexp.MustCompile("SQL query failed"),
			},
		},
	})
}

func TestSQLQueryIDChangesWhenArgsChange(t *testing.T) {
	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"results":[{"rows":[{"id":1,"email":"alice@example.com"}]}]}`))
		require.NoError(t, err)
	}))

	var firstID string

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: sqlQueryConfig(`["2025-01-01"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("data.singlestoredb_sql_query.this", config.IDAttribute, func(value string) error {
						firstID = value

						return nil
					}),
				),
			},
			{
				Config: sqlQueryConfig(`["2025-02-01"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("data.singlestoredb_sql_query.this", config.IDAttribute, func(value string) error {
						if value == firstID {
							return fmt.Errorf("expected id to change when args change, still %q", value)
						}

						return nil
					}),
				),
			},
		},
	})
}

func TestSQLQueryDataSourceIntegration(t *testing.T) {
	adminPassword := testAdminPassword
	isDataAPIReady := testutil.IsDataAPIReady(adminPassword)

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "example",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.SQLQueryDataSource).
					WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", isDataAPIReady),
					resource.TestCheckResourceAttrSet("data.singlestoredb_sql_query.this", config.IDAttribute),
					resource.TestCheckResourceAttr("data.singlestoredb_sql_query.this", "rows.#", "1"),
					resource.TestCheckResourceAttr("data.singlestoredb_sql_query.this", "rows.0.value", "1"),
				),
			},
		},
	})
}
