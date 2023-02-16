terraform {
  required_providers {
    singlestore = {
      source = "registry.terraform.io/singlestoredb/singlestore"
    }
  }
}

provider "singlestore" {}

data "singlestore_regions" "all" {}

output "all_regions" {
  value = data.singlestore_regions.all
}