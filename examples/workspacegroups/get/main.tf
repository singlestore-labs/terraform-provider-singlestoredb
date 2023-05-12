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

data "singlestoredb_workspace_group" "example" {
  id = "bc8c0deb-50dd-4a58-a5a5-1c62eb5c456d" # Replace with the ID of a workspace group that exists.
}

output "example_workspace_group" {
  value = data.singlestoredb_workspace_group.example
}