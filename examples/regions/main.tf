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

data "singlestoredb_regions" "all" {}

output "all_regions" {
  value = data.singlestoredb_regions.all
}