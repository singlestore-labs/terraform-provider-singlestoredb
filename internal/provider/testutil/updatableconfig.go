package testutil

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/workspaces"
	"github.com/zclconf/go-cty/cty"
)

// UpdatableConfig is the convenience for updating the config example for tests.
type UpdatableConfig string

// AttributeSetter is a type for setting an hcl attribute for a provider, data source, or resource.
type AttributeSetter func(name string, val cty.Value) UpdatableConfig

// WithOverride replaces k with v.
func (uc UpdatableConfig) WithOverride(k, v string) UpdatableConfig {
	return UpdatableConfig(strings.ReplaceAll(uc.String(), k, v))
}

func (uc UpdatableConfig) WithWorkspace(workspaceName string) AttributeSetter {
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

// String shows the result.
func (uc UpdatableConfig) String() string {
	return string(uc)
}
