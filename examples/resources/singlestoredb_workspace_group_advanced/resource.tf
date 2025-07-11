provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_workspace_group" "this" {
  name                        = "terraform_test_group_advanced"
  firewall_ranges             = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at                  = "2222-01-01T00:00:00Z"
  region_name                 = "us-west-2"
  cloud_provider              = "AWS"
  admin_password              = "mockPassword193!"
  deployment_type             = "NON-PRODUCTION"
  opt_in_preview_feature      = true
  high_availability_two_zones = true
}
