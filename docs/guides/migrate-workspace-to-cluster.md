---
page_title: "Migrating from Workspace to Cluster - terraform-provider-singlestoredb"
description: |-
  Upgrade to provider v0.2.0. singlestoredb_workspace and singlestoredb_workspace_group stay supported through 0.2.x and are removed in the first release on or after April 30, 2027.
---

# Migrating from workspace to cluster

Provider v0.2.0 adopts the cluster model. Use `singlestoredb_cluster` for new configurations. One cluster replaces a workspace plus its workspace group.

`singlestoredb_workspace` and `singlestoredb_workspace_group` are deprecated in v0.2.0. Their arguments stay the same, and they receive bug fixes, through every 0.2.x release. Both resources are removed in the first provider release on or after April 30, 2027. If v0.2.0 is published after October 31, 2026, that removal date moves so the deprecation period is still six months. Upgrading does not require a state migration.

Under Management API `/v2/clusters`, a workspace and its workspace group are one cluster, so a few operations cannot work the way they did against `/v1/workspaceGroups` and `/v1/workspaces`.

Pin the provider version you run today until `terraform plan` is clean in a non-production configuration. This page is the upgrade notice: what changed, who must act, who can defer, and the removal date.

## Do I need to act before upgrading?

Act before you upgrade when a configuration has any of these:

- `singlestoredb_regions` uses the nested region UUID `id`. Use `region_name` and `provider` instead. `singlestoredb_regions_v2` is the data source for new configurations.
- A workspace group has more than one `singlestoredb_workspace`.
- A new workspace group sets `region_id`. New groups use `cloud_provider` and `region_name`.
- Automation destroys a workspace in a separate apply from its workspace group.
- `admin_password` is set and must stay that exact value. Omitting it lets `/v2/clusters` generate the password that Terraform stores.

You can upgrade and keep the deprecated resources when all of these are true:

- The configuration has one workspace group and one workspace.
- You have reviewed the behavior sections below.
- `singlestoredb_flow` still sets `workspace_id` to `singlestoredb_workspace.id`. That UUID is the cluster ID.
- `singlestoredb_private_connection` may keep `workspace_id` or `workspace_group_id` for now. Those arguments are temporary aliases. `cluster_id` is preferred, and the aliases are removed with the workspace resources.

Moving an existing deployment to `singlestoredb_cluster` is optional while the deprecated resources still work.

## Upgrade checklist

1. Back up Terraform state and commit `.terraform.lock.hcl`.
2. Pin v0.2.0 in a non-production configuration.
3. Replace region UUIDs with `region_name` and the cloud provider.
4. Run `terraform init -upgrade` and `terraform plan`. Do not apply a plan that destroys or replaces a deployment.
5. If you keep the deprecated resources, confirm the group has one workspace and that the workspace and group are destroyed in the same apply.
6. If you consolidate to `singlestoredb_cluster`, follow the state removal and import steps below and confirm the plan is empty.
7. Apply in production only after the non-production plan is clean.

## Roll back

Keep the state backup and the lock file from the previous provider version. To roll back, restore that lock file, set `required_providers` to the previous version, and run `terraform init`. Do not apply a plan produced while a migration is incomplete.

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

Do this against the existing deployment. Import reads the cluster. It does not create a new one.

A `moved` block cannot merge two resources into one. Remove both addresses from state, then import the cluster.

A plan that replaces the cluster can destroy the deployment and its data. Do not apply that plan. Fix the configuration until `terraform plan` is empty.

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

`terraform plan` should be empty after you align `name` and the other attributes with the imported state. A plan that wants to replace the cluster means an identity field does not match the live object. Fix the configuration. Applying a replacement destroys the cluster and its data.

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

- `singlestoredb_flow` keeps `workspace_id`. Set it to `singlestoredb_workspace.id`; that UUID is the cluster ID. The provider sends it to the v2 API as `clusterID`.
- `singlestoredb_private_connection` prefers `cluster_id`. `workspace_id` and `workspace_group_id` remain temporary aliases until the workspace resources are removed. `workspace_id` is the cluster ID.

## Support

Use this page for the impact list, commands, and rollback steps above. Configurations to review with customers before they upgrade:

- More than one workspace in a workspace group.
- `singlestoredb_regions` nested `id` values.
- Workspace group creates that set `region_id`.
- Automation that destroys a workspace separately from its workspace group.
- A configured `admin_password` that can drift from the password `/v2/clusters` stores.

Escalation:

- Provider behavior and this guide: [open a GitHub issue](https://github.com/singlestore-labs/terraform-provider-singlestoredb/issues/new).
- Account or deployment access: [SingleStore support](https://www.singlestore.com/support/).

## Two-week review

For the first two weeks after v0.2.0 is published, review GitHub issues and failed migrations daily. Correct this guide when a step is wrong. Keep the April 30, 2027 removal date unless those reports show customers cannot finish the import procedure in six months. A change to that date must name the last 0.2.x release and the new removal date.
