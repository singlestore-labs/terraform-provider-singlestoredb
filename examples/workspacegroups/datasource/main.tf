terraform {
  required_providers {
    singlestore = {
      source = "registry.terraform.io/singlestoredb/singlestore"
    }
  }
}

provider "singlestore" {
  # export SINGLESTORE_API_KEY with a SingleStore Management API key
  #unit_test_replace_with_api_key
  #unit_test_replace_with_api_service_url
}

data "singlestore_workspace_groups" "all" {}

output "all_workspace_groups" {
  value = data.singlestore_workspace_groups.all
}