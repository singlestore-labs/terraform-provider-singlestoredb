provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_regions_v2" "all" {}

output "all_regions" {
  description = "All available regions for the user that support workspaces."
  value       = data.singlestoredb_regions_v2.all
}