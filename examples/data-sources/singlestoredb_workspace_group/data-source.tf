provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_workspace_group" "this" {
  id = "7be43ca1-77bd-4075-9a21-f49d9079f8dc" # Replace with the actual ID of the workspace group.
}

output "this_workspace_group" {
  value = data.singlestoredb_workspace_group.this
}