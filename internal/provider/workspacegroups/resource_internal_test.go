package workspacegroups

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/singlestore-labs/singlestore-go/management"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestToManagementUpdateWindow(t *testing.T) {
	t.Run("null object returns nil", func(t *testing.T) {
		result := toManagementUpdateWindow(t.Context(), types.ObjectNull(map[string]attr.Type{
			"hour": types.Int64Type,
			"day":  types.Int64Type,
		}))
		require.Nil(t, result)
	})

	t.Run("unknown object returns nil", func(t *testing.T) {
		result := toManagementUpdateWindow(t.Context(), types.ObjectUnknown(map[string]attr.Type{
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

		result := toManagementUpdateWindow(t.Context(), obj)
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

		result := toManagementUpdateWindow(t.Context(), obj)
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
		diags := result.As(t.Context(), &model, basetypes.ObjectAsOptions{})
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
		diags := result.As(t.Context(), &model, basetypes.ObjectAsOptions{})
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
		diags := result.As(t.Context(), &model, basetypes.ObjectAsOptions{})
		require.False(t, diags.HasError())

		require.Equal(t, int64(10), model.Hour.ValueInt64())
		require.Equal(t, int64(5), model.Day.ValueInt64())
	})
}

func TestApplyWorkspaceGroupRegion(t *testing.T) {
	t.Run("keeps a previously configured region_id", func(t *testing.T) {
		model := workspaceGroupResourceModel{}
		prior := types.StringValue("3c0c0d99-3c09-45ac-a01f-5ab62afd35cf")
		applyWorkspaceGroupRegion(&model, management.Cluster{
			Provider: util.Ptr(management.CloudProviderAWS),
			Region:   util.Ptr("us-east-1"),
		}, prior)

		require.Equal(t, prior, model.RegionID)
		require.True(t, model.CloudProvider.IsNull())
		require.True(t, model.RegionName.IsNull())
	})

	t.Run("uses provider and region name when region_id is unset", func(t *testing.T) {
		model := workspaceGroupResourceModel{}
		applyWorkspaceGroupRegion(&model, management.Cluster{
			Provider: util.Ptr(management.CloudProviderAWS),
			Region:   util.Ptr("us-east-1"),
		}, types.StringNull())

		require.True(t, model.RegionID.IsNull())
		require.Equal(t, string(management.CloudProviderAWS), model.CloudProvider.ValueString())
		require.Equal(t, "us-east-1", model.RegionName.ValueString())
	})
}

func TestValidateRequiredRegionParameters(t *testing.T) {
	regionID := types.StringValue("3c0c0d99-3c09-45ac-a01f-5ab62afd35cf")

	t.Run("existing region_id configuration is valid", func(t *testing.T) {
		err := validateRequiredRegionParameters(&workspaceGroupResourceModel{RegionID: regionID})
		require.Nil(t, err)
	})

	t.Run("cloud provider and region name are valid", func(t *testing.T) {
		err := validateRequiredRegionParameters(&workspaceGroupResourceModel{
			CloudProvider: types.StringValue("AWS"),
			RegionName:    types.StringValue("us-east-1"),
		})
		require.Nil(t, err)
	})

	t.Run("setting both region forms is invalid", func(t *testing.T) {
		err := validateRequiredRegionParameters(&workspaceGroupResourceModel{
			RegionID:      regionID,
			CloudProvider: types.StringValue("AWS"),
			RegionName:    types.StringValue("us-east-1"),
		})
		require.NotNil(t, err)
	})

	t.Run("setting neither region form is invalid", func(t *testing.T) {
		err := validateRequiredRegionParameters(&workspaceGroupResourceModel{})
		require.NotNil(t, err)
	})
}

func TestValidateWorkspaceGroupName(t *testing.T) {
	require.Nil(t, validateWorkspaceGroupName("group"))
	require.Nil(t, validateWorkspaceGroupName(strings.Repeat("n", clusterNameMaxLen)))
	require.NotNil(t, validateWorkspaceGroupName(strings.Repeat("n", clusterNameMaxLen+1)))
	require.NotNil(t, validateWorkspaceGroupName(""))
}

func TestIsNotFound(t *testing.T) {
	require.True(t, isNotFound(&util.SummaryWithDetailError{Summary: "Not Found"}))
	require.True(t, isNotFound(&util.SummaryWithDetailError{Summary: "Workspace group not found"}))
	require.False(t, isNotFound(&util.SummaryWithDetailError{Summary: "SingleStore API client call failed"}))
	require.False(t, isNotFound(nil))
}
