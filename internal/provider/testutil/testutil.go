package testutil

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"

	// Loading the mysql driver to test connecting to SingleStore DB.
	_ "github.com/go-sql-driver/mysql"
)

const (
	UnusedAPIKey = "foo"
	devVersion   = "dev"
)

type UnitTestConfig struct {
	APIKeyFromEnv string
	APIKey        string
	APIServiceURL string
}

type IntegrationTestConfig struct {
	APIKey             string
	WorkspaceGroupName string
}

// UnitTest is a helper around resource.UnitTest with provider factories
// already configured.
func UnitTest(t *testing.T, conf UnitTestConfig, c resource.TestCase) {
	t.Helper()

	if conf.APIKeyFromEnv == "" {
		t.Setenv(config.EnvAPIKey, "") // The default behavior is to ignore the environment.
	} else {
		t.Setenv(config.EnvAPIKey, conf.APIKeyFromEnv)
	}

	for i, s := range c.Steps {
		c.Steps[i].Config = UpdatableConfig(s.Config).
			WithAPIKey(conf.APIKey).
			WithAPIServiceURL(conf.APIServiceURL).
			String()
	}

	f := provider.New(devVersion)
	c.ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		config.ProviderName: providerserver.NewProtocol6WithError(f()),
	}
	resource.UnitTest(t, c)
}

func IntegrationTest(t *testing.T, conf IntegrationTestConfig, c resource.TestCase) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test because go test is run with the flag -short")
	}

	require.NotEmpty(t, conf.APIKey, "envirnomental variable %s should be set for running integration tests", config.EnvTestAPIKey)

	for i, s := range c.Steps {
		if conf.WorkspaceGroupName != "" {
			instantExpiration := time.Now().UTC().Add(config.TestWorkspaceGroupExpiration).Format(time.RFC3339)

			c.Steps[i].Config = UpdatableConfig(s.Config).
				WithWorkspaceGroupResource(conf.WorkspaceGroupName)("expires_at", cty.StringVal(instantExpiration)).
				String() // Ensures garbage collection.
		}
	}

	t.Setenv("TF_ACC", "on") // Enables running the integration test.
	t.Setenv(config.EnvAPIKey, conf.APIKey)

	f := provider.New(devVersion)
	c.ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		config.ProviderName: providerserver.NewProtocol6WithError(f()),
	}
	resource.Test(t, c)
}

func MustJSON(s interface{}) []byte {
	result, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}

	return result
}

// IsConnectableWithAdminPassword attempts to connect to the workspace and execute a sample SQL query.
func IsConnectableWithAdminPassword(adminPassword string) resource.CheckResourceAttrWithFunc {
	return func(endpoint string) error {
		defaultParams := map[string]string{
			"parseTime":         "true",
			"interpolateParams": "true",
			"timeout":           "10s",
			"tls":               "preferred",
		}

		mergedParams := []string{}
		for parameName, paramVal := range defaultParams {
			mergedParams = append(mergedParams, fmt.Sprintf("%s=%s", parameName, paramVal))
		}

		connParams := strings.Join(mergedParams, "&")

		connString := fmt.Sprintf(
			"%s:%s@tcp(%s)/?%s",
			"admin",
			adminPassword,
			endpoint,
			connParams,
		)

		conn, err := sql.Open("mysql", connString)
		if err != nil {
			return err
		}
		defer conn.Close()

		conn.SetConnMaxLifetime(time.Hour)
		conn.SetMaxIdleConns(config.TestMaxIdleConns)
		conn.SetMaxOpenConns(config.TestMaxOpenConns)

		if err := conn.Ping(); err != nil {
			return err
		}

		var one int
		if err := conn.QueryRow("SELECT 1").Scan(&one); err != nil {
			return err
		}

		if one != 1 {
			return fmt.Errorf("executing 'SELECT 1' for the endpoint %s failed because the query returned %d while expecting 1", endpoint, one)
		}

		return nil
	}
}

func resourceTypeName(name string) string {
	return strings.Join([]string{config.ProviderName, name}, "_")
}

func dataSourceTypeName(name string) string {
	return strings.Join([]string{config.ProviderName, name}, "_")
}
