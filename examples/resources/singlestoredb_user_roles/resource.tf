provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_user_roles" "this" {
  user_id = "17290909-3016-4f63-b601-e30410f1b05f"
  roles = [
    {
      role_name     = "Owner"
      resource_type = "Team"
      resource_id   = "d4f34ce0-e79f-46e4-994f-3da004d98bff"
    },
    {
      role_name     = "Operator"
      resource_type = "Organization"
      resource_id   = "c7e83804-2e49-4dcf-bbd4-27fd7ad28d5d"
    }
  ]
}
