package util

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = timeValidator{}

// timeValidator validates that a string Attribute's value matches the expected time format.
type timeValidator struct {
	regexp  *regexp.Regexp
	message string
}

// Description describes the validation in plain text formatting.
func (validator timeValidator) Description(_ context.Context) string {
	if validator.message != "" {
		return validator.message
	}

	return "value must be an RFC3339 time string"
}

// MarkdownDescription describes the validation in Markdown formatting.
func (validator timeValidator) MarkdownDescription(ctx context.Context) string {
	return validator.Description(ctx)
}

// Validate performs the validation.
func (v timeValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()
	_, err := parseTime(value)
	if err != nil {
		v.message = err.Error()
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}

// NewTimeValidator returns an AttributeValidator which ensures that any configured
// attribute value:
//
//   - Is a string.
//   - Matches the string format RFC3339.
//
// Null (unconfigured) and unknown (known after apply) values are skipped.
func NewTimeValidator() validator.String {
	return timeValidator{}
}

// parseTime parses time in RFC3339.
func parseTime(timeString string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, timeString)
	if err != nil {
		return time.Time{}, fmt.Errorf("should be an RFC3339 string, e.g., %q", "2222-01-01T00:00:00Z")
	}

	return t.UTC(), nil
}
