package util

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// DataSourceTypeName constructs the type name for the data source of the provider.
func DataSourceTypeName(req datasource.MetadataRequest, name string) string {
	return strings.Join([]string{req.ProviderTypeName, name}, "_")
}

// ResourceTypeName constructs the type name for the resource of the provider.
func ResourceTypeName(req resource.MetadataRequest, name string) string {
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

// Ptr returns a pointer to the input value.
func Ptr[A any](a A) *A {
	return &a
}

// FirstNotEmpty returns the first encountered not empty string if present.
func FirstNotEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}

	return ""
}

// FirstSetStringValue returns the first set string value.
// If not found, it returns an unset string.
func FirstSetStringValue(ss ...types.String) types.String {
	for _, s := range ss {
		if !s.IsNull() && !s.IsUnknown() {
			return s
		}
	}

	return types.StringNull()
}
