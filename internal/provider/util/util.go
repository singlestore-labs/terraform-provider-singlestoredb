package util

import (
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
)

type SummaryWithDetailError struct {
	Summary string
	Detail  string
}

func (swd SummaryWithDetailError) Error() string {
	return fmt.Sprintf("%s: %s", swd.Summary, swd.Detail)
}

// TerraformProviderUserAgent identifies the provider as a versioned User Agent.
func TerraformProviderUserAgent(version string) string {
	return fmt.Sprintf("terraform-provider-%s/%s", config.ProviderName, version)
}

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

// MapWithError applies the function f to each element of the input list and returns a new
// list containing the results. The input list is not modified. The function f
// should take an element of the input list as its argument and return a value
// of a different type. The output list has the same length as the input list.
// If the converter returns an error, the first error is returned and no result.
func MapWithError[A, B any](as []A, f func(A) (B, *SummaryWithDetailError)) ([]B, *SummaryWithDetailError) {
	result := make([]B, 0, len(as))
	for _, a := range as {
		r, err := f(a)
		if err != nil {
			return nil, err
		}

		result = append(result, r)
	}

	return result, nil
}

func Join[A any](ss []A, separator string) string {
	result := Map(ss, func(s A) string {
		return fmt.Sprintf("%v", s)
	})

	return strings.Join(result, separator)
}

// CheckLastN returns true if the last n elements of the array are equal to any of the values.
// If the array does not have the desired count of elements, it returns false.
func CheckLastN[T comparable](ts []T, n int, values ...T) bool {
	if len(ts) < n {
		return false
	}

	for i := len(ts) - n; i < len(ts); i++ {
		if !Any(values, ts[i]) {
			return false
		}
	}

	return true
}

// Any returns true if any element of the array is equal to the value.
func Any[T comparable](ts []T, value T) bool {
	for _, t := range ts {
		if t == value {
			return true
		}
	}

	return false
}

// ReadNotEmptyFileTrimmed reads the file at path and returns the white space trimmed non-empty content.
func ReadNotEmptyFileTrimmed(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path '%s' is not an absolute file path", path)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(body))
	if len(result) == 0 {
		return "", fmt.Errorf("file '%s' is empty", path)
	}

	return result, nil
}

func SubtractListValues(a, b []types.String) []types.String {
	bSet := make(map[string]struct{})
	for _, v := range b {
		bSet[v.ValueString()] = struct{}{}
	}

	var result []types.String
	for _, v := range a {
		if _, exists := bSet[v.ValueString()]; !exists {
			result = append(result, v)
		}
	}

	return result
}

func IsValidEmail(email string) bool {
	_, err := mail.ParseAddress(email)

	return err == nil
}

func ValidateEmails(emails []string) error {
	for _, email := range emails {
		if !IsValidEmail(email) {
			return fmt.Errorf("invalid email address: %s", email)
		}
	}

	return nil
}
