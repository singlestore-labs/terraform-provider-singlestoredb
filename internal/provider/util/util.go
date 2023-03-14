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
func Deref[T any](a *T) T {
	var result T
	if a == nil {
		return result
	}

	result = *a

	return result
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

// Map applies the function f to each element of the input list and returns a new
// list containing the results. The input list is not modified. The function f
// should take an element of the input list as its argument and return a value
// of a different type. The output list has the same length as the input list.
func Map[A, B any](as []A, f func(A) B) []B {
	result := make([]B, 0, len(as))
	for _, a := range as {
		result = append(result, f(a))
	}

	return result
}
