package testutil

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspacegroups"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspaces"
	"github.com/zclconf/go-cty/cty"
)

// UpdatableConfig is the convenience for updating the config *.tf examples.
// This enables overriding values like an API key of the provider for testing purposes.
type UpdatableConfig string

// AttributeSetter is a type for setting an hcl attribute for a provider, data source, or resource.
type AttributeSetter func(name string, val cty.Value) UpdatableConfig

func (uc UpdatableConfig) WithWorkspaceGroupGetDataSource(workspaceGroupName string) AttributeSetter {
	return withAttribute(uc, config.DataSourceTypeName, []string{dataSourceTypeName(workspacegroups.DataSourceGetName), workspaceGroupName})
}

func (uc UpdatableConfig) WithWorkspaceGetDataSource(workspaceName string) AttributeSetter {
	return withAttribute(uc, config.DataSourceTypeName, []string{dataSourceTypeName(workspaces.DataSourceGetName), workspaceName})
}

func (uc UpdatableConfig) WithWorkspaceListDataSource(workspaceListName string) AttributeSetter {
	return withAttribute(uc, config.DataSourceTypeName, []string{dataSourceTypeName(workspaces.DataSourceListName), workspaceListName})
}

func (uc UpdatableConfig) WithWorkspaceResource(workspaceName string) AttributeSetter {
	return withAttribute(uc, config.ResourceTypeName, []string{resourceTypeName(workspaces.ResourceName), workspaceName})
}

func (uc UpdatableConfig) WithWorkspaceGroupResource(workspaceGroupName string) AttributeSetter {
	return withAttribute(uc, config.ResourceTypeName, []string{resourceTypeName(workspacegroups.ResourceName), workspaceGroupName})
}

// WithAPIKey extends the config with the API key if the key is not empty.
func (uc UpdatableConfig) WithAPIKey(apiKey string) UpdatableConfig {
	if apiKey == "" {
		return uc
	}

	return withAttribute(uc, config.ProviderTypeName, []string{config.ProviderName})(
		config.APIKeyAttribute, cty.StringVal(apiKey),
	)
}

// WithAPIKeyPath extends the config with the API key path.
func (uc UpdatableConfig) WithAPIKeyPath(apiKeyPath string) UpdatableConfig {
	return withAttribute(uc, config.ProviderTypeName, []string{config.ProviderName})(
		config.APIKeyPathAttribute, cty.StringVal(apiKeyPath),
	)
}

// WithAPIKey extends the config with the API service url if the url is not empty.
func (uc UpdatableConfig) WithAPIServiceURL(url string) UpdatableConfig {
	if url == "" {
		return uc
	}

	return withAttribute(uc, config.ProviderTypeName, []string{config.ProviderName})(
		config.APIServiceURLAttribute, cty.StringVal(url),
	)
}

// String shows the resulting *.tf config with all the overrides applied.
func (uc UpdatableConfig) String() string {
	return string(uc)
}

// withAttribute accesses a resource, data source, or a provider defined by the typeName and labels,
// that is a part of the updatable config and returns a function that enables setting an attribute.
//
// This enables reading *.tf files from examples and, in tests, overriding values like
// an API key of the provider.
func withAttribute(uc UpdatableConfig, typeName string, labels []string) AttributeSetter {
	return func(attributeName string, val cty.Value) UpdatableConfig {
		file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
		if diags.HasErrors() {
			panic(diags)
		}

		block := file.Body().FirstMatchingBlock(typeName, labels)
		if block == nil {
			message := fmt.Sprintf("config file should contain a block with %s %s to add or update an attribute",
				typeName, strings.Join(labels, "."),
			)
			panic(message)
		}
		_ = block.Body().SetAttributeValue(attributeName, val)

		return UpdatableConfig(file.Bytes())
	}
}

func resourceTypeName(name string) string {
	return strings.Join([]string{config.ProviderName, name}, "_")
}

func dataSourceTypeName(name string) string {
	return strings.Join([]string{config.ProviderName, name}, "_")
}
