provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_workspace_group" "example" {
  name            = "group"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
  admin_password  = "mockPassword193!"
}

resource "singlestoredb_workspace" "this" {
  name               = "workspace-1"
  workspace_group_id = singlestoredb_workspace_group.example.id
  size               = "S-00"
  suspended          = false
}

output "endpoint" {
  value = singlestoredb_workspace.this.endpoint
}

output "admin_password" {
  value     = singlestoredb_workspace_group.example.admin_password
  sensitive = true
}

// Note: The 'mysql' client must be installed locally for the following provisioner to work.
resource "null_resource" "create_table" {
  triggers = {
    schema_hash = filesha256("${path.module}/schema.sql")
  }

  provisioner "local-exec" {
    command = <<EOT
      mysql -u admin \
        -h ${singlestoredb_workspace.this.endpoint} \
        -P 3306 --default-auth=mysql_native_password \
        --password="${singlestoredb_workspace_group.example.admin_password}" \
        < ${path.module}/schema.sql
    EOT
  }
}
