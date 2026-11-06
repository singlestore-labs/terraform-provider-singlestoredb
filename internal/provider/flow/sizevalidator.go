package flow

import (
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

var sizeFormat = util.SizeFormat{
	Kind:     "flow",
	Notation: "flow size notation",
	Prefix:   "F",
	Examples: "F1, F2, or F3",
}

// NewSizeValidator returns a validator for the flow size format (F1, F2, or F3).
func NewSizeValidator() validator.String {
	return util.NewSizeValidator(sizeFormat)
}

// ValidateTerraformSize returns an error unless value matches the flow size format.
func ValidateTerraformSize(value string) error {
	return sizeFormat.Validate(value)
}
