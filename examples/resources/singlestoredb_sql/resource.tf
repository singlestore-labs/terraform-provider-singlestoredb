resource "singlestoredb_sql" "this" {
  endpoint = "workspace.example.com"
  password = "mockPassword193!"
  execute  = "CREATE DATABASE IF NOT EXISTS my_app_db"
  revert   = "DROP DATABASE IF EXISTS my_app_db"
  query    = "SHOW DATABASES LIKE 'my_app_db'"
}
