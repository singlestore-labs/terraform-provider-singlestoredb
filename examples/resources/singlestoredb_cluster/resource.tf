provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_cluster" "this" {
  name            = "cluster-1"
  project_name    = "Standard Project"
  size            = "S-00"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
  suspended       = false
}

output "endpoint" {
  value = singlestoredb_cluster.this.endpoint
}

output "admin_password" {
  value     = singlestoredb_cluster.this.admin_password
  sensitive = true
}

output "group_id" {
  value = singlestoredb_cluster.this.group_id
}
