package sql_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/sql"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

const (
	testWorkspaceEndpoint = "workspace.example.com"
	testAdminPassword     = "sfkjDIJ423d44w1sfooBar1$" //nolint:gosec

	dataAPIExecPath  = "/api/v2/exec"
	dataAPIQueryPath = "/api/v2/query/rows"
)

func withMockDataAPIServer(t *testing.T, handler http.Handler) {
	t.Helper()

	server := httptest.NewServer(handler)
	target, err := url.Parse(server.URL)
	require.NoError(t, err)

	restore := sql.SetHTTPClientFactoryForTest(func() *http.Client {
		return &http.Client{
			Transport: redirectTransport{target: target},
		}
	})

	t.Cleanup(func() {
		restore()
		server.Close()
	})
}

type redirectTransport struct {
	target *url.URL
}

func (rt redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = rt.target.Scheme
	cloned.URL.Host = rt.target.Host

	return http.DefaultTransport.RoundTrip(cloned)
}

func sqlExecuteConfig(endpoint string) string {
	return fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_sql_execute" "this" {
  endpoint     = %q
  username     = "admin"
  password     = "secret"
  execute      = "CREATE DATABASE IF NOT EXISTS ?"
  execute_args = ["my_app_db"]
  revert       = "DROP DATABASE IF EXISTS my_app_db"
  query        = "SHOW DATABASES LIKE ?"
  query_args   = ["my_app_db"]
}
`, endpoint)
}

func minimalSQLExecuteConfig(endpoint string) string {
	return fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_sql_execute" "this" {
  endpoint = %q
  username = "admin"
  password = "secret"
  execute  = "SELECT 1"
  revert   = "SELECT 1"
}
`, endpoint)
}

func TestSQLExecuteCreateReadDestroy(t *testing.T) {
	var execCalls atomic.Int32
	var queryCalls atomic.Int32

	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		switch r.URL.Path {
		case dataAPIExecPath:
			execCalls.Add(1)
			require.Equal(t, http.MethodPost, r.Method)

			user, pass, ok := r.BasicAuth()
			require.True(t, ok)
			require.Equal(t, "admin", user)
			require.Equal(t, "secret", pass)

			if execCalls.Load() == 1 {
				require.JSONEq(t, `{"sql":"CREATE DATABASE IF NOT EXISTS ?","args":["my_app_db"]}`, string(body))
			} else {
				require.JSONEq(t, `{"sql":"DROP DATABASE IF EXISTS my_app_db"}`, string(body))
			}

			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{"lastInsertId":7,"rowsAffected":1}`))
			require.NoError(t, err)
		case dataAPIQueryPath:
			queryCalls.Add(1)
			require.JSONEq(t, `{"sql":"SHOW DATABASES LIKE ?","args":["my_app_db"]}`, string(body))

			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write([]byte(`{"results":[{"rows":[{"Database":"my_app_db"}]}]}`))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: sqlExecuteConfig(testWorkspaceEndpoint),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("singlestoredb_sql_execute.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "last_insert_id", "7"),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "rows_affected", "1"),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.0.Database", "my_app_db"),
				),
			},
			{
				ResourceName: "singlestoredb_sql_execute.this",
				ImportState:  true,
				ExpectError:  regexp.MustCompile("Import not supported"),
			},
		},
	})

	require.Equal(t, int32(2), execCalls.Load(), "create and destroy should call /exec")
	require.GreaterOrEqual(t, queryCalls.Load(), int32(1), "create/read should call /query/rows")
}

func TestSQLExecutePasswordFromEnvNotInState(t *testing.T) {
	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "admin", user)
		require.Equal(t, "env-secret", pass)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case dataAPIExecPath:
			_, err := w.Write([]byte(`{"lastInsertId":0,"rowsAffected":0}`))
			require.NoError(t, err)
		case dataAPIQueryPath:
			_, err := w.Write([]byte(`{"results":[{"rows":[]}]}`))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	t.Setenv(config.EnvSQLUserPassword, "env-secret")

	configNoPassword := fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_sql_execute" "this" {
  endpoint = %q
  username = "admin"
  execute  = "SELECT 1"
  revert   = "SELECT 1"
  query    = "SELECT 1"
}
`, testWorkspaceEndpoint)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: configNoPassword,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("singlestoredb_sql_execute.this", "password"),
				),
			},
		},
	})
}

