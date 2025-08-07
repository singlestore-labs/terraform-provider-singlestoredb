import {
  to = singlestoredb_workspace_group.this
  id = "3c0c0d99-3c09-45ac-a01f-5ab62afd35cf"
}

output "imported_workspace_group" {
  value = singlestoredb_workspace_group.this.name
}