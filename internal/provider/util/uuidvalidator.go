package util

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = uuidValidator{}

// uuidValidator validates that a string Attribute's value matches the UUID format.
type uuidValidator struct {
	message string
}

// Description describes the validation in plain text formatting.
func (v uuidValidator) Description(_ context.Context) string {
	if v.message != "" {
		return v.message
	}

	return "value must be a valid UUID"
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v uuidValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// Validate performs the validation.
func (v uuidValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()
	if _, err := uuid.Parse(value); err != nil {
		v.message = err.Error()
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}

// NewUUIDValidator returns an AttributeValidator which ensures that any configured
// attribute value:
//
//   - Is a string.
//   - Matches the UUID format.
//
// Null (unconfigured) and unknown (known after apply) values are skipped.
func NewUUIDValidator() validator.String {
	return uuidValidator{}
}
