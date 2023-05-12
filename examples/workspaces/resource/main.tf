terraform {
  required_providers {
    singlestoredb = {
      source = "registry.terraform.io/singlestore-labs/singlestoredb"
    }
  }
}

provider "singlestoredb" {
  # export SINGLESTOREDB_API_KEY with a SingleStore Management API key
}

data "singlestoredb_regions" "all" {}

resource "singlestoredb_workspace_group" "example" {
  name            = "terraform-provider-ci-integration-test-workspace-group"
  firewall_ranges = ["0.0.0.0/0"] # Allows all the traffic. Make sure to set limiting CIDR ranges for production environments or an empty list for no traffic.
  expires_at      = "2222-01-01T00:00:00Z"
  region_id       = data.singlestoredb_regions.all.regions.0.id # In production, prefer indicating the explicit region ID because the list of regions changes.
  admin_password  = "fooBAR12$"                                 # Exlicitly setting password is not mandatory. If it is not indicated, server generates one.
}

resource "singlestoredb_workspace" "example" {
  name               = "test-workspace"
  workspace_group_id = singlestoredb_workspace_group.example.id
  size               = "S-00"
  suspended          = false
}

output "example_endpoint" {
  value = singlestoredb_workspace.example.endpoint
}