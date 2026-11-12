provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

# Deprecated. Use singlestoredb_private_connections_v2 instead. Removed in v1.0.0, the first release on or after April 30, 2027.
data "singlestoredb_private_connections" "all" {
  workspace_group_id = "319a4b58-0f84-4933-bc19-3b5d11297a1a"
}

output "all_private_connections" {
  value = data.singlestoredb_private_connections.all
}
