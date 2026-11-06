package clusters

import (
	"context"
	"math"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// wholeNumberValidator rejects fractional values for attributes the Management API
// stores as integers. A fractional value would be truncated on send and read back
// as a different number than the plan.
type wholeNumberValidator struct{}

var _ validator.Float32 = wholeNumberValidator{}

func (wholeNumberValidator) Description(_ context.Context) string {
	return "value must be a whole number"
}

func (v wholeNumberValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateFloat32 rejects fractional numbers. The Management API stores this attribute as an integer.
func (v wholeNumberValidator) ValidateFloat32(ctx context.Context, req validator.Float32Request, resp *validator.Float32Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := float64(req.ConfigValue.ValueFloat32())
	if value != math.Trunc(value) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", v.Description(ctx))
	}
}
