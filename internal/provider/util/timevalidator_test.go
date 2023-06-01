package util_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestTimeValidator(t *testing.T) {
	ctx := context.Background()

	v := util.NewTimeValidator()
	defaultMessage := v.Description(ctx)
	require.NotEmpty(t, defaultMessage)
	require.NotEmpty(t, v.MarkdownDescription(ctx))

	v = util.NewTimeValidator()
	resp := &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{}, resp)
	require.Empty(t, resp.Diagnostics, "not set string is fine")
	require.Equal(t, defaultMessage, v.Description(ctx), "not set string is fine")

	v = util.NewTimeValidator()
	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("tomorrow")}, resp)
	require.NotEmpty(t, resp.Diagnostics)
	require.NotEqual(t, defaultMessage, v.Description(ctx), "shows the error")

	v = util.NewTimeValidator()
	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("tomorrow")}, resp)
	require.NotEmpty(t, resp.Diagnostics)
	require.NotEqual(t, defaultMessage, v.Description(ctx), "shows the error")

	v = util.NewTimeValidator()
	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("2222-01-01T00:00:00Z")}, resp)
	require.Empty(t, resp.Diagnostics)
	require.Equal(t, defaultMessage, v.Description(ctx))

	v = util.NewTimeValidator()
	resp = &validator.StringResponse{}
	v.ValidateString(ctx, validator.StringRequest{ConfigValue: types.StringValue("2222-01-01T00:00:00+07:00")}, resp)
	require.NotEmpty(t, resp.Diagnostics)
	require.NotEqual(t, defaultMessage, v.Description(ctx), "requires UTC")
}
