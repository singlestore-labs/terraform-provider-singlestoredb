---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "singlestoredb_user_role Resource - terraform-provider-singlestoredb"
subcategory: ""
description: |-
  Manages a single role grant for a user (the 'subject' in RBAC terminology). This resource allows you to assign a specific role to a user, defining what access permission the user has to a particular resource (object) in the system. In Role-Based Access Control, this resource establishes the relationship between the subject (user), the permission level (role), and the target resource that can be accessed. Use the singlestoredb_roles data source with a specific resource's type and ID to discover what roles are available for that resource object. This resource is currently in beta and may undergo changes in future releases.
---

# singlestoredb_user_role (Resource)

Manages a single role grant for a user (the 'subject' in RBAC terminology). This resource allows you to assign a specific role to a user, defining what access permission the user has to a particular resource (object) in the system. In Role-Based Access Control, this resource establishes the relationship between the subject (user), the permission level (role), and the target resource that can be accessed. Use the `singlestoredb_roles` data source with a specific resource's type and ID to discover what roles are available for that resource object. This resource is currently in beta and may undergo changes in future releases.

## Example Usage

```terraform
provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_user_role" "this" {
  user_id = "17290909-3016-4f63-b601-e30410f1b05f"
  role = {
    role_name     = "Owner"
    resource_type = "Team"
    resource_id   = "c2757c25-26d2-434a-91ee-f47683e6cdb3"
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `role` (Attributes) The role to be assigned to the user. (see [below for nested schema](#nestedatt--role))
- `user_id` (String) The unique identifier of the user.

### Read-Only

- `id` (String) The unique identifier of the granted role.

<a id="nestedatt--role"></a>
### Nested Schema for `role`

Required:

- `resource_id` (String) The identifier of the resource.
- `resource_type` (String) The type of the resource.
- `role_name` (String) The name of the role.


