package sql_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/sql"
	"github.com/stretchr/testify/require"
)

func TestDataAPIURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "host only",
			input: "svc-abc.aws-east-1.svc.singlestore.com",
			want:  "https://svc-abc.aws-east-1.svc.singlestore.com",
		},
		{
			name:  "host with port suffix",
			input: "svc-abc.aws-east-1.svc.singlestore.com:3306",
			want:  "https://svc-abc.aws-east-1.svc.singlestore.com",
		},
		{
			name:  "strips any port suffix",
			input: "svc-abc.aws-east-1.svc.singlestore.com:8080",
			want:  "https://svc-abc.aws-east-1.svc.singlestore.com",
		},
		{
			name:  "trimmed",
			input: "  svc-abc.aws-east-1.svc.singlestore.com:3306  ",
			want:  "https://svc-abc.aws-east-1.svc.singlestore.com",
		},
		{
			name:    "empty",
			input:   "",
			wantErr: "must not be empty",
		},
		{
			name:    "whitespace",
			input:   "   ",
			wantErr: "must not be empty",
		},
		{
			name:    "scheme prefixed",
			input:   "https://svc-abc.aws-east-1.svc.singlestore.com",
			wantErr: "not a URL with a scheme",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sql.DataAPIURL(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.wantErr)
				require.Empty(t, got)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
