package testutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cenkalti/backoff/v4"
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
	UnusedAPIKey   = "foo"
	devVersion     = "dev"
	connectRetries = 10
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

// Generate unique workspace group name for this test run to enable parallel execution.
var UniqueGroupName = GenerateUniqueResourceName(config.TestInitialWorkspaceGroupName)

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
		updatedConfig := UpdatableConfig(s.Config).WithUniqueWorkspaceGroupNames(UniqueGroupName)

		if conf.WorkspaceGroupName != "" {
			instantExpiration := time.Now().UTC().Add(config.TestWorkspaceGroupExpiration).Format(time.RFC3339)
			updatedConfig = updatedConfig.WithWorkspaceGroupResource(conf.WorkspaceGroupName)("expires_at", cty.StringVal(instantExpiration))
		}
		c.Steps[i].Config = updatedConfig.String() // Ensures garbage collection and unique naming for parallel test execution.
	}

	t.Setenv("TF_ACC", "on") // Enables running the integration test.
	t.Setenv(config.EnvAPIKey, conf.APIKey)

	f := provider.New(devVersion)
	c.ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		config.ProviderName: providerserver.NewProtocol6WithError(f()),
	}
	resource.Test(t, c)
}

// GenerateUniqueResourceName generates a unique resource name by appending a timestamp and random suffix.
// This enables running multiple test suites in parallel without resource name conflicts.
func GenerateUniqueResourceName(baseName string) string {
	timestamp := time.Now().UTC().Format("20060102-150405")
	byteLen := 4
	randomBytes := make([]byte, byteLen)
	_, _ = rand.Read(randomBytes)
	randomSuffix := hex.EncodeToString(randomBytes)

	return fmt.Sprintf("terraform-%s-%s-%s", baseName, timestamp, randomSuffix)
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
		b := backoff.WithMaxRetries(backoff.NewExponentialBackOff(), connectRetries)

		return backoff.Retry(func() error {
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
		}, b)
	}
}

// CreateTemp creates a temporary file with body and returns the path and the cleanup callback.
func CreateTemp(body string) (string, func(), error) {
	f, err := os.CreateTemp("", "*-test.txt")
	if err != nil {
		return "", nil, err
	}

	clean := func() {
		err := os.Remove(f.Name())
		if err != nil {
			panic(err)
		}
	}

	if _, err = f.Write([]byte(body)); err != nil {
		if err != nil {
			return "", nil, err
		}
	}

	if err := f.Close(); err != nil {
		return "", nil, err
	}

	return f.Name(), clean, nil
}
