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

data "singlestoredb_regions" "all" {}

output "all_regions" {
  value = data.singlestoredb_regions.all
}