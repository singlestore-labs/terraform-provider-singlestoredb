package util_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestSizeValidator(t *testing.T) {
	ctx := t.Context()
	format := util.SizeFormat{
		Kind:     "cluster",
		Notation: "workspace size notation",
		Prefix:   "S-",
		Examples: "S-00, S-0, S-1, or S-2",
	}

	require.NoError(t, format.Validate("S-00"))
	for _, value := range []string{"", "S-", "X-00", "S-abc", "S--1"} {
		err := format.Validate(value)
		require.ErrorAs(t, err, &util.SizeError{})
		require.Contains(t, err.Error(), value)
	}

	v := util.NewSizeValidator(format)
	require.NotEmpty(t, v.Description(ctx))
	require.Equal(t, v.Description(ctx), v.MarkdownDescription(ctx))

	resp := &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringNull()}, resp)
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringUnknown()}, resp)
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("S-1")}, resp)
	require.False(t, resp.Diagnostics.HasError())

	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("nope")}, resp)
	require.True(t, resp.Diagnostics.HasError())
}
