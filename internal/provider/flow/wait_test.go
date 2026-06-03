package flow

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
	"github.com/stretchr/testify/require"
)

func TestFlowFieldAvailable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value *string
		want  bool
	}{
		{name: "nil", value: nil, want: false},
		{name: "empty", value: util.Ptr(""), want: false},
		{name: "unknown lowercase", value: util.Ptr("unknown"), want: false},
		{name: "unknown capitalized", value: util.Ptr("Unknown"), want: false},
		{name: "unknown uppercase", value: util.Ptr("UNKNOWN"), want: false},
		{name: "valid", value: util.Ptr("adam_ss_flow_rw"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, flowFieldAvailable(tt.value))
		})
	}
}