func TestSQLExecutePlanReplacementOnExecuteChange(t *testing.T) {
	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"lastInsertId":0,"rowsAffected":0}`))
		require.NoError(t, err)
	}))

	baseConfig := minimalSQLExecuteConfig(testWorkspaceEndpoint)
	updatedConfig := fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_sql_execute" "this" {
  endpoint = %q
  username = "admin"
  password = "secret"
  execute  = "SELECT 2"
  revert   = "SELECT 1"
}
`, testWorkspaceEndpoint)

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{Config: baseConfig},
			{
				Config:      updatedConfig,
				ExpectError: regexp.MustCompile("Execute statement change requires replacement"),
			},
		},
	})
}

func TestSQLExecuteUpdateInPlace(t *testing.T) {
	var execCalls atomic.Int32

	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case dataAPIExecPath:
			execCalls.Add(1)
			_, err := w.Write([]byte(`{"lastInsertId":0,"rowsAffected":1}`))
			require.NoError(t, err)
		case dataAPIQueryPath:
			_, err := w.Write([]byte(`{"results":[{"rows":[{"Database":"my_app_db"}]}]}`))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	configWithRevert := func(revert string) string {
		return fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_sql_execute" "this" {
  endpoint   = %q
  username   = "admin"
  password   = "secret"
  execute    = "SELECT 1"
  revert     = %q
  query      = "SHOW DATABASES LIKE ?"
  query_args = ["my_app_db"]
}
`, testWorkspaceEndpoint, revert)
	}

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: configWithRevert("SELECT 1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "revert", "SELECT 1"),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.#", "1"),
				),
			},
			{
				Config: configWithRevert("SELECT 2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "revert", "SELECT 2"),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.#", "1"),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.0.Database", "my_app_db"),
				),
			},
		},
	})

	require.Equal(t, int32(2), execCalls.Load(), "only create and destroy call /exec; in-place update must not run the execute statement")
}

func TestSQLExecuteDestroySucceedsWhenWorkspaceUnreachable(t *testing.T) {
	var revertCalls atomic.Int32

	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		// The revert statement (run on destroy) gets a 503, simulating a
		// suspended/deleted workspace. Destroy must still succeed.
		if r.URL.Path == dataAPIExecPath && strings.Contains(string(body), "DROP DATABASE") {
			revertCalls.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case dataAPIExecPath:
			_, err = w.Write([]byte(`{"lastInsertId":0,"rowsAffected":0}`))
		default:
			_, err = w.Write([]byte(`{"results":[{"rows":[]}]}`))
		}
		require.NoError(t, err)
	}))

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{Config: sqlExecuteConfig(testWorkspaceEndpoint)},
		},
	})

	require.GreaterOrEqual(t, revertCalls.Load(), int32(1), "destroy should attempt the revert statement")
}

func TestSQLExecuteCreateFailsWhenQueryFails(t *testing.T) {
	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case dataAPIExecPath:
			_, err := w.Write([]byte(`{"lastInsertId":0,"rowsAffected":0}`))
			require.NoError(t, err)
		case dataAPIQueryPath:
			// Read-back query fails with an in-body error. On create this is a
			// configuration error and must fail the apply (not just warn).
			_, err := w.Write([]byte(`{"error":{"code":1146,"message":"Table 'missing' doesn't exist"}}`))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      sqlExecuteConfig(testWorkspaceEndpoint),
				ExpectError: regexp.MustCompile("SQL query failed"),
			},
		},
	})
}

func TestSQLExecuteUpdatePasswordChange(t *testing.T) {
	var lastPassword atomic.Value
	lastPassword.Store("")

	withMockDataAPIServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, pass, ok := r.BasicAuth()
		require.True(t, ok)
		lastPassword.Store(pass)

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case dataAPIExecPath:
			_, err := w.Write([]byte(`{"lastInsertId":0,"rowsAffected":0}`))
			require.NoError(t, err)
		case dataAPIQueryPath:
			_, err := w.Write([]byte(`{"results":[{"rows":[{"Database":"my_app_db"}]}]}`))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	configWithPassword := func(password string) string {
		return fmt.Sprintf(`
provider "singlestoredb" {
}

resource "singlestoredb_sql_execute" "this" {
  endpoint   = %q
  username   = "admin"
  password   = %q
  execute    = "SELECT 1"
  revert     = "SELECT 1"
  query      = "SHOW DATABASES LIKE ?"
  query_args = ["my_app_db"]
}
`, testWorkspaceEndpoint, password)
	}

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIKey: testutil.UnusedAPIKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: configWithPassword("secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "password", "secret"),
				),
			},
			{
				Config: configWithPassword("rotated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "password", "rotated"),
					resource.TestCheckResourceAttrWith("singlestoredb_sql_execute.this", "password", func(string) error {
						if got := lastPassword.Load().(string); got != "rotated" {
							return fmt.Errorf("expected rotated password sent to Data API on update, got %q", got)
						}

						return nil
					}),
				),
			},
		},
	})
}

func TestDataAPIRequestBodyShape(t *testing.T) {
	t.Parallel()

	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&captured))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"lastInsertId":0,"rowsAffected":0}`))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	client := sql.NewClient(server.URL, "admin", "secret")
	_, err := client.Exec(t.Context(), sql.ExecRequest{
		SQL:      "CREATE USER ?",
		Args:     []any{"x"},
		Database: "db",
	})
	require.NoError(t, err)
	require.Equal(t, "CREATE USER ?", captured["sql"])
	require.Equal(t, "db", captured["database"])
}

