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

resource "singlestoredb_sql_execute" "this" {
  depends_on = [singlestoredb_workspace.this]

  endpoint = singlestoredb_workspace.this.endpoint
  username = "admin"
  password = singlestoredb_workspace_group.example.admin_password

  execute      = "CREATE DATABASE IF NOT EXISTS ?"
  execute_args = ["my_app_db"]
  revert       = "DROP DATABASE IF EXISTS my_app_db"

  query      = "SHOW DATABASES LIKE ?"
  query_args = ["my_app_db"]
}

output "endpoint" {
  value = singlestoredb_workspace.this.endpoint
}

output "database_exists" {
  value = length(singlestoredb_sql_execute.this.query_results) > 0
}
