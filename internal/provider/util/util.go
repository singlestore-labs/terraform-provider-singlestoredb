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

// MapList converts the list of type A into the list of type B using the convert.
func MapList[A, B any](input []A, convert func(A) B) (result []B) {
	for _, i := range input {
		result = append(result, convert(i))
	}

	return
}

// Maybe performs the conversion if input is not nil.
func Maybe[A, B any](input *A, convert func(A) B) *B {
	if input == nil {
		return nil
	}

	result := convert(*input)
	return &result
}

// Ptr returns a pointer to the input value.
func Ptr[A any](a A) *A {
	return &a
}
