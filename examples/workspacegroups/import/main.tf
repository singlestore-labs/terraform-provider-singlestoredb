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
  name           = "override-import-name"
  region_id      = "64031b39-3da1-4a7b-8d3d-6ca86e8d71a7" # Change the region of the region where the group was created.
  expires_at     = "2222-01-01T00:00:00Z"
  admin_password = "fooBAR12$" # This will override the admin password. Not that the provider never fetches password from remote.
}