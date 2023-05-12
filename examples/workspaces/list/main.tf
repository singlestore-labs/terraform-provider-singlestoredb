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

data "singlestoredb_workspaces" "all" {
  workspace_group_id = "bc8c0deb-50dd-4a58-a5a5-1c62eb5c456d"
}

output "all_workspaces" {
  value = data.singlestoredb_workspaces.all
}