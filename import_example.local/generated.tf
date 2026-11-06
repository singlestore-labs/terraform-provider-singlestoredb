# __generated__ by Terraform
# Please review these resources and move them into your main configuration files.

# __generated__ by Terraform from "6dd08491-626e-4015-85ec-f2cb383317d5"
resource "singlestoredb_cluster" "this" {
  auto_scale = {
    max_scale_factor = 1
    sensitivity      = "NORMAL"
  }
  auto_suspend = {
    suspend_after_seconds = 7200
    suspend_type          = "IDLE"
  }
  cache_config           = 1
  cloud_provider         = "AWS"
  deployment_type        = "PRODUCTION"
  expires_at             = null
  firewall_ranges        = ["0.0.0.0/0"]
  kai                    = false
  multi_az               = false
  name                   = "feast-demo"
  opt_in_preview_feature = false
  project_name           = "Standard Project"
  region_name            = "eu-central-1"
  scale_factor           = 1
  size                   = "S-00"
  suspended              = true
}
