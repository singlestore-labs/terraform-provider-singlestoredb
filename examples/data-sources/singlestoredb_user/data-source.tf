provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_user" "this" {
  id = "9e481f02-7e4c-478a-98b9-df2c712bed4c" # Replace with the actual ID of the user.
}

output "this_user" {
  value = data.singlestoredb_user.this
}
