provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_role" "example" {
  name          = "custom-reader"
  resource_type = "Organization"
  description   = "A custom role with read-only permissions"

  permissions = [
    "View Organization",
  ]

  inherits = [
    {
      resource_type = "Organization"
      role          = "Reader"
    }
  ]
}

output "role" {
  value = singlestoredb_role.example
}
