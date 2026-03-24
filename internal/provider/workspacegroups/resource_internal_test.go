package workspacegroups

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/stretchr/testify/require"
)

func TestToManagementUpdateWindow(t *testing.T) {
	t.Run("null object returns nil", func(t *testing.T) {
		result := toManagementUpdateWindow(context.Background(), types.ObjectNull(map[string]attr.Type{
			"hour": types.Int64Type,
			"day":  types.Int64Type,
		}))
		require.Nil(t, result)
	})

	t.Run("unknown object returns nil", func(t *testing.T) {
		result := toManagementUpdateWindow(context.Background(), types.ObjectUnknown(map[string]attr.Type{
			"hour": types.Int64Type,
			"day":  types.Int64Type,
		}))
		require.Nil(t, result)
	})

	t.Run("valid object converts correctly", func(t *testing.T) {
		obj, diags := types.ObjectValue(
			map[string]attr.Type{
				"hour": types.Int64Type,
				"day":  types.Int64Type,
			},
			map[string]attr.Value{
				"hour": types.Int64Value(12),
				"day":  types.Int64Value(3),
			},
		)
		require.False(t, diags.HasError())

		result := toManagementUpdateWindow(context.Background(), obj)
		require.NotNil(t, result)
		require.Equal(t, float32(12), result.Hour)
		require.Equal(t, float32(3), result.Day)
	})

	t.Run("boundary values", func(t *testing.T) {
		obj, diags := types.ObjectValue(
			map[string]attr.Type{
				"hour": types.Int64Type,
				"day":  types.Int64Type,
			},
			map[string]attr.Value{
				"hour": types.Int64Value(0),
				"day":  types.Int64Value(6),
			},
		)
		require.False(t, diags.HasError())

		result := toManagementUpdateWindow(context.Background(), obj)
		require.NotNil(t, result)
		require.Equal(t, float32(0), result.Hour)
		require.Equal(t, float32(6), result.Day)
	})
}

func TestToUpdateWindowResourceModel(t *testing.T) {
	t.Run("nil pointer returns null object", func(t *testing.T) {
		result := toUpdateWindowResourceModel(nil)
		require.True(t, result.IsNull())
	})

	t.Run("valid update window converts correctly", func(t *testing.T) {
		uw := &management.UpdateWindow{
			Hour: 15,
			Day:  2,
		}

		result := toUpdateWindowResourceModel(uw)
		require.False(t, result.IsNull())

		var model updateWindowResourceModel
		diags := result.As(context.Background(), &model, basetypes.ObjectAsOptions{})
		require.False(t, diags.HasError())

		require.Equal(t, int64(15), model.Hour.ValueInt64())
		require.Equal(t, int64(2), model.Day.ValueInt64())
	})

	t.Run("boundary values", func(t *testing.T) {
		uw := &management.UpdateWindow{
			Hour: 23,
			Day:  0,
		}

		result := toUpdateWindowResourceModel(uw)
		require.False(t, result.IsNull())

		var model updateWindowResourceModel
		diags := result.As(context.Background(), &model, basetypes.ObjectAsOptions{})
		require.False(t, diags.HasError())

		require.Equal(t, int64(23), model.Hour.ValueInt64())
		require.Equal(t, int64(0), model.Day.ValueInt64())
	})

	t.Run("float values are properly converted to int64", func(t *testing.T) {
		uw := &management.UpdateWindow{
			Hour: 10.0,
			Day:  5.0,
		}

		result := toUpdateWindowResourceModel(uw)
		require.False(t, result.IsNull())

		var model updateWindowResourceModel
		diags := result.As(context.Background(), &model, basetypes.ObjectAsOptions{})
		require.False(t, diags.HasError())

		require.Equal(t, int64(10), model.Hour.ValueInt64())
		require.Equal(t, int64(5), model.Day.ValueInt64())
	})
}
