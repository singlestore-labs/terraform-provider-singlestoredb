provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_team" "t1" {
  name        = "terrafrom-role-test-team-to-team-1"
  description = "Terrafrom test role team to team 1"
}

resource "singlestoredb_team" "t2" {
  name        = "terrafrom-role-test-team-to-team-2"
  description = "Terrafrom test role team to team 2"
}

data "singlestoredb_regions_v2" "r" {}

resource "singlestoredb_workspace_group" "g" {
  name            = "test-role-to-team-group"
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

resource "singlestoredb_team_roles" "this" {
  team_id = singlestoredb_team.t1.id
  roles = [
    {
      role_name     = data.singlestoredb_roles.t1roles.roles.0
      resource_type = "Team"
      resource_id   = singlestoredb_team.t2.id
    },
    {
      role_name     = data.singlestoredb_roles.clusterroles.roles.0
      resource_type = "Cluster"
      resource_id   = singlestoredb_workspace_group.g.id
    }
  ]
}
