provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_invitation" "this" {
  id = "c87337b8-fe50-41e9-92e0-2387d5476f90"
}

output "this_invitation" {
  value = data.singlestoredb_invitation.this
}