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

data "singlestoredb_regions_v2" "r" {}

resource "singlestoredb_workspace_group" "g" {
  name            = "test-role-group"
  firewall_ranges = ["0.0.0.0/0"]
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = data.singlestoredb_regions_v2.r.regions.0.provider
  region_name     = data.singlestoredb_regions_v2.r.regions.0.region_name
  admin_password  = "mockPassword193!"
}

data "singlestoredb_roles" "t1roles" {
  resource_type = "Team"
  resource_id   = singlestoredb_team.t1.id
}

data "singlestoredb_roles" "clusterroles" {
  resource_type = "Cluster"
  resource_id   = singlestoredb_workspace_group.g.id
}

resource "singlestoredb_user_roles" "this" {
  user_id = data.singlestoredb_users.u.users[0].id
  roles = [
    {
      role_name     = data.singlestoredb_roles.t1roles.roles.0
      resource_type = "Team"
      resource_id   = singlestoredb_team.t1.id
    },
    {
      role_name     = data.singlestoredb_roles.clusterroles.roles.0
      resource_type = "Cluster"
      resource_id   = singlestoredb_workspace_group.g.id
    }
  ]
}
