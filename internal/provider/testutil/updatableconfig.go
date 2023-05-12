package testutil

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspacegroups"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspaces"
	"github.com/zclconf/go-cty/cty"
)

// UpdatableConfig is the convenience for updating the config example for tests.
type UpdatableConfig string

// AttributeSetter is a type for setting an hcl attribute for a provider, data source, or resource.
type AttributeSetter func(name string, val cty.Value) UpdatableConfig

func (uc UpdatableConfig) WithWorkspaceGroupGetDataSoure(workspaceGroupName string) AttributeSetter {
	return func(attributeName string, val cty.Value) UpdatableConfig {
		file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
		if diags.HasErrors() {
			panic(diags)
		}

		workspaceGroup := file.Body().FirstMatchingBlock("data", []string{
			dataSourceTypeName(workspacegroups.DataSourceGetName), workspaceGroupName,
		})
		if workspaceGroup == nil {
			panic("config file should contain a block with the workspace group data source to add or update an attribute")
		}
		_ = workspaceGroup.Body().SetAttributeValue(attributeName, val)

		return UpdatableConfig(file.Bytes())
	}
}

func (uc UpdatableConfig) WithWorkspaceGetDataSource(workspaceName string) AttributeSetter {
	return func(attributeName string, val cty.Value) UpdatableConfig {
		file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
		if diags.HasErrors() {
			panic(diags)
		}

		workspace := file.Body().FirstMatchingBlock("data", []string{
			dataSourceTypeName(workspaces.DataSourceGetName), workspaceName,
		})
		if workspace == nil {
			panic("config file should contain a block with the workspace data source to add or update an attribute")
		}
		_ = workspace.Body().SetAttributeValue(attributeName, val)

		return UpdatableConfig(file.Bytes())
	}
}

func (uc UpdatableConfig) WithWorkspaceListDataSoure(workspaceListName string) AttributeSetter {
	return func(attributeName string, val cty.Value) UpdatableConfig {
		file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
		if diags.HasErrors() {
			panic(diags)
		}

		workspaceList := file.Body().FirstMatchingBlock("data", []string{
			dataSourceTypeName(workspaces.DataSourceListName), workspaceListName,
		})
		if workspaceList == nil {
			panic("config file should contain a block with the workspace list data source to add or update an attribute")
		}
		_ = workspaceList.Body().SetAttributeValue(attributeName, val)

		return UpdatableConfig(file.Bytes())
	}
}

func (uc UpdatableConfig) WithWorkspaceResource(workspaceName string) AttributeSetter {
	return func(attributeName string, val cty.Value) UpdatableConfig {
		file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
		if diags.HasErrors() {
			panic(diags)
		}

		workspace := file.Body().FirstMatchingBlock("resource", []string{
			resourceTypeName(workspaces.ResourceName), workspaceName,
		})
		if workspace == nil {
			panic("config file should contain a block with the workspace resource to add or update an attribute")
		}
		_ = workspace.Body().SetAttributeValue(attributeName, val)

		return UpdatableConfig(file.Bytes())
	}
}

func (uc UpdatableConfig) WithWorkspaceGroupResource(workspaceGroupName string) AttributeSetter {
	return func(attributeName string, val cty.Value) UpdatableConfig {
		file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
		if diags.HasErrors() {
			panic(diags)
		}

		workspace := file.Body().FirstMatchingBlock("resource", []string{
			resourceTypeName(workspacegroups.ResourceName), workspaceGroupName,
		})
		if workspace == nil {
			panic("config file should contain a block with the workspace group resource to add or update an attribute")
		}
		_ = workspace.Body().SetAttributeValue(attributeName, val)

		return UpdatableConfig(file.Bytes())
	}
}

// WithAPIKey extends the config with the API key if the key is not empty.
func (uc UpdatableConfig) WithAPIKey(apiKey string) UpdatableConfig {
	if apiKey == "" {
		return uc
	}

	file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
	if diags.HasErrors() {
		panic(diags)
	}

	provider := file.Body().FirstMatchingBlock("provider", []string{config.ProviderName})
	if provider == nil {
		panic("config file should contain a block with the provider to add or update an attribute")
	}
	_ = provider.Body().SetAttributeValue(config.APIKeyAttribute, cty.StringVal(apiKey))

	return UpdatableConfig(file.Bytes())
}

// WithAPIKey extends the config with the API service url if the url is not empty.
func (uc UpdatableConfig) WithAPIServiceURL(url string) UpdatableConfig {
	if url == "" {
		return uc
	}

	file, diags := hclwrite.ParseConfig([]byte(uc), "", hcl.InitialPos)
	if diags.HasErrors() {
		panic(diags)
	}

	provider := file.Body().FirstMatchingBlock("provider", []string{config.ProviderName})
	if provider == nil {
		panic("config file should contain a block with the provider to add or update an attribute")
	}
	_ = provider.Body().SetAttributeValue(config.APIServiceURLAttribute, cty.StringVal(url))

	return UpdatableConfig(file.Bytes())
}

// String shows the result.
func (uc UpdatableConfig) String() string {
	return string(uc)
}
