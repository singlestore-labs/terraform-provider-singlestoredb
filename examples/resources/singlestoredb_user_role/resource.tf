provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_user_role" "this" {
  user_id = "17290909-3016-4f63-b601-e30410f1b05f"
  role = {
    role_name     = "Owner"
    resource_type = "Team"
    resource_id   = "c2757c25-26d2-434a-91ee-f47683e6cdb3"
  }
}
