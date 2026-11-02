provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

# Deprecated. Use singlestoredb_regions_v2 instead. Removed in v0.3.0, the first release on or after April 30, 2027.
data "singlestoredb_regions" "all" {}

output "all_regions" {
  description = "All available regions for the user that support workspaces."
  value       = data.singlestoredb_regions.all
}