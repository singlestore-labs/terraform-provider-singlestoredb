package privateconnectionsv2

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func regions(values ...string) types.List {
	elems := make([]attr.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, types.StringValue(v))
	}

	return types.ListValueMust(types.StringType, elems)
}

func TestFillOmittedKeepsConfiguredSpelling(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name        string
		resultID    types.String
		priorID     types.String
		resultRegs  types.List
		priorRegs   types.List
		wantID      types.String
		wantRegions types.List
	}{
		{
			name:        "uppercase cluster ID and reordered regions keep the prior value",
			resultID:    types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			priorID:     types.StringValue("283D4B0D-B0D6-485A-BC2D-A763C523C68A"),
			resultRegs:  regions("eu-west-1", "us-west-2"),
			priorRegs:   regions("us-west-2", "eu-west-1"),
			wantID:      types.StringValue("283D4B0D-B0D6-485A-BC2D-A763C523C68A"),
			wantRegions: regions("us-west-2", "eu-west-1"),
		},
		{
			name:        "empty configured list matches no regions",
			resultID:    types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			priorID:     types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			resultRegs:  types.ListNull(types.StringType),
			priorRegs:   regions(),
			wantID:      types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			wantRegions: regions(),
		},
		{
			name:        "different values come from the response",
			resultID:    types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			priorID:     types.StringValue("458d14e6-fcc4-4985-a2a6-f1f1f15cef2f"),
			resultRegs:  regions("eu-west-1"),
			priorRegs:   regions("us-west-2"),
			wantID:      types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			wantRegions: regions("eu-west-1"),
		},
		{
			name:        "unknown prior regions are not kept",
			resultID:    types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			priorID:     types.StringUnknown(),
			resultRegs:  regions("eu-west-1"),
			priorRegs:   types.ListUnknown(types.StringType),
			wantID:      types.StringValue("283d4b0d-b0d6-485a-bc2d-a763c523c68a"),
			wantRegions: regions("eu-west-1"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := privateConnectionModel{ClusterID: test.resultID, SupportedCrossRegions: test.resultRegs}
			prior := privateConnectionModel{ClusterID: test.priorID, SupportedCrossRegions: test.priorRegs}
			fillOmitted(ctx, &result, prior)
			require.Equal(t, test.wantID, result.ClusterID)
			require.True(t, test.wantRegions.Equal(result.SupportedCrossRegions), "got %s", result.SupportedCrossRegions)
		})
	}
}
