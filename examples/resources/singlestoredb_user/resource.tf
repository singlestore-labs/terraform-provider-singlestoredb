provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_user" "this" {
  email = "test@user.com"
}

output "singlestoredb_user_this" {
  value = singlestoredb_user.this
}