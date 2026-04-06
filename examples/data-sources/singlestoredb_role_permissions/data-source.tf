provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

# Look up all available permissions for the Organization resource type.
data "singlestoredb_role_permissions" "org" {
  resource_type = "Organization"
}

# Use the permissions data source to select specific permissions when creating a custom role.
resource "singlestoredb_role" "custom" {
  name          = "custom-org-manager"
  resource_type = "Organization"
  description   = "A custom role with selected Organization permissions"

  # Pick from data.singlestoredb_role_permissions.org.permissions to see what is available.
  permissions = [
    data.singlestoredb_role_permissions.org.permissions[0],
  ]
}

output "available_org_permissions" {
  value = data.singlestoredb_role_permissions.org.permissions
}
