provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_workspace_group" "group" {
  name            = "group"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-west-2"
}

resource "singlestoredb_workspace" "workspace" {
  name               = "workspace-1"
  workspace_group_id = singlestoredb_workspace_group.group.id
  size               = "S-00"
}

resource "singlestoredb_private_connection" "this" {
  allow_list         = "651246146166"
  type               = "INBOUND"
  workspace_group_id = singlestoredb_workspace_group.group.id
  workspace_id       = singlestoredb_workspace.workspace.id
}

output "endpoint" {
  value = singlestoredb_workspace.workspace.endpoint
}

output "admin_password" {
  value     = singlestoredb_workspace_group.group.admin_password
  sensitive = true
}

output "private_connection_id" {
  value = singlestoredb_private_connection.this.id
}
