package clusters_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/clusters"
	"github.com/stretchr/testify/require"
)

func TestValidateTerraformSize(t *testing.T) {
	require.NoError(t, clusters.ValidateTerraformSize("S-1"))
	require.Error(t, clusters.ValidateTerraformSize("X-00"))
}
