---
page_title: "Migrating from Workspace to Cluster - terraform-provider-singlestoredb"
description: |-
  Keep singlestoredb_workspace and singlestoredb_workspace_group working, or move them to singlestoredb_cluster without recreating the deployment.
---

# Migrating from workspace to cluster

`singlestoredb_workspace_group` and `singlestoredb_workspace` stay in the provider. Their arguments are unchanged. Under Management API `/v2/clusters`, a workspace and its workspace group are one cluster, so a few operations cannot work the way they did against `/v1/workspaceGroups` and `/v1/workspaces`.

Use this guide in two ways:

- Keep the existing resources and check the behavior section before you upgrade the provider.
- Replace them with `singlestoredb_cluster` when you want one resource for that deployment. The state migration does not recreate the cluster.

## Behavior that stays the same

These `singlestoredb_workspace_group` and `singlestoredb_workspace` uses keep working:

- Resource addresses and argument names are unchanged. You do not need a `moved` block to keep using them.
- One workspace group plus one workspace still ends as one deployment. The group creates a starter cluster (size `S-00`). The workspace adopts that cluster, so the admin password, firewall, and `workspace_group_id` stay on that object.
- Size, scale factor, cache, autoscale, autosuspend, suspend, and resume still update the workspace.
- Firewall ranges, admin password, expiration, and deployment type still update the workspace group.
- Import still uses the same IDs: the workspace group ID for `singlestoredb_workspace_group`, and the workspace ID for `singlestoredb_workspace`. That workspace ID is the cluster ID.
- A workspace group that already stores `region_id` keeps that value. Refresh does not clear it, and plans that still set only `region_id` are accepted. `cloud_provider` and `region_name` remain the way to choose a region for a new group.
- An existing workspace group or workspace name longer than 32 characters can still be planned and updated. The 1–32 character limit applies when creating a new name.

## What /v2/clusters cannot keep the same

- Creating a workspace group also creates its starter cluster. There is no empty group.
- `project_name` is required on workspace group create. The API requires a project ID.
- `name` and `update_window` cannot be changed after the workspace group is created.
- New workspace groups cannot set `region_id`. Set `cloud_provider` and `region_name`. The `singlestoredb_workspace_group` data source leaves `region_id` empty and returns `cloud_provider` and `region_name`.
- New names must be 1–32 characters.
- A workspace group supports one `singlestoredb_workspace`. A second workspace in the same apply is rejected. Do not add another workspace in a later apply either. The provider cannot tell that request apart from adding the first workspace after the group exists, because `/v2/clusters` keeps the starter name (usually the group name). That later apply adopts the existing cluster and updates it to match the new workspace. Use `singlestoredb_cluster` for each additional deployment. A create that the API places in a different group is deleted and returned as an error.
- Destroying the adopted workspace deletes that cluster. If it is the only cluster in the group, the group is gone too. Destroy the workspace and the workspace group in the same apply, or migrate to `singlestoredb_cluster` before you remove the workspace resource.
- The live cluster name is usually the workspace group name. `/v2/clusters` often ignores a rename when the workspace is adopted. Terraform still stores the workspace `name` you configured. Data sources show the name the API returns.

## Move to singlestoredb_cluster

`singlestoredb_cluster` is the resource that matches `/v2/clusters`: one object for the workspace and its workspace group.

Do this against the existing deployment. Import reads the cluster; it does not create a new one.

A `moved` block cannot merge two resources into one. Remove both addresses from state, then import the cluster.

### 1. Record the IDs and the admin password

```shell
terraform state show singlestoredb_workspace.this
terraform state show singlestoredb_workspace_group.example
```

Copy these values before the next step. `terraform state rm` drops them from state.

- Workspace `id`. This is the cluster ID you will import. Do not import the workspace group `id`.
- Workspace group `id`. It becomes `group_id` on the cluster (computed).
- `admin_password` from the workspace group. It is sensitive and is not returned again by a later read.
- The live cluster name. Use the workspace group `name` unless you have confirmed the API renamed the cluster to the workspace `name`.

### 2. Replace the configuration

Remove `singlestoredb_workspace_group` and `singlestoredb_workspace`. Add `singlestoredb_cluster` with the attributes below.

Before:

```terraform
resource "singlestoredb_workspace_group" "example" {
  name            = "group"
  project_name    = "Standard Project"
  firewall_ranges = ["0.0.0.0/0"]
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
}

resource "singlestoredb_workspace" "this" {
  name               = "workspace-1"
  workspace_group_id = singlestoredb_workspace_group.example.id
  size               = "S-00"
  suspended          = false
}
```

After:

```terraform
resource "singlestoredb_cluster" "this" {
  # Live /v2/clusters name. This is usually the workspace group name.
  name            = "group"
  project_name    = "Standard Project"
  size            = "S-00"
  firewall_ranges = ["0.0.0.0/0"]
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
  suspended       = false

  # Omit admin_password to keep the generated password that import stores.
  # Set it only when you want Terraform to manage that password.
}
```

### 3. Move state and import

```shell
terraform state rm singlestoredb_workspace.this
terraform state rm singlestoredb_workspace_group.example
terraform import singlestoredb_cluster.this <workspace-id>
terraform plan
```

`terraform plan` should be empty after you align `name` and the other attributes with the imported state. A plan that wants to replace the cluster means an identity field does not match the live object. Fix the configuration. Do not apply a replace if you need to keep the data.

`endpoint` and `group_id` are computed. `id` is the workspace ID you imported.

### Attribute map

| Existing attribute | Cluster attribute |
| --- | --- |
| `singlestoredb_workspace.id` | `id` (import this value) |
| `singlestoredb_workspace_group.id` | `group_id` (computed) |
| `singlestoredb_workspace_group.name` | `name` (use the live API name) |
| `singlestoredb_workspace.size` | `size` |
| `singlestoredb_workspace.suspended` | `suspended` |
| `singlestoredb_workspace.kai_enabled` | `kai` |
| `singlestoredb_workspace.cache_config` | `cache_config` |
| `singlestoredb_workspace.scale_factor` | `scale_factor` |
| `singlestoredb_workspace.auto_scale` | `auto_scale` |
| `singlestoredb_workspace.auto_suspend` | `auto_suspend` |
| `singlestoredb_workspace.endpoint` | `endpoint` (computed) |
| `singlestoredb_workspace_group.admin_password` | `admin_password` |
| `singlestoredb_workspace_group.firewall_ranges` | `firewall_ranges` |
| `singlestoredb_workspace_group.expires_at` | `expires_at` |
| `singlestoredb_workspace_group.project_name` | `project_name` |
| `singlestoredb_workspace_group.cloud_provider` | `cloud_provider` |
| `singlestoredb_workspace_group.region_name` | `region_name` |
| `singlestoredb_workspace_group.region_id` | not on the cluster; use `cloud_provider` and `region_name` |
| `singlestoredb_workspace_group.deployment_type` | `deployment_type` |
| `singlestoredb_workspace_group.opt_in_preview_feature` | `opt_in_preview_feature` |
| `singlestoredb_workspace_group.high_availability_two_zones` | `multi_az` |
| `singlestoredb_workspace_group.update_window` | `update_window` |
| `singlestoredb_workspace_group.outbound_allow_list` | `outbound_allow_list` (computed) |

## Resources that reference the workspace

- `singlestoredb_flow` uses `cluster_id`. Set it to the same UUID that was `workspace_id` (the workspace / cluster ID).
- `singlestoredb_private_connection` prefers `cluster_id`. `workspace_id` and `workspace_group_id` remain as deprecated aliases. `workspace_id` is the cluster ID.
