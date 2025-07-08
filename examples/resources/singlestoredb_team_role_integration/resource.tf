provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_team" "t1" {
  name        = "terrafrom-role-test-team-team-1"
  description = "Terrafrom test role team to team 1"
}


resource "singlestoredb_team" "t2" {
  name        = "terrafrom-role-test-team-to-team-2"
  description = "Terrafrom test role team to team 2"
}

data "singlestoredb_roles" "rlist" {
  resource_type = "Team"
  resource_id   = singlestoredb_team.t2.id
}

resource "singlestoredb_team_role" "this" {
  team_id = singlestoredb_team.t1.id
  role = {
    role_name     = data.singlestoredb_roles.rlist.roles.0
    resource_type = "Team"
    resource_id   = singlestoredb_team.t2.id
  }
}
