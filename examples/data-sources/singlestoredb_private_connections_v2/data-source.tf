provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_private_connections_v2" "all" {
  cluster_id = "319a4b58-0f84-4933-bc19-3b5d11297a1a" # Replace with the cluster ID.
}

output "all_private_connections" {
  value = data.singlestoredb_private_connections_v2.all
}
