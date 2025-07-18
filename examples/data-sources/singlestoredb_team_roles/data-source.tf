provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_team_roles" "all" {
  team_id = "d123ec69-936c-4e71-92bb-a45d987f9118"
}

output "all_team_roles" {
  value = data.singlestoredb_team_roles.all
}
