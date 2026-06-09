---
page_title: "singlestoredb_sql Resource - singlestoredb"
subcategory: ""
description: |-
  Execute SQL statements against a SingleStore workspace.
---

# singlestoredb_sql (Resource)

Execute SQL statements against a SingleStore workspace. This resource is useful for bootstrapping schemas, users, and grants after workspace provisioning.

## Example Usage

```terraform
resource "singlestoredb_sql" "schema" {
  endpoint = singlestoredb_workspace.this.endpoint
  password = singlestoredb_workspace_group.example.admin_password
  execute  = file("${path.module}/schema.sql")
  revert   = "DROP DATABASE IF EXISTS my_app_db"
  query    = "SHOW DATABASES LIKE 'my_app_db'"
}
```

## Schema

### Required

- `endpoint` (String) The workspace SQL endpoint hostname.
- `execute` (String) SQL statement to execute when the resource is created. Changing this value forces recreation of the resource.
- `password` (String, Sensitive) The SQL user password used to connect to the workspace.

### Optional

- `database` (String) The default database to connect to.
- `port` (Number) The SQL port used to connect to the workspace. Defaults to `3306`.
- `query` (String) Optional SQL statement to run on every read. Use this to verify the state of objects created by `execute`.
- `revert` (String) SQL statement to execute when the resource is destroyed.
- `tls` (String) TLS mode for the SQL connection. Valid values are `true`, `false`, `skip-verify`, and `preferred`. Defaults to `preferred`.
- `username` (String) The SQL username used to connect to the workspace. Defaults to `admin`.

### Read-Only

- `id` (String) The unique identifier of the SQL execution resource.
- `query_results` (List of Map of String) List of key-value maps retrieved after executing the optional `query` statement.

## Import

Import is supported using the following syntax:

```shell
terraform import singlestoredb_sql.example 00000000-0000-0000-0000-000000000000
```

The import ID must be a UUID. After import, Terraform will run the optional `query` statement on the next refresh.
