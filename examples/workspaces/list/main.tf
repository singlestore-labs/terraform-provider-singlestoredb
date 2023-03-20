terraform {
  required_providers {
    singlestore = {
      source = "registry.terraform.io/singlestoredb/singlestore"
    }
  }
}

provider "singlestore" {
  # export SINGLESTORE_API_KEY with a SingleStore Management API key
  #test_replace_with_api_key
  #test_replace_with_api_service_url
}

data "singlestore_workspaces" "all" {
  workspace_group_id = "bc8c0deb-50dd-4a58-a5a5-1c62eb5c456d"
}

output "all_workspaces" {
  value = data.singlestore_workspaces.all
}