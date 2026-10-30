package roles

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/stretchr/testify/require"
)

// requiresReplaceDescription is stringplanmodifier.RequiresReplace()'s fixed description,
// used here to assert the modifier is attached without depending on its unexported type.
const requiresReplaceDescription = "If the value of this attribute changes, Terraform will destroy and recreate the resource."

// TestUserTeamIdentityAttributesRequireReplace guards against regressing to a plan that
// lets user_id/team_id change in place: an Update that leaves the old identity still
// granted and the new identity never granted, or - when the role's own spelling also
// changes - a response state that still carries the old identity and gets rejected by
// Terraform as an inconsistent apply. Requiring replacement instead forces a Delete
// (revoke on the old identity) followed by a Create (grant on the new identity).
func TestUserTeamIdentityAttributesRequireReplace(t *testing.T) {
	requireAttributeRequiresReplace(t, NewUserRoleGrantResource(), "user_id")
	requireAttributeRequiresReplace(t, NewTeamRoleGrantResource(), "team_id")
	requireAttributeRequiresReplace(t, NewUserRolesGrantResource(), "user_id")
	requireAttributeRequiresReplace(t, NewTeamRolesGrantResource(), "team_id")
}

func requireAttributeRequiresReplace(t *testing.T, r resource.Resource, attrName string) {
	t.Helper()

	var resp resource.SchemaResponse
	r.Schema(t.Context(), resource.SchemaRequest{}, &resp)
	require.Empty(t, resp.Diagnostics)

	attr, ok := resp.Schema.Attributes[attrName].(schema.StringAttribute)
	require.True(t, ok, "%s should be a string attribute", attrName)

	var found bool
	for _, modifier := range attr.PlanModifiers {
		if modifier.Description(context.Background()) == requiresReplaceDescription {
			found = true

			break
		}
	}
	require.True(t, found, "%s should have a RequiresReplace plan modifier", attrName)
}
