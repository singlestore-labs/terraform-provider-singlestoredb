package workspaces

import (
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

var sizeFormat = util.SizeFormat{
	Kind:     "workspace",
	Notation: "workspace size notation",
	Prefix:   "S-",
	Examples: "S-00, S-0, S-1, or S-2",
}

// NewSizeValidator returns a validator for the workspace size format (S-00, S-0, S-1, or S-2).
func NewSizeValidator() validator.String {
	return util.NewSizeValidator(sizeFormat)
}

// ValidateTerraformSize returns an error unless value matches the workspace size format.
func ValidateTerraformSize(value string) error {
	return sizeFormat.Validate(value)
}
