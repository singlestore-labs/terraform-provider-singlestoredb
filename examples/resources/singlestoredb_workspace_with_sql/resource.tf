provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

variable "app_user_password" {
  type        = string
  default     = "password123"
  sensitive   = true
  description = "Password for the application SQL user."
}

variable "app_readonly_password" {
  type        = string
  default     = "readonly123"
  sensitive   = true
  description = "Password for the read-only SQL user."
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

locals {
  sql_endpoint = singlestoredb_workspace.this.endpoint
  sql_username = "admin"
  sql_password = singlestoredb_workspace_group.example.admin_password
  app_db       = "my_app_db"
}

resource "singlestoredb_sql_execute" "create_db" {
  depends_on = [singlestoredb_workspace.this]

  endpoint = local.sql_endpoint
  username = local.sql_username
  password = local.sql_password

  execute = "CREATE DATABASE IF NOT EXISTS my_app_db"
  revert  = "DROP DATABASE IF EXISTS my_app_db"

  query      = "SHOW DATABASES LIKE ?"
  query_args = [local.app_db]
}

resource "singlestoredb_sql_execute" "create_app_user" {
  depends_on = [singlestoredb_sql_execute.create_db]

  endpoint = local.sql_endpoint
  username = local.sql_username
  password = local.sql_password
  database = local.app_db

  execute      = "CREATE USER IF NOT EXISTS 'app_user'@'%' IDENTIFIED BY ?"
  execute_args = [var.app_user_password]
  revert       = "DROP USER IF EXISTS 'app_user'@'%'"
}

resource "singlestoredb_sql_execute" "create_readonly_user" {
  depends_on = [singlestoredb_sql_execute.create_db]

  endpoint = local.sql_endpoint
  username = local.sql_username
  password = local.sql_password
  database = local.app_db

  execute      = "CREATE USER IF NOT EXISTS 'app_readonly'@'%' IDENTIFIED BY ?"
  execute_args = [var.app_readonly_password]
  revert       = "DROP USER IF EXISTS 'app_readonly'@'%'"
}

resource "singlestoredb_sql_execute" "grant_app_user" {
  depends_on = [singlestoredb_sql_execute.create_app_user]

  endpoint = local.sql_endpoint
  username = local.sql_username
  password = local.sql_password
  database = local.app_db

  execute = <<-EOT
    GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, PROCESS, INDEX, ALTER, DROP, SHOW METADATA, CREATE DATABASE, DROP DATABASE, CREATE USER ON my_app_db.* TO 'app_user'@'%'
  EOT
  revert  = <<-EOT
    REVOKE SELECT, INSERT, UPDATE, DELETE, CREATE, PROCESS, INDEX, ALTER, DROP, SHOW METADATA, CREATE DATABASE, DROP DATABASE, CREATE USER ON my_app_db.* FROM 'app_user'@'%'
  EOT
}

resource "singlestoredb_sql_execute" "grant_readonly" {
  depends_on = [singlestoredb_sql_execute.create_readonly_user]

  endpoint = local.sql_endpoint
  username = local.sql_username
  password = local.sql_password
  database = local.app_db

  execute = "GRANT SELECT ON my_app_db.* TO 'app_readonly'@'%'"
  revert  = "REVOKE SELECT ON my_app_db.* FROM 'app_readonly'@'%'"
}

resource "singlestoredb_sql_execute" "create_users_table" {
  depends_on = [
    singlestoredb_sql_execute.grant_app_user,
    singlestoredb_sql_execute.grant_readonly,
  ]

  endpoint = local.sql_endpoint
  username = local.sql_username
  password = local.sql_password
  database = local.app_db

  execute = <<-EOT
    CREATE TABLE IF NOT EXISTS users (
      id INT AUTO_INCREMENT PRIMARY KEY,
      email VARCHAR(100) NOT NULL,
      password VARCHAR(100) NOT NULL
    )
  EOT
  revert  = "DROP TABLE IF EXISTS users"
}

resource "singlestoredb_sql_execute" "create_posts_table" {
  depends_on = [singlestoredb_sql_execute.create_users_table]

  endpoint = local.sql_endpoint
  username = local.sql_username
  password = local.sql_password
  database = local.app_db

  execute = <<-EOT
    CREATE TABLE IF NOT EXISTS posts (
      id INT AUTO_INCREMENT PRIMARY KEY,
      user_id INT NOT NULL,
      title VARCHAR(200),
      body TEXT
    )
  EOT
  revert  = "DROP TABLE IF EXISTS posts"
}

output "endpoint" {
  value = singlestoredb_workspace.this.endpoint
}

output "database_exists" {
  value = length(singlestoredb_sql_execute.create_db.query_results) > 0
}
