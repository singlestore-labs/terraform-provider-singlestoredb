provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_private_connection" "this" {
  id = "26171125-ecb8-5944-9896-209fbffc1f15" # Replace with the actual ID of the private connection.
}

output "this_private_connection" {
  value = data.singlestoredb_private_connection.this
}