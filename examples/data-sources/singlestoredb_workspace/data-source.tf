provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_workspace" "this" {
  id = "e3e461ad-61b7-45fd-a108-7e342e9fa0aa" # Replace with the actual ID of the workspace.
}

output "this_workspace" {
  value = data.singlestoredb_workspace.this
}