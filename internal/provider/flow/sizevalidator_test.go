package flow_test

import (
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/flow"
	"github.com/stretchr/testify/require"
)

func TestValidateTerraformSize(t *testing.T) {
	err := flow.ValidateTerraformSize("F1")
	require.NoError(t, err, "Only F-format sizes are allowed as Terraform input")

	err = flow.ValidateTerraformSize("F2")
	require.NoError(t, err, "Only F-format sizes are allowed as Terraform input")

	err = flow.ValidateTerraformSize("F3")
	require.NoError(t, err, "Only F-format sizes are allowed as Terraform input")

	err = flow.ValidateTerraformSize("F4")
	require.NoError(t, err, "Only F-format sizes are allowed as Terraform input")

	err = flow.ValidateTerraformSize("Fnan")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("S-00")
	require.Error(t, err)

	err = flow.ValidateTerraformSize(".")
	require.Error(t, err)

	err = flow.ValidateTerraformSize(".0")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("0.0")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("0")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("0.250")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("0.25")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("0.5")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("1.0")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("1.")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("1")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("2.0")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("2.")
	require.Error(t, err)

	err = flow.ValidateTerraformSize("2")
	require.Error(t, err)
}
