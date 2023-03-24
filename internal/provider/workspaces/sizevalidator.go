package workspaces

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

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

	return "value must be a valid workspace size such as 0 (suspended), 0.25 (S-00), 0.5 (S-0), 1 (S-1) or 2 (S-2)"
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
	if value == "0" || value == "0.25" || value == "0.5" {
		return nil
	}

	if strings.HasPrefix(value, "S-") {
		return errors.New("workspace size should be of the strict decimal form, such as 0 (suspended), 0.25 (S-00), 0.5 (S-0), 1 (S-1) or 2 (S-2) without the dot for 0 (suspended) or sizes bigger then 0.5")
	}

	_, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err //nolint
	}

	if strings.Contains(value, ".") {
		return errors.New("workspace size should be of the strict decimal form, such as 0 (suspended), 0.25 (S-00), 0.5 (S-0), 1 (S-1) or 2 (S-2) without the dot for 0 (suspended) or sizes bigger then 0.5")
	}

	return nil
}
