provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication. 
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_users" "u" {
}

resource "singlestoredb_team" "t1" {
  name        = "terrafrom-role-test-team-1"
  description = "Terrafrom test role team 1"
}

data "singlestoredb_roles" "rlist" {
  resource_type = "Team"
  resource_id   = singlestoredb_team.t1.id
}

resource "singlestoredb_user_role" "this" {
  user_id = data.singlestoredb_users.u.users[0].id
  role = {
    role_name     = data.singlestoredb_roles.rlist.roles.0
    resource_type = "Team"
    resource_id   = singlestoredb_team.t1.id
  }
}
