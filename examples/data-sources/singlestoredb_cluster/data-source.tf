provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_cluster" "this" {
  id = "f2a1a960-8591-4156-bb26-f53f0f8e35ce" # Replace with the actual ID of the cluster.
}

output "this_cluster" {
  value = data.singlestoredb_cluster.this
}
