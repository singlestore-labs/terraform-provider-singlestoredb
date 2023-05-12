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
  firewall_ranges = ["192.168.0.1/32"] # Edit to the desired ranges to connect successfully.
  expires_at      = "2222-01-01T00:00:00Z"
  region_id       = data.singlestoredb_regions.all.regions.0.id # In production, prefer indicating the explicit region ID because the list of regions changes.
  admin_password  = "fooBAR12$"                                 # Exlicitly setting password is not mandatory. If it is not indicated, server generates one.
}