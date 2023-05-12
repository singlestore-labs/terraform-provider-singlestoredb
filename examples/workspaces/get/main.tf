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

data "singlestoredb_workspace" "example" {
  id = "26171125-ecb8-5944-9896-209fbffc1f15" # Replace with the ID of a workspace group that exists.
}

output "example_workspace" {
  value = data.singlestoredb_workspace.example
}