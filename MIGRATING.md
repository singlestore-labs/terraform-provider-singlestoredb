# Migrating from workspace resources

`singlestoredb_cluster` is one deployment. It replaces the pair `singlestoredb_workspace` and `singlestoredb_workspace_group`.

In the cloud, the migration has already happened. Every workspace is already a cluster: `GET /v2/clusters/{id}` returns it under the workspace id, with the workspace group id as its `group_id`. What is left is Terraform's side. This migration moves the configuration and the state from `singlestoredb_workspace` and `singlestoredb_workspace_group` to `singlestoredb_cluster`. It creates, changes, and deletes no cloud resource. The workspace keeps running with the same id, endpoint, admin password, and data.

A `moved` block renames one resource address, so it cannot merge the two resources into one. Instead, one apply imports each workspace as a cluster and drops the old resources from state, with Terraform generating the cluster configuration from the live deployment.

These steps need a workspace to import. A workspace group with no workspace has no cluster to import, and removing it from state leaves it unmanaged. Keep managing an empty group with `singlestoredb_workspace_group` until v1.0.0, or destroy it with that resource if you no longer need it. Do not remove it from state.

## Schedule

`singlestoredb_workspace` and `singlestoredb_workspace_group` keep their current behavior in v0.2.0. `terraform plan` prints a deprecation warning. The data sources `singlestoredb_workspace`, `singlestoredb_workspaces`, `singlestoredb_workspace_group`, and `singlestoredb_workspace_groups` do the same.

These resources and data sources are removed in v1.0.0, the first release on or after August 20, 2027. New configurations use `singlestoredb_cluster`.

## Example used in this guide

The commands and configuration below use this deployment: a workspace group named `group` with one workspace. The addresses are `singlestoredb_workspace_group.example_wg` and `singlestoredb_workspace.example_workspace`. Replace them with yours.

```hcl
resource "singlestoredb_workspace_group" "example_wg" {
  name            = "group"
  project_name    = "Standard Project"
  firewall_ranges = ["0.0.0.0/0"]
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
  admin_password  = "mockPassword193!"
}

resource "singlestoredb_workspace" "example_workspace" {
  name               = "workspace-1"
  workspace_group_id = singlestoredb_workspace_group.example_wg.id
  size               = "S-00"
  suspended          = false

  auto_suspend = {
    suspend_type          = "SCHEDULED"
    suspend_after_seconds = 3600
  }
}

output "endpoint" {
  value = singlestoredb_workspace.example_workspace.endpoint
}

output "admin_password" {
  value     = singlestoredb_workspace_group.example_wg.admin_password
  sensitive = true
}
```

## Before you start

