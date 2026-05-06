provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_workspace_private_connections" "this" {
  workspace_id = "00000000-0000-0000-0000-000000000000"
}

output "workspace_private_connections" {
  value = data.singlestoredb_workspace_private_connections.this
}
