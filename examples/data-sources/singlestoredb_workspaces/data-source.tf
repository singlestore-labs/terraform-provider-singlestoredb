provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_workspaces" "all" {
  workspace_group_id = "f5356175-1ae7-4ef1-8356-43e3cfd9d12a" # Replace with the actual ID of the workspace group.
}

output "all_workspaces" {
  value = data.singlestoredb_workspaces.all
}