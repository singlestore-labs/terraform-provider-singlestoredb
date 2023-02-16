package util

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// DataSourceTypeName constructs the type name for the data source of the provider.
func DataSourceTypeName(req datasource.MetadataRequest, name string) string {
	return strings.Join([]string{req.ProviderTypeName, name}, "_")
}
