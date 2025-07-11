---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "singlestoredb_user Resource - terraform-provider-singlestoredb"
subcategory: ""
description: |-
  The 'apply' action sends a user an invitation to join the organization. The 'destroy' action removes a user from the organization and revokes their pending invitation(s). The 'update' action is not supported for this resource. This resource is currently in beta and may undergo changes in future releases.
---

# singlestoredb_user (Resource)

The 'apply' action sends a user an invitation to join the organization. The 'destroy' action removes a user from the organization and revokes their pending invitation(s). The 'update' action is not supported for this resource. This resource is currently in beta and may undergo changes in future releases.

## Example Usage

```terraform
provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_user" "this" {
  email = "test@user.com"
}

output "singlestoredb_user_this" {
  value = singlestoredb_user.this
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `email` (String) The email address of the user.

### Optional

- `teams` (List of String) A list of user teams associated with the invitation.

### Read-Only

- `created_at` (String) The timestamp when the invitation was created, in ISO 8601 format.
- `id` (String) The unique identifier of the invitation.
- `state` (String) The state of the invitation. Possible values are Pending, Accepted, Refused, or Revoked.
- `user_id` (String) The unique identifier of the user. It is set when the user accepts the invitation.


