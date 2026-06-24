package sql

import (
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/config"
	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/util"
)

// resolvePassword returns the effective SQL password or JWT.
// Precedence: explicit non-empty attribute > SINGLESTORE_SQL_USER_PASSWORD env var.
func resolvePassword(attr types.String) (string, *util.SummaryWithDetailError) {
	if !attr.IsNull() && !attr.IsUnknown() && attr.ValueString() != "" {
		return attr.ValueString(), nil
	}

	if env := os.Getenv(config.EnvSQLUserPassword); env != "" {
		return env, nil
	}

	return "", &util.SummaryWithDetailError{
		Summary: "Missing SQL credentials",
		Detail: fmt.Sprintf(
			"Set the password attribute or the %s environment variable.",
			config.EnvSQLUserPassword,
		),
	}
}

// passwordForState returns the password value to store in Terraform state.
// Env-sourced passwords are not persisted.
func passwordForState(attr types.String) types.String {
	if !attr.IsNull() && !attr.IsUnknown() && attr.ValueString() != "" {
		return attr
	}

	return types.StringNull()
}

// passwordConfiguredInPlan reports whether the user set password explicitly in config.
func passwordConfiguredInPlan(attr types.String) bool {
	return !attr.IsNull() && !attr.IsUnknown() && attr.ValueString() != ""
}
