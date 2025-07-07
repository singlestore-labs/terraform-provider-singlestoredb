provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_team_roles" "this" {
  team_id = "93e40aef-df01-467b-944c-3b091afd304e"
  roles = [
    {
      role_name     = "Owner"
      resource_type = "Team"
      resource_id   = "f1e01e30-440c-4633-a165-77589419eb42"
    },
    {
      role_name     = "Operator"
      resource_type = "Organization"
      resource_id   = "24f31e2d-847f-4a62-9a93-a10e9bcd0dae"
    }
  ]
}
