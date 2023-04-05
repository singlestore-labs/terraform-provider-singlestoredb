terraform {
  required_providers {
    singlestoredb = {
      source = "registry.terraform.io/singlestore-labs/singlestoredb"
    }
  }
}

provider "singlestoredb" {
  # export SINGLESTOREDB_API_KEY with a SingleStore Management API key
  #test_replace_with_api_key
  #test_replace_with_api_service_url
}

data "singlestoredb_workspace_groups" "all" {}

output "all_workspace_groups" {
  value = data.singlestoredb_workspace_groups.all
}