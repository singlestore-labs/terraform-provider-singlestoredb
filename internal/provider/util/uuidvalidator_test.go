package util_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestUUIDValidator(t *testing.T) {
	ctx := context.Background()

	v := util.NewUUIDValidator()
	defaultMessage := v.Description(ctx)
	require.NotEmpty(t, defaultMessage)
	require.NotEmpty(t, v.MarkdownDescription(ctx))

	v = util.NewUUIDValidator()
	resp := &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{}, resp)
	require.Empty(t, resp.Diagnostics, "not set string is fine")
	require.Equal(t, defaultMessage, v.Description(ctx), "not set string is fine")

	v = util.NewUUIDValidator()
	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("tomorrow")}, resp)
	require.NotEmpty(t, resp.Diagnostics)
	require.NotEqual(t, defaultMessage, v.Description(ctx), "shows the error")

	v = util.NewUUIDValidator()
	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("not-uuid")}, resp)
	require.NotEmpty(t, resp.Diagnostics)
	require.NotEqual(t, defaultMessage, v.Description(ctx), "shows the error")

	v = util.NewUUIDValidator()
	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("9966fccf-5116-437e-a34f-008ee32e8d94")}, resp)
	require.Empty(t, resp.Diagnostics)
	require.Equal(t, defaultMessage, v.Description(ctx))
}
