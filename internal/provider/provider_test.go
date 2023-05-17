package provider_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/examples"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

func TestProviderAuthenticates(t *testing.T) {
	apiKey := "buzz"
	actualAPIKey := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualAPIKey = r.Header.Get("Authorization")
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        apiKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
			},
		},
	})

	require.Equal(t, fmt.Sprintf("Bearer %s", apiKey), actualAPIKey)
}

func TestProviderAuthenticationError(t *testing.T) {
	apiKey := "foo"
	actualAPIKey := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualAPIKey = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        apiKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.Regions,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})

	require.Equal(t, fmt.Sprintf("Bearer %s", apiKey), actualAPIKey)
}

func TestProviderAuthenticatesFromEnv(t *testing.T) {
	apiKey := "buzz"
	actualAPIKey := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualAPIKey = r.Header.Get("Authorization")
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKeyFromEnv: apiKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
			},
		},
	})

	require.Equal(t, fmt.Sprintf("Bearer %s", apiKey), actualAPIKey)
}

func TestProviderAuthenticatesFromAPIKeyPath(t *testing.T) {
	apiKey := "bar"

	apiKeyPath, clean, err := testutil.CreateTemp(apiKey)
	require.NoError(t, err)

	defer clean()

	actualAPIKey := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualAPIKey = r.Header.Get("Authorization")
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.Regions).
					WithAPIKeyPath(apiKeyPath).
					String(),
			},
		},
	})

	require.Equal(t, fmt.Sprintf("Bearer %s", apiKey), actualAPIKey)
}

func TestProviderAuthenticationErrorFromAPIKeyPathIfNoSuchFile(t *testing.T) {
	apiKey := "bar"

	apiKeyPath, clean, err := testutil.CreateTemp(apiKey)
	require.NoError(t, err)

	clean()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Fail(t, "should not get here because should error with no '%s' file, yet got here and called some Management API endpoint", config.APIKeyPathAttribute)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.Regions).
					WithAPIKeyPath(apiKeyPath).
					String(),
				ExpectError: regexp.MustCompile(apiKeyPath),
			},
		},
	})
}

func TestProviderAuthenticationErrorFromAPIKeyPathIfEmptyFile(t *testing.T) {
	apiKey := ""

	apiKeyPath, clean, err := testutil.CreateTemp(apiKey)
	require.NoError(t, err)

	defer clean()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Fail(t, "should not get here because should error with empty '%s' file, yet got here and called some Management API endpoint", config.APIKeyPathAttribute)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.Regions).
					WithAPIKeyPath(apiKeyPath).
					String(),
				ExpectError: regexp.MustCompile(apiKeyPath),
			},
		},
	})
}

func TestProviderAuthenticationErrorIfBothAPIKeyAndAPIKeyPath(t *testing.T) {
	apiKey := "foo"

	apiKeyPath, clean, err := testutil.CreateTemp(apiKey)
	require.NoError(t, err)

	defer clean()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Fail(t, "should not get here because should error with '%s' and '%s' specified, yet got here and called some Management API endpoint", config.APIKeyAttribute, config.APIKeyPathAttribute)
	}))
	defer server.Close()

	testutil.UnitTest(t, testutil.UnitTestConfig{
		APIServiceURL: server.URL,
		APIKey:        apiKey,
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testutil.UpdatableConfig(examples.Regions).
					WithAPIKeyPath(apiKeyPath).
					String(),
				ExpectError: regexp.MustCompile(config.APIKeyPathAttribute),
			},
		},
	})
}

func TestProviderAuthenticationErrorIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: "foo",
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config:      examples.Regions,
				ExpectError: regexp.MustCompile(http.StatusText(http.StatusUnauthorized)),
			},
		},
	})
}

func TestProviderAuthenticatesIntegration(t *testing.T) {
	testutil.IntegrationTest(t, testutil.IntegrationTestConfig{
		APIKey: os.Getenv(config.EnvTestAPIKey),
	}, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: examples.Regions,
			},
		},
	})
}
