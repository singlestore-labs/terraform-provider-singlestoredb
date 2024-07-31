provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_regions" "all" {}

resource "singlestoredb_workspace_group" "example" {
  name            = "group"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  region_id       = data.singlestoredb_regions.all.regions.0.id // Prefer specifying the explicit region ID in production environments as the list of regions may vary.
}

resource "singlestoredb_workspace" "this" {
  name               = "workspace"
  workspace_group_id = singlestoredb_workspace_group.example.id
  size               = "S-00"
  suspended          = false
  kai_api_enabled = true
}

output "endpoint" {
  value = singlestoredb_workspace.this.endpoint
}

output "admin_password" {
  value     = singlestoredb_workspace_group.example.admin_password
  sensitive = true
}