You need Terraform 1.7 or later for `removed` blocks, and `jq`. On older Terraform, see [Terraform older than 1.7](#terraform-older-than-17).

Change no cloud resources from step 1 until the migration is applied in step 4. Run no other `terraform apply` in this state, and make no changes in the SingleStore Portal or through Management API to anything this state manages. The migration plan must show `0 to change`: any change it shows is a mistake, not part of the migration. Make size, scaling, firewall, and suspend changes after the migration, in separate applies. Keeping the cloud resources unchanged is also what makes [rollback](#roll-back) a matter of putting the configuration and state back.

Check the deployment name. The cluster `name` is the workspace `name`, not the workspace group name, and it must be 1 to 32 characters. Management API rejects longer names when it creates a workspace, but an older workspace can have one, and a workspace cannot be renamed: `PATCH /v2/clusters/{id}` returns success and keeps the old name. If a workspace name is longer than 32 characters, first follow [A workspace name longer than 32 characters](#a-workspace-name-longer-than-32-characters).

A suspended workspace migrates as a suspended cluster. Leave it suspended for the migration. Afterwards, the provider rejects changing `suspended` together with other configuration, and rejects other changes while the cluster is suspended: resume in one apply, change settings in the next, and suspend again in a third.

## 1. Back up configuration and state, and save the password

Commit the current configuration, so you can restore it exactly. Then save the state while the workspace resources are still in it. Terraform overwrites `terraform.tfstate.backup` on every state change, so keep this copy under a different name. Together, the commit and the backup are your [rollback](#roll-back).

```shell
(umask 077 && terraform state pull > pre-cluster-migration.tfstate)
```

The backup holds the admin password in plain text. Keep it out of shared logs and version control.

Print the workspace group password and store it in your secret store:

```shell
jq -r '.resources[] | select(.type == "singlestoredb_workspace_group") | "\(.name): \(.instances[0].attributes.admin_password)"' pre-cluster-migration.tfstate
```

The migration drops the password from state, and import cannot recover it: Management API returns the cluster password only on create. The SQL password itself does not change. See [Admin password](#admin-password).

If the workspace group has an expiration, note it now. After the migration Terraform no longer shows it. See [Expiration](#expiration).

```shell
jq -r '.resources[] | select(.type == "singlestoredb_workspace_group") | "\(.name): \(.instances[0].attributes.expires_at)"' pre-cluster-migration.tfstate
```

## 2. Generate the cluster configuration

Write an `import` block for every workspace. Each cluster gets the address of its workspace, so `singlestoredb_workspace.example_workspace` becomes `singlestoredb_cluster.example_workspace`:

```shell
jq -r '.resources[]
  | select(.mode == "managed" and .module == null and .each == null and .type == "singlestoredb_workspace")
  | "import {\n  to = singlestoredb_cluster.\(.name)\n  id = \"\(.instances[0].attributes.id)\"\n}\n"' \
  pre-cluster-migration.tfstate > cluster-imports.tf
```

The command covers resources in the root module without `count` or `for_each`. For others, write the block by hand with the full address, for example `to = module.db.singlestoredb_cluster.this["eu"]`, and the workspace id as `id`.

Leave the workspace resources in configuration for now, and generate the cluster resources from the live deployment:

```shell
terraform plan -generate-config-out=cluster.tf
```

This plan changes nothing. It shows `N to import, 0 to add, 0 to change, 0 to destroy`, and writes `cluster.tf`. For the [example](#example-used-in-this-guide):

```hcl
resource "singlestoredb_cluster" "example_workspace" {
  auto_scale = {
    max_scale_factor = 1
    sensitivity      = "NORMAL"
  }
  auto_suspend = {
    suspend_after_seconds = 3600
    suspend_type          = "SCHEDULED"
  }
  cache_config           = 1
  cloud_provider         = "AWS"
  deployment_type        = "PRODUCTION"
  expires_at             = null
  firewall_ranges        = ["0.0.0.0/0"]
  kai                    = false
  multi_az               = false
  name                   = "workspace-1"
  opt_in_preview_feature = false
  project_name           = "Standard Project"
  region_name            = "us-east-1"
  scale_factor           = 1
  size                   = "S-00"
  suspended              = false
}
```

The values come from the deployment, so do not edit them during the migration. The one exception is `suspended` when `auto_suspend` is enabled: delete that line. Omitting `suspended` keeps the current status, so the plan stays clean if auto-suspend suspends the workspace during the migration, and later applies do not resume it. This includes `firewall_ranges`, which keeps the order and normalization the API returns, and `expires_at = null`: see [Expiration](#expiration). Read-only attributes such as `endpoint`, `group_id`, and `update_window` are not generated.

## 3. Replace the old resources

Write a `removed` block for every workspace and workspace group. `destroy = false` drops them from state without deleting them:

```shell
jq -r '.resources[]
  | select(.mode == "managed" and .module == null and .each == null)
  | select(.type == "singlestoredb_workspace" or .type == "singlestoredb_workspace_group")
  | "removed {\n  from = \(.type).\(.name)\n  lifecycle {\n    destroy = false\n  }\n}\n"' \
  pre-cluster-migration.tfstate > workspace-removed.tf
```

Then edit the configuration:

- Delete the `singlestoredb_workspace` and `singlestoredb_workspace_group` blocks.
- Point references to the workspace `id` or `endpoint` at the cluster, for example `singlestoredb_cluster.example_workspace.endpoint`.
- Point references to the workspace group `id` at `singlestoredb_cluster.example_workspace.group_id`.
- Point references to the group `admin_password` at the password you saved, through a sensitive variable or a secret store. Not at the cluster.

For the [example](#example-used-in-this-guide), the rest of the configuration becomes:

```hcl
variable "admin_password" {
  type      = string
  sensitive = true
}

output "endpoint" {
  value = singlestoredb_cluster.example_workspace.endpoint
}

output "admin_password" {
  value     = var.admin_password
  sensitive = true
}
```

[Resources that point at a workspace](#resources-that-point-at-a-workspace) covers flows, private connections, and SQL resources.

## 4. Plan and apply

```shell
terraform plan -out=migrate.tfplan
```

Apply only when the summary is `N to import, 0 to add, 0 to change, 0 to destroy`, with `N` the number of workspaces, and the warning lists every workspace and workspace group as no longer managed. Output values do not change, because the cluster id and endpoint are the workspace id and endpoint.

Anything else means a step was missed:

- A destroy of `singlestoredb_workspace` or `singlestoredb_workspace_group` means a `removed` block is missing. Do not apply: it deletes the deployment and its data.
- A change on a cluster means `cluster.tf` was edited, or the deployment changed after step 2. Restore the generated value, or generate `cluster.tf` again.
- A change on another resource means a reference now produces a different value. Point it at the matching cluster attribute.
- `Reference to undeclared resource` means a reference to a deleted block remains.

Then apply the saved plan, and confirm that the next plan is clean:

```shell
terraform apply migrate.tfplan
terraform plan
```

Delete `cluster-imports.tf` and `workspace-removed.tf` after the apply. Move the resources from `cluster.tf` into your configuration if you like; the file needs no other change.

If anything goes wrong, see [Roll back](#roll-back).

## Expiration

The cluster has no workspace group expiration. `GET /v2/clusters/{id}` does not return it, so import stores no `expires_at`, and the plan fails with `Cannot change cluster expires_at` if the configuration sets one. Leave `expires_at` unset or `null` on a migrated cluster.

The workspace group still expires at its `expires_at`. Extend or remove it in the SingleStore Portal if needed.

## Admin password

Management API generates the cluster admin password, ignores a password sent in the request, and returns it only on create. `admin_password` on `singlestoredb_cluster` is read-only. Setting it in the cluster configuration is unavailable.

A migrated cluster has no `admin_password` in state, and refresh does not fill it. The SQL password is still the workspace group password saved in [step 1](#1-back-up-configuration-and-state-and-save-the-password). Use that value wherever configuration used `singlestoredb_workspace_group.example_wg.admin_password`. Do not point those references at `singlestoredb_cluster.example_workspace.admin_password`. The first update after the migration shows `+ admin_password = (sensitive value)`; the value stays empty.

## Attribute map

| Existing attribute                                          | Cluster attribute                                              |
| ----------------------------------------------------------- | -------------------------------------------------------------- |
| `singlestoredb_workspace.id`                                | `id` (import this value)                                       |
| `singlestoredb_workspace_group.id`                          | `group_id` (computed)                                          |
| `singlestoredb_workspace.name`                              | `name`                                                         |
| `singlestoredb_workspace.size`                              | `size`                                                         |
| `singlestoredb_workspace.suspended`                         | `suspended`                                                    |
| `singlestoredb_workspace.kai_enabled`                       | `kai`                                                          |
| `singlestoredb_workspace.cache_config`                      | `cache_config`                                                 |
| `singlestoredb_workspace.scale_factor`                      | `scale_factor`                                                 |
| `singlestoredb_workspace.auto_scale`                        | `auto_scale`                                                   |
| `singlestoredb_workspace.auto_suspend`                      | `auto_suspend`                                                 |
| `singlestoredb_workspace.endpoint`                          | `endpoint` (computed)                                          |
| `singlestoredb_workspace_group.admin_password`              | none (see [Admin password](#admin-password))                   |
| `singlestoredb_workspace_group.firewall_ranges`             | `firewall_ranges`                                              |
| `singlestoredb_workspace_group.expires_at`                  | none; leave `expires_at` unset (see [Expiration](#expiration)) |
| `singlestoredb_workspace_group.project_name`                | `project_name`                                                 |
| `singlestoredb_workspace_group.cloud_provider`              | `cloud_provider`                                               |
| `singlestoredb_workspace_group.region_name`                 | `region_name`                                                  |
| `singlestoredb_workspace_group.deployment_type`             | `deployment_type`                                              |
| `singlestoredb_workspace_group.opt_in_preview_feature`      | `opt_in_preview_feature`                                       |
| `singlestoredb_workspace_group.high_availability_two_zones` | `multi_az`                                                     |
| `singlestoredb_workspace_group.update_window`               | `update_window` (computed; read-only)                          |
| `singlestoredb_workspace_group.outbound_allow_list`         | `outbound_allow_list` (computed)                               |

These attributes have no cluster argument:

- `singlestoredb_workspace_group.name`. The group name is not used by the cluster.
- `singlestoredb_workspace.workspace_group_id`. Use the computed `group_id`.
- `singlestoredb_workspace_group.region_id`. The cluster uses `cloud_provider` and `region_name`, and the generated configuration fills them in.
- `singlestoredb_workspace.created_at` and `singlestoredb_workspace_group.created_at`. The cluster `created_at` is the cluster's own timestamp.
- The workspace data source attribute `last_resumed_at`.

The provider rejects a plan that changes any of these: `name`, `project_name`, `cloud_provider`, `region_name`, `kai`, `multi_az`, `opt_in_preview_feature`, `expires_at`. It also rejects configuring `update_window`, and changing `deployment_type` away from `NON-PRODUCTION` while `opt_in_preview_feature` is true. The provider does not schedule a replacement for them.

`size`, `scale_factor`, `cache_config`, `firewall_ranges`, `deployment_type`, `auto_scale`, `auto_suspend`, and `suspended` update in place after the migration.

## A workspace group with several workspaces

Each workspace becomes its own cluster, and the clusters share `group_id`. The commands in steps 2 and 3 already write one `import` block per workspace and one `removed` block for the group, so the steps are the same. Each cluster's `name` is its workspace name.

After the migration plan is clean, you can keep the shared settings in one place so they cannot drift apart. This changes expressions, not values, so the next plan should show no changes:

```hcl
locals {
  group = {
    project_name    = "Standard Project"
    firewall_ranges = ["0.0.0.0/0"]
    cloud_provider  = "AWS"
    region_name     = "us-east-1"
  }
}

resource "singlestoredb_cluster" "one" {
  name            = "workspace-1"
  size            = "S-00"
  project_name    = local.group.project_name
  firewall_ranges = local.group.firewall_ranges
  cloud_provider  = local.group.cloud_provider
  region_name     = local.group.region_name
}

resource "singlestoredb_cluster" "two" {
  name            = "workspace-2"
  size            = "S-00"
  project_name    = local.group.project_name
  firewall_ranges = local.group.firewall_ranges
  cloud_provider  = local.group.cloud_provider
  region_name     = local.group.region_name
}
```

Clusters that share a `group_id` share one firewall and one deployment type. Each cluster resource sends its own `firewall_ranges` and `deployment_type` to that group, so when two clusters disagree, every apply switches the group to the other cluster's values and the next plan shows a change again. Clients lose access while the group has the wrong list. This happens when one block is edited and the other is not, or when the clusters are managed from different configurations. Keep the shared values in one `locals` block, as above, and manage every cluster in a group from the same configuration.

`project_name`, `cloud_provider`, `region_name`, `multi_az`, and `opt_in_preview_feature` also belong to the group, but cannot change after creation. Configuring a different value fails the plan.

## A workspace name longer than 32 characters

No cluster configuration can match a workspace name longer than 32 characters, and the workspace cannot be renamed. Unlike the migration, the fix below changes cloud resources, so finish it as a separate change, with a clean `terraform plan`, before step 1. Move its databases to a new workspace in the same workspace group, then migrate the new workspace. The databases stay in the group's storage, so `DETACH DATABASE` and `ATTACH DATABASE` move them without copying data. The new workspace shares the group's firewall ranges, admin password, and private connections that have no `workspace_id`.

1. Add a `singlestoredb_workspace` with a name of 1 to 32 characters to the same workspace group, with the size and settings of the old one. Apply. This only creates the new workspace.

   ```hcl
   resource "singlestoredb_workspace" "new" {
     name               = "workspace-2"
     workspace_group_id = singlestoredb_workspace_group.example_wg.id
     size               = "S-00"
   }
   ```

2. Stop writes to the databases. For each database, detach it on the old workspace endpoint and attach it on the new one, as the admin user:

   ```sql
   -- on the old workspace endpoint
   DETACH DATABASE my_database;
   -- on the new workspace endpoint
   ATTACH DATABASE my_database;
   ```

   Check the data on the new endpoint. `SHOW DATABASES` on the old endpoint should list only system databases.

3. Point applications and configuration at the new workspace. Its `id` and `endpoint` differ from the old workspace's. The provider rejects changing `workspace_id` on `singlestoredb_flow` and on `singlestoredb_private_connection`, so recreate those resources against the new workspace. Changing `endpoint` on `singlestoredb_sql_execute` replaces it and runs `revert` against the old endpoint; see [SQL](#sql).

4. Remove the old `singlestoredb_workspace` from configuration and apply. The plan destroys only the old workspace. The workspace group and the new workspace stay.

5. Migrate the new workspace with the rest of this guide, starting at [Back up state and save the password](#1-back-up-configuration-and-state-and-save-the-password).

## Deleting a migrated deployment

Destroying `singlestoredb_cluster` deletes the workspace. When it is the last workspace in its group, Management API also terminates the workspace group. While other workspaces share the group, the group and those workspaces keep running, with the same firewall ranges and admin password.

## Data sources

| Existing data source             | Replacement                                                                                                                                                                                                                                                                                                           |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `singlestoredb_workspace`        | `singlestoredb_cluster`, looked up by `id` (the workspace id). The cluster data source has no `name` argument.                                                                                                                                                                                                        |
| `singlestoredb_workspaces`       | `singlestoredb_clusters`. The filters are an optional `project_id` and `include_terminated`. There is no `workspace_group_id` filter. Each entry's `group_id` is the old workspace group id, and `id` is the old workspace id.                                                                                        |
| `singlestoredb_workspace_group`  | `singlestoredb_cluster`. Group fields (`firewall_ranges`, `region_name`, `multi_az`, and the rest of the map above) are attributes of the cluster. The group expiration is not available.                                                                                                                             |
| `singlestoredb_workspace_groups` | `singlestoredb_clusters`. This lists clusters, not groups. A workspace group with no workspaces has no cluster entry, so it does not appear. A group with several workspaces appears once per workspace, and the entries share a `group_id`. Do not use the list as a group inventory or count its entries as groups. |

On the cluster data source, `kai_enabled` is `kai`, `workspace_group_id` is `group_id`, and `high_availability_two_zones` is `multi_az`. `last_resumed_at`, `allow_all_traffic`, `project_name`, and `region_id` are absent. Unrestricted access is `firewall_ranges` containing `0.0.0.0/0`.

The workspace data sources still read the v1 API in v0.2.0, and `terraform plan` prints a deprecation warning. New configuration uses the cluster data sources.

## Resources that point at a workspace

Keep the same ids and the same SQL endpoint. The values do not change because the deployment does not change. Update the expressions that produce them.

### Flow

`singlestoredb_flow.workspace_id` is the cluster id. Set it to `singlestoredb_cluster.example_workspace.id`. That expression still evaluates to the workspace id, so the value is unchanged. The provider rejects a plan that changes it.

```hcl
resource "singlestoredb_flow" "this" {
  name          = "my-flow-instance"
  workspace_id  = singlestoredb_cluster.example_workspace.id
  user_name     = "admin"
  database_name = "my_database"
  size          = "F1"
}
```

### Private connection

`singlestoredb_private_connection` is unchanged. `workspace_group_id` is required, and `workspace_id` is optional. Keep their values: the provider rejects a change. Set `workspace_group_id` to `singlestoredb_cluster.example_workspace.group_id`, which is the old workspace group id. When the connection also set `workspace_id` from `singlestoredb_workspace.example_workspace.id`, set it to `singlestoredb_cluster.example_workspace.id`, which is the old workspace id.

```hcl
resource "singlestoredb_private_connection" "this" {
  allow_list         = "651246146166"
  type               = "INBOUND"
  workspace_group_id = singlestoredb_cluster.example_workspace.group_id
  workspace_id       = singlestoredb_cluster.example_workspace.id
}
```

### SQL

`singlestoredb_sql_execute` and `singlestoredb_sql_query` take the SQL host and the admin password. Point the host at the cluster and the password at the saved value:

```hcl
resource "singlestoredb_sql_execute" "this" {
  endpoint = singlestoredb_cluster.example_workspace.endpoint
  username = "admin"
  password = var.admin_password

  execute = "CREATE DATABASE IF NOT EXISTS ?"
  # ...
}
```

`endpoint` must stay the same host. Changing it replaces `singlestoredb_sql_execute`, and the replacement runs `revert` against the previous endpoint. The cluster endpoint is the workspace endpoint for this deployment.

## Roll back

To undo the migration, put the original configuration and state back. Neither the migration nor the rollback changes the deployment: both change only Terraform state, so the workspace keeps running and its id, endpoint, and data stay the same. That is also why the state saved in step 1 still describes the deployment.

These commands work at any point: before the migration apply, after it, and after an apply that failed partway.

```shell
git restore --source=<commit-from-step-1> --worktree -- .
rm -f cluster.tf cluster-imports.tf workspace-removed.tf migrate.tfplan
terraform state push -force pre-cluster-migration.tfstate
terraform plan
```

Use the commit you made in step 1, or restore the files another way. `-force` is needed because the migration apply raised the state serial. It also brings back the workspace group `admin_password`, which the migration dropped from state.

The plan should show no changes. Apply nothing that destroys a resource: it deletes the deployment.

### After later changes

The backup is current only until something else changes state. Decide on a rollback before you make other changes. Once later applies have run, pushing the backup would lose them. Going back to the workspace resources is then a migration in the other direction: an `import` block for each workspace and workspace group, and a `removed` block with `destroy = false` for each cluster. Import cannot read the workspace group `admin_password`, so that plan sends the configured password again; make sure configuration has the current one.

## Terraform older than 1.7

Terraform 1.5 and 1.6 support `import` blocks and `-generate-config-out`, but not `removed` blocks. Follow steps 1 to 3 without `workspace-removed.tf`. Right before step 4, remove the old resources from state with the commands below, then plan. Without them, the plan destroys the deployment.

```shell
terraform state rm singlestoredb_workspace.example_workspace
terraform state rm singlestoredb_workspace_group.example_wg
```

To roll back on these versions, use the same [rollback](#roll-back) commands.

Before Terraform 1.5, upgrade Terraform for the migration.
