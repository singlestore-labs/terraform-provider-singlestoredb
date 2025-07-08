provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_team" "this" {
  id = "24f31e2d-847f-4a62-9a93-a10e9bcd0dae" # Replace with the actual ID of the team.
}

output "this_team" {
  value = data.singlestoredb_team.this
}