func TestSQLExecuteResourceIntegration(t *testing.T) {
	adminPassword := testAdminPassword
	isDataAPIReady := testutil.IsDataAPIReady(adminPassword)

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "example",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.SQLExecuteResource).
					WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
					String(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_workspace.this", "name", config.TestWorkspaceName),
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", isDataAPIReady),
					resource.TestCheckResourceAttrSet("singlestoredb_sql_execute.this", config.IDAttribute),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.#", "1"),
				),
			},
		},
	})
}

func TestSQLExecuteDriftIntegration(t *testing.T) {
	adminPassword := testAdminPassword
	isDataAPIReady := testutil.IsDataAPIReady(adminPassword)

	var workspaceEndpoint string

	integrationConfig := testutil.UpdatableConfig(examples.SQLExecuteResource).
		WithWorkspaceGroupResource("example")("admin_password", cty.StringVal(adminPassword)).
		String()

	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey:             os.Getenv(config.EnvTestAPIKey),
		WorkspaceGroupName: "example",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: integrationConfig,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", isDataAPIReady),
					resource.TestCheckResourceAttrWith("singlestoredb_workspace.this", "endpoint", func(endpoint string) error {
						workspaceEndpoint = endpoint

						return nil
					}),
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.#", "1"),
				),
			},
			{
				PreConfig: func() {
					baseURL, err := sql.DataAPIURL(workspaceEndpoint)
					require.NoError(t, err)

					client := sql.NewClient(baseURL, "admin", adminPassword)
					_, err = client.Exec(t.Context(), sql.ExecRequest{
						SQL: "DROP DATABASE IF EXISTS my_app_db",
					})
					require.NoError(t, err)
				},
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("singlestoredb_sql_execute.this", "query_results.#", "0"),
				),
			},
		},
	})
}
