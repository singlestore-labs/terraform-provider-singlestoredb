provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_roles" "all" {
  resource_type = "Organization"
  resource_id   = "8769efa3-7578-49e1-9c07-bcd763488301"
}

output "all_roles" {
  value = data.singlestoredb_roles.all
}
