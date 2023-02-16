package util

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

// DataSourceTypeName constructs the type name for the data source of the provider.
func DataSourceTypeName(req datasource.MetadataRequest, name string) string {
	return strings.Join([]string{req.ProviderTypeName, name}, "_")
}

// Deref returns the value under the pointer.
//
// If the pointer is nil, it returns an empty value.
func Deref[T any](a *T) (result T) {
	if a == nil {
		return
	}

	result = *a
	return
}
