provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_role" "test" {
  name          = "terraform-test-custom-role"
  resource_type = "Organization"
  description   = "A test custom role created by Terraform integration tests"

  permissions = []

  inherits = [
    {
      resource_type = "Organization"
      role          = "Reader"
    }
  ]
}

output "role_id" {
  value = singlestoredb_role.test.id
}
