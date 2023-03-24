package workspaces_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestore/internal/provider/workspaces"
	"github.com/stretchr/testify/require"
)

func TestValidateTerraformSize(t *testing.T) {
	err := workspaces.ValidateTerraformSize("S-00")
	require.Error(t, err, "S-format sizes are not allowed as Terraform input")

	err = workspaces.ValidateTerraformSize("S-0")
	require.Error(t, err, "S-format sizes are not allowed as Terraform input")

	err = workspaces.ValidateTerraformSize("S-1")
	require.Error(t, err, "S-format sizes are not allowed as Terraform input")

	err = workspaces.ValidateTerraformSize("S-2")
	require.Error(t, err, "S-format sizes are not allowed as Terraform input")

	err = workspaces.ValidateTerraformSize(".")
	require.Error(t, err)

	err = workspaces.ValidateTerraformSize(".0")
	require.Error(t, err)

	err = workspaces.ValidateTerraformSize("0.0")
	require.Error(t, err, "only 0 is allowed for suspended")

	err = workspaces.ValidateTerraformSize("0")
	require.NoError(t, err, "only 0 is allowed for suspended")

	err = workspaces.ValidateTerraformSize("0.250")
	require.Error(t, err, "only 0.25 for S-00")

	err = workspaces.ValidateTerraformSize("0.25")
	require.NoError(t, err)

	err = workspaces.ValidateTerraformSize("0.50")
	require.Error(t, err, "only 0.5 for S-0")

	err = workspaces.ValidateTerraformSize("0.5")
	require.NoError(t, err)

	err = workspaces.ValidateTerraformSize("1.0")
	require.Error(t, err, "only 1 for S-1")

	err = workspaces.ValidateTerraformSize("1.")
	require.Error(t, err, "only 1 for S-1")

	err = workspaces.ValidateTerraformSize("1")
	require.NoError(t, err)

	err = workspaces.ValidateTerraformSize("2.0")
	require.Error(t, err, "only 2 for S-2")

	err = workspaces.ValidateTerraformSize("2.")
	require.Error(t, err, "only 2 for S-2")

	err = workspaces.ValidateTerraformSize("2")
	require.NoError(t, err)
}
