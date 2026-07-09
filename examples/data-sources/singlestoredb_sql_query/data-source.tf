provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_workspace_group" "example" {
  name            = "group"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
  admin_password  = "mockPassword193!"
}

resource "singlestoredb_workspace" "this" {
  name               = "workspace-1"
  workspace_group_id = singlestoredb_workspace_group.example.id
  size               = "S-00"
  suspended          = false
}

data "singlestoredb_sql_query" "this" {
  depends_on = [singlestoredb_workspace.this]

  endpoint = singlestoredb_workspace.this.endpoint
  username = "admin"
  password = singlestoredb_workspace_group.example.admin_password
  query    = "SELECT ? AS value"
  args     = ["1"]
}

output "endpoint" {
  value = singlestoredb_workspace.this.endpoint
}

output "query_row_count" {
  value = length(data.singlestoredb_sql_query.this.rows)
}
