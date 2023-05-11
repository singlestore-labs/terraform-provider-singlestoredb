package workspaces

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// SizeError is the error that indicates an invalid workspace size format.
type SizeError string

func (err SizeError) Error() string {
	return fmt.Sprintf("expecting size of the workspace (in workspace size notation), such as S-00, S-0, S-1, or S-2; got %q", string(err))
}

var _ validator.String = sizeValidator{}

// sizeValidator validates that a string Attribute's value matches the workspace size format.
type sizeValidator struct {
	message string
}

// Description describes the validation in plain text formatting.
func (v sizeValidator) Description(_ context.Context) string {
	if v.message != "" {
		return v.message
	}

	return "value must be a valid workspace size such as S-00, S-0, S-1, or S-2"
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
//   - Matches the workspace size format.
//
// Null (unconfigured) and unknown (known after apply) values are skipped.
func NewSizeValidator() validator.String {
	return sizeValidator{}
}

func ValidateTerraformSize(value string) error {
	prefix := "S-"
	if !strings.HasPrefix(value, prefix) {
		return SizeError(value)
	}

	if len(value) < len(prefix)+1 {
		return SizeError(value)
	}

	_, err := strconv.ParseInt(value[2:], 10, 64)
	if err != nil {
		return SizeError(value)
	}

	return nil
}
