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

data "singlestore_workspace" "example" {
  id = "26171125-ecb8-5944-9896-209fbffc1f15" # Replace with the ID of a workspace group that exists.
}

output "example_workspace" {
  value = data.singlestore_workspace.example
}