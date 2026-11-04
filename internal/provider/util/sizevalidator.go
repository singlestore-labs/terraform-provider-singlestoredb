package util

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// SizeFormat describes a size notation: a fixed prefix followed by one or more digits,
// for example S-00 for workspaces and clusters or F1 for flow.
type SizeFormat struct {
	// Kind names the sized object in messages, for example "workspace".
	Kind string
	// Notation names the notation in messages, for example "workspace size notation".
	Notation string
	Prefix   string
	// Examples lists valid sizes in messages, for example "S-00, S-0, S-1, or S-2".
	Examples string
}

// SizeError is the error that indicates an invalid size format.
type SizeError struct {
	Format SizeFormat
	Value  string
}

func (err SizeError) Error() string {
	return fmt.Sprintf("expecting size of the %s (in %s), such as %s; got %q",
		err.Format.Kind, err.Format.Notation, err.Format.Examples, err.Value)
}

// Validate returns a SizeError unless value is the prefix followed by digits only.
// Signs are rejected, so S--1 and S-+1 are invalid.
func (f SizeFormat) Validate(value string) error {
	digits, ok := strings.CutPrefix(value, f.Prefix)
	if !ok || digits == "" {
		return SizeError{Format: f, Value: value}
	}

	for _, r := range digits {
		if r < '0' || r > '9' {
			return SizeError{Format: f, Value: value}
		}
	}

	return nil
}

var _ validator.String = sizeValidator{}

type sizeValidator struct {
	format  SizeFormat
	message string
}

// Description describes the validation in plain text formatting.
func (v sizeValidator) Description(_ context.Context) string {
	if v.message != "" {
		return v.message
	}

	return fmt.Sprintf("value must be a valid %s size such as %s", v.format.Kind, v.format.Examples)
}

// MarkdownDescription describes the validation in Markdown formatting.
func (v sizeValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateString performs the validation.
func (v sizeValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()
	if err := v.format.Validate(value); err != nil {
		v.message = err.Error()
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}

// NewSizeValidator returns a validator that ensures a configured string matches format.
// Null (unconfigured) and unknown (known after apply) values are skipped.
func NewSizeValidator(format SizeFormat) validator.String {
	return sizeValidator{format: format}
}
