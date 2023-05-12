provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_workspace_group" "this" {
  id = "bc8c0deb-50dd-4a58-a5a5-1c62eb5c456d" # Replace with the actual ID of the workspace group.
}

output "this_workspace_group" {
  value = data.singlestoredb_workspace_group.this
}