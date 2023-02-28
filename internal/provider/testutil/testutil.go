package testutil

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider"
	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/config"
	"github.com/stretchr/testify/require"
)

type Config struct {
	APIKeyFromEnv string
	APIKey        string
	APIServiceURL string
}

// UnitTest is a helper around resource.UnitTest with provider factories
// already configured.
func UnitTest(t *testing.T, conf Config, c resource.TestCase) {
	t.Helper()

	if conf.APIKeyFromEnv == "" {
		os.Unsetenv(config.EnvAPIKey) // The default behavior is to ignore the environment.
	} else {
		os.Setenv(config.EnvAPIKey, conf.APIKeyFromEnv)
	}

	for i, s := range c.Steps {
		c.Steps[i].Config = compile(conf, s.Config)
	}

	f := provider.New()
	c.ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		config.ProviderName: providerserver.NewProtocol6WithError(f()),
	}
	resource.UnitTest(t, c)
}

func IntegrationTest(t *testing.T, apiKey string, c resource.TestCase) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test because go test is run with the flag -short")
	}

	if apiKey == "" {
		require.NotEmpty(t, apiKey, "envirnomental variable %s should be set for running integration tests", config.EnvTestAPIKey)
	}

	os.Setenv("TF_ACC", "on") // Enables running the integration test.
	os.Setenv(config.EnvAPIKey, apiKey)

	f := provider.New()
	c.ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		config.ProviderName: providerserver.NewProtocol6WithError(f()),
	}
	resource.Test(t, c)
}

func compile(conf Config, c string) string {
	for _, kvp := range []struct {
		key     string
		value   string
		pattern string
	}{
		{
			key:     config.APIKeyAttribute,
			value:   conf.APIKey,
			pattern: config.UnitTestReplaceWithAPIKey,
		},
		{
			key:     config.APIServiceURLAttribute,
			value:   conf.APIServiceURL,
			pattern: config.UnitTestReplaceWithAPIServiceURL,
		},
	} {
		if kvp.value != "" {
			v := fmt.Sprintf(`%s = %q`, kvp.key, kvp.value)
			c = strings.ReplaceAll(c, kvp.pattern, v)
		}
	}

	return c
}
