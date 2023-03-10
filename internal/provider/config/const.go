package config

import "time"

const (
	// Version is the version of the provider.
	Version = "0.0.0"
	// APIKeyAttribute defines the API key as a part of the provider configuration.
	APIKeyAttribute = "api_key"
	// APIServiceURLAttribute defines the Management API server URL part of the provider configuration.
	APIServiceURLAttribute = "api_service_url"
	// APIServiceURL is the default URL for the SingleStore Management API service.
	APIServiceURL = "https://api.singlestore.com"
	// EnvAPIKey is the environmental variable for fetching the API key.
	EnvAPIKey = "SINGLESTORE_API_KEY"
	// EnvTestAPIKey is the environmental variable for API key for integration tests.
	EnvTestAPIKey = "TEST_SINGLESTORE_API_KEY"
	// ProviderName is the name of the provider.
	ProviderName = "singlestore"
	// TestIDAttribute is the test only ID attribute.
	TestIDAttribute = "id"
	// TestIDValue indicates the value of the test only ID field.
	TestIDValue = "internal"
	// UnitTestReplaceWithAPIKey converts an example tf file into a unit test config.
	UnitTestReplaceWithAPIKey = "#unit_test_replace_with_api_key" //nolint
	// UnitTestReplaceWihtAPIServiceURL converts an example tf file into a unit test config.
	UnitTestReplaceWithAPIServiceURL = "#unit_test_replace_with_api_service_url"
	// HTTPRequestTimeout limits all the calls to Management API by 10 seconds.
	HTTPRequestTimeout = time.Second * 10
	// WorkspaceGroupCreationTimeout limits the workspace group creation time to 1 hour.
	WorkspaceGroupCreationTimeout = time.Hour
)
