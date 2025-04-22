provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_invitation" "this" {
  id = "a04f9645-729e-4f92-98b4-206644a12344"
}

output "this_invitation" {
  value = data.singlestoredb_invitation.this
}