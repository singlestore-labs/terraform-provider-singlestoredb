package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	// APIKeyAttribute defines the API key as a part of the provider configuration.
	APIKeyAttribute = "api_key"
	// APIServiceURLAttribute defines the Management API server URL part of the provider configuration.
	APIServiceURLAttribute = "api_service_url"
	// IDAttribute is the idiomatic Terraform ID attribute.
	IDAttribute = "id"
	// WorkspaceGroupIDAttribute is the attribute of a workspace list data source.
	WorkspaceGroupIDAttribute = "workspace_group_id"
	// APIServiceURL is the default URL for the SingleStore Management API service.
	APIServiceURL = "https://api.singlestore.com"
	// EnvAPIKey is the environmental variable for fetching the API key.
	EnvAPIKey = "SINGLESTOREDB_API_KEY"
	// ProviderName is the name of the provider.
	ProviderName = "singlestoredb"
	// HTTPRequestTimeout limits all the calls to Management API by 10 seconds.
	HTTPRequestTimeout = time.Second * 10
	// WorkspaceGroupCreationTimeout limits the workspace group creation time.
	WorkspaceGroupCreationTimeout = time.Hour
	// WorkspaceReadTimeout limits the workspace creation time.
	WorkspaceReadTimeout = 10 * time.Minute
	// WorkspaceCreationTimeout limits the workspace creation time.
	WorkspaceCreationTimeout = 5 * time.Hour
	// WorkspaceResumeTimeout limits the workspace resume time.
	WorkspaceResumeTimeout = 6 * time.Hour
	// WorkspaceScaleTakesAtLeast ensures the least required time for scaling.
	WorkspaceScaleTakesAtLeast = 30 * time.Second
	// PortalAPIKeysPageRedirect redirects to the API keys page of the default organization.
	PortalAPIKeysPageRedirect = "https://portal.singlestore.com/organizations/org-id/api-keys" //nolint:gosec
	// SupportURL directs to SingleStore support.
	SupportURL = "https://www.singlestore.com/support/"
	// ProviderNewIssueURL  direct to creating a GitHub issue for the provider.
	ProviderNewIssueURL = "https://github.com/singlestore-labs/terraform-provider-singlestoredb/issues/new"
	// WorkspaceGroupConsistencyThreshold is the count of polling iterations where the state should equal the desired state.
	WorkspaceGroupConsistencyThreshold = 5
	// WorkspaceConsistencyThreshold is the count of polling iterations where the state should equal the desired state.
	WorkspaceConsistencyThreshold = 5

	// TestIDValue indicates the value of the test only ID field.
	TestIDValue = "internal"
	// EnvTestAPIKey is the environmental variable for API key for integration tests.
	EnvTestAPIKey = "TEST_SINGLESTOREDB_API_KEY"
	// TestInitialWorkspaceGroupName is the default workspace group name in examples.
	TestInitialWorkspaceGroupName = "group"
	// TestInitialWorkspaceGroupExpiresAt is the initial workspace group expiration in examples.
	TestInitialWorkspaceGroupExpiresAt = "2222-01-01T00:00:00Z"
	// TestInitialAdminPassword is the initial workspace admin password in examples.
	TestInitialAdminPassword = "fooBAR12$"
	// TestInitialFirewallRange is the firewall range in the example.
	TestInitialFirewallRange = "0.0.0.0/0"
	// TestFirewallRangeAllTraffic is the firewall range in the example for allowing all traffic.
	TestFirewallFirewallRangeAllTraffic = "0.0.0.0/0"
	// TestInitialWorkspaceID is the workspace ID in the example.
	TestInitialWorkspaceID = "26171125-ecb8-5944-9896-209fbffc1f15"
	// TestWorkspaceName is the default workspace name in examples.
	TestWorkspaceName = "workspace"
	// TestWorkspaceDeploymentType is the default workspace deployment type in examples.
	TestWorkspaceDeploymentType = "NON-PRODUCTION"
	// TestInitialWorkspaceSize is the default workspace size in examples.
	TestInitialWorkspaceSize = "S-00"
	// TestMaxIdleConns is the maximum number of idle connections for a SQL mysql connection for tests.
	TestMaxIdleConns = 16
	// TestMaxOpenConns is the maximum number of open connections for a SQL mysql connection for tests.
	TestMaxOpenConns = 64
	// TestWorkspaceGroupExpiration is the time after which a workspace group auto-terminates.
	// This is an extra safeguard to cleanup resources after running integration tests.
	TestWorkspaceGroupExpiration = 2 * time.Hour
	// ResourceTypeName is a type name for accessing resource objects in *.tf files. The other types are data source and provider.
	ResourceTypeName = "resource"
	// DataSourceTypeName is a type name for accessing data source objects in *.tf files. The other types are resource and provider.
	DataSourceTypeName = "data"
	// ProviderTypeName is a type name for accessing provider objects in *.tf files. The other types are resource and data source.
	ProviderTypeName = "provider"
)

var (
	// APIKeyPathAttribute defines the API key path as a part of the provider configuration.
	APIKeyPathAttribute      = strings.Join([]string{APIKeyAttribute, "path"}, "_")
	InvalidAPIKeyErrorDetail = fmt.Sprintf("Ensure a valid API key is created at %s and it's provided in one of the following ways: \n1. Directly set as the '%s' attribute in the provider configuration.\n2. Stored in a file with its absolute path set in the '%s' attribute.\n3. Set as the '%s' environment variable.",
		PortalAPIKeysPageRedirect,
		APIKeyAttribute,
		APIKeyPathAttribute,
		EnvAPIKey,
	)
	ContactSupportErrorDetail                = fmt.Sprintf("Contact SingleStore support %s.", SupportURL)
	ContactSupportLaterErrorDetail           = fmt.Sprintf("If nothing changes in a few hours, contact SingleStore support %s.", SupportURL)
	CreateProviderIssueErrorDetail           = fmt.Sprintf("Internal errror took place. Please, report the issue %s.", ProviderNewIssueURL)
	CreateProviderIssueIfNotClearErrorDetail = fmt.Sprintf("If the error is not clear, please report the issue %s.", SupportURL)
)
