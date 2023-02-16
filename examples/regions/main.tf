terraform {
  required_providers {
    singlestore = {
      source = "registry.terraform.io/singlestoredb/singlestore"
    }
  }
}

provider "singlestore" {
  # export SINGLESTORE_API_KEY with a SingleStore Management API key
}

data "singlestore_regions" "all" {}

output "all_regions" {
  value = data.singlestore_regions.all
}