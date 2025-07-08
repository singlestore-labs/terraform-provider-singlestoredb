provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_regions_v2" "all" {}

resource "singlestoredb_workspace_group" "this" {
  name            = "group"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = data.singlestoredb_regions_v2.all.regions.0.provider
  region_name     = data.singlestoredb_regions_v2.all.regions.0.region_name
  admin_password  = "mockPassword193!"
}
