package testutil_test

import (
	"strings"
	"testing"

	"github.com/singlestore-labs/terraform-provider-singlestoredb/internal/provider/testutil"
	"github.com/stretchr/testify/require"
)

func TestWithUniqueWorkspaceGroupNames(t *testing.T) {
	config := `
resource "singlestoredb_workspace_group" "group" {
  name            = "group"
  firewall_ranges = ["0.0.0.0/0"]
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-west-2"
}

resource "singlestoredb_workspace_group" "another" {
  name            = "another-group"
  firewall_ranges = ["0.0.0.0/0"]
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
}

resource "singlestoredb_workspace" "workspace" {
  name               = "workspace-1"
  workspace_group_id = singlestoredb_workspace_group.group.id
  size               = "S-00"
}
`

	uniqueName := "unique-test-name-12345"
	result := testutil.UpdatableConfig(config).WithUniqueWorkspaceGroupNames(uniqueName).String()

	// Both workspace groups should have the unique name
	require.Contains(t, result, `"`+uniqueName+`"`)

	// Count how many times the unique name appears (should be 2 - one for each workspace group)
	count := strings.Count(result, `"`+uniqueName+`"`)
	require.Equal(t, 2, count, "Should have exactly 2 workspace groups with the unique name")

	// Should still contain the workspace name (not modified)
	require.Contains(t, result, `"workspace-1"`)

	// Verify the resource labels are preserved
	require.Contains(t, result, `resource "singlestoredb_workspace_group" "group"`)
	require.Contains(t, result, `resource "singlestoredb_workspace_group" "another"`)
}
