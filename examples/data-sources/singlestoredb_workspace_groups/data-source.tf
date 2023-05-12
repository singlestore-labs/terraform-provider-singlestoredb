provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_workspace_groups" "all" {}

output "all_workspace_groups" {
  description = "All accessible workspace groups for the user."
  value       = data.singlestoredb_workspace_groups.all
}