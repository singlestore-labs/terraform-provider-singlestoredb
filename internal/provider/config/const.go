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
	// IDAttribute is the idiomatic Terraform ID attribute.
	IDAttribute = "id"
	// TestIDValue indicates the value of the test only ID field.
	TestIDValue = "internal"
	// TestReplaceWithAPIKey converts an example tf file into a unit test config.
	TestReplaceWithAPIKey = "#test_replace_with_api_key"
	// TestReplaceWithAPIServiceURL converts an example tf file into a unit test config.
	TestReplaceWithAPIServiceURL = "#test_replace_with_api_service_url"
	// HTTPRequestTimeout limits all the calls to Management API by 10 seconds.
	HTTPRequestTimeout = time.Second * 10
	// WorkspaceGroupCreationTimeout limits the workspace group creation time to 1 hour.
	WorkspaceGroupCreationTimeout = time.Hour
	// TestInitialWorkspaceGroupName is the default workspace group name in examples.
	TestInitialWorkspaceGroupName = "terraform-provider-ci-integration-test-workspace-group"
	// TestInitialWorkspaceGroupExpiresAt is the initial workspace group expiration in examples.
	TestInitialWorkspaceGroupExpiresAt = "2222-01-01T00:00:00Z"
	// TestInitialAdminPassword is the initial workspace admin password in examples.
	TestInitialAdminPassword = "fooBAR12$"
	// TestInitialWorkspaceGroupID is the workspace group ID in the example.
	TestInitialWorkspaceGroupID = "bc8c0deb-50dd-4a58-a5a5-1c62eb5c456d"
	// TestInitialFirewallRange is the firewall range in the example.
	TestInitialFirewallRange = "192.168.0.1/32"
	// TestInitialWorkspaceID is the workspace ID in the example.
	TestInitialWorkspaceID = "26171125-ecb8-5944-9896-209fbffc1f15"
)
