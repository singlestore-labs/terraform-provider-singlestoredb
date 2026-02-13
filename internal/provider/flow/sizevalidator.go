package flow

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// SizeError is the error that indicates an invalid flow size format.
type SizeError string

func (err SizeError) Error() string {
	return fmt.Sprintf("expecting size of the flow (in flow size notation), such as F1, F2, or F3; got %q", string(err))
}

var _ validator.String = sizeValidator{}

// sizeValidator validates that a string Attribute's value matches the flow size format.
type sizeValidator struct {
	message string
}

// Description describes the validation in plain text formatting.
func (v sizeValidator) Description(_ context.Context) string {
	if v.message != "" {
		return v.message
	}

	return "value must be a valid flow size such as F1, F2, or F3"
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v sizeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// Validate performs the validation.
func (v sizeValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()
	if err := ValidateTerraformSize(value); err != nil {
		v.message = err.Error()
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}

// NewSizeValidator returns an AttributeValidator which ensures that any configured
// attribute value:
//
//   - Is a string.
//   - Matches the flow size format.
//
// Null (unconfigured) and unknown (known after apply) values are skipped.
func NewSizeValidator() validator.String {
	return sizeValidator{}
}

func ValidateTerraformSize(value string) error {
	prefix := "F"
	if !strings.HasPrefix(value, prefix) {
		return SizeError(value)
	}

	if len(value) < len(prefix)+1 {
		return SizeError(value)
	}

	_, err := strconv.ParseInt(value[1:], 10, 64)
	if err != nil {
		return SizeError(value)
	}

	return nil
}
