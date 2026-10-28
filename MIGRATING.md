# Migrating from workspace resources

`singlestoredb_cluster` is one deployment. It replaces the pair `singlestoredb_workspace` and `singlestoredb_workspace_group`.

The cluster `id` is the workspace id. The cluster `group_id` is the workspace group id, and it is computed. A `moved` block renames one resource address, so it cannot merge the two resources into one. Adopt the existing deployment by removing both resources from state and importing the workspace id.

This adopts the deployment that is already running. It does not create a second cluster, and `terraform state rm` does not delete it.

## Schedule

`singlestoredb_workspace` and `singlestoredb_workspace_group` keep their current behavior in v0.2.0. `terraform plan` prints a deprecation warning. The data sources `singlestoredb_workspace`, `singlestoredb_workspaces`, `singlestoredb_workspace_group`, and `singlestoredb_workspace_groups` do the same.

These resources and data sources are removed in v0.3.0, the first release on or after April 30, 2027. `singlestoredb_private_connection.workspace_id` and `workspace_group_id` are removed in that same release. New configurations use `singlestoredb_cluster`.

## Before you edit configuration

Run this while the workspace resources are still in state. Replace the addresses with yours.

```shell
touch terraform.tfstate.backup
chmod 600 terraform.tfstate.backup
terraform state pull > terraform.tfstate.backup
terraform state show singlestoredb_workspace.this
terraform state show singlestoredb_workspace_group.example
```

Save these values outside the terminal scrollback:

- The workspace `id`. This is the import id.
- The workspace group `id`. After import it is `group_id` on the cluster. You do not set it.
- `admin_password` from the workspace group. `terraform state show` redacts sensitive values, so retrieve it from the protected raw-state backup above or an existing sensitive output. The Management API does not return it again, and `terraform state rm` drops it from state. Keep the password and backup out of shared logs.

Resume a suspended workspace before you migrate. The first apply after import updates `admin_password`, and the provider rejects any configuration change, including that password update, while the cluster is suspended. It also rejects changing `suspended` in the same apply as any other configuration. Resume with the workspace resource, migrate, then set `suspended = true` on the cluster in a later apply if the deployment should be suspended again.

If the deployment must stay suspended through the import, omit `admin_password` on the cluster until you can resume it. Import then leaves the password empty in state and does not send one. The password on the deployment stays as it is. Set `admin_password` only after a later resume, in its own apply.

## Rewrite the configuration

Remove `singlestoredb_workspace` and `singlestoredb_workspace_group`. Add `singlestoredb_cluster` with the same deployment. Point every reference that used the workspace id, endpoint, or group password at the cluster. Do not apply yet. Applying after the old resources are gone from configuration and still present in state destroys them.

A workspace group named `group` with one workspace looks like this before the move:

```hcl
resource "singlestoredb_workspace_group" "example" {
  name            = "group"
  project_name    = "Standard Project"
  firewall_ranges = ["0.0.0.0/0"]
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
```

After the move, `name` is the name `GET /v2/clusters/{id}` returns. That is usually the workspace group name, `group` here, and it is not the workspace name `workspace-1`. The provider rejects a later rename.

```hcl
resource "singlestoredb_cluster" "this" {
  name            = "group"
  project_name    = "Standard Project"
  size            = "S-00"
  firewall_ranges = ["0.0.0.0/0"]
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
  admin_password  = "mockPassword193!"
  suspended       = false
}

output "endpoint" {
  value = singlestoredb_cluster.this.endpoint
}

output "admin_password" {
  value     = singlestoredb_cluster.this.admin_password
  sensitive = true
}

output "group_id" {
  value = singlestoredb_cluster.this.group_id
}
```

`firewall_ranges` is required. `name` must be 1 to 32 characters. Set `cloud_provider` and `region_name` together, or omit both and the imported state keeps them.

Copy every live value you previously set. Several attributes default when omitted, and an omitted value that differs from the deployment is a real update:

| Attribute | Value used when omitted |
| --- | --- |
| `scale_factor` | `1` |
| `cache_config` | `1` |
| `suspended` | `false` |
| `kai` | `false` |
| `deployment_type` | `PRODUCTION` |
| `opt_in_preview_feature` | `false` |
| `multi_az` | `false` |
| `auto_scale.max_scale_factor` | `1` |
| `auto_scale.sensitivity` | `NORMAL` |
| `auto_suspend.suspend_type` | `DISABLED` |
| `skip_admin_password_validation` | `false` |

`kai`, `multi_az`, and `opt_in_preview_feature` cannot be changed after import. If the live value is `true` and the configuration leaves the default `false`, the plan fails until the configuration matches state. `expires_at` cannot be removed: when the workspace group has one, set the same RFC3339 timestamp on the cluster.

## Attribute map

| Existing attribute | Cluster attribute |
| --- | --- |
| `singlestoredb_workspace.id` | `id` (import this value) |
| `singlestoredb_workspace_group.id` | `group_id` (computed) |
| `singlestoredb_workspace_group.name` | `name` (the live `/v2/clusters` name, usually this name) |
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
| `singlestoredb_workspace_group.deployment_type` | `deployment_type` |
| `singlestoredb_workspace_group.opt_in_preview_feature` | `opt_in_preview_feature` |
| `singlestoredb_workspace_group.high_availability_two_zones` | `multi_az` |
| `singlestoredb_workspace_group.update_window` | `update_window` |
| `singlestoredb_workspace_group.outbound_allow_list` | `outbound_allow_list` (computed) |

These attributes have no cluster argument:

- `singlestoredb_workspace.name`. The cluster name comes from the clusters API.
- `singlestoredb_workspace.workspace_group_id`. Use the computed `group_id`.
- `singlestoredb_workspace_group.region_id`. The cluster uses `cloud_provider` and `region_name`. `singlestoredb_regions.regions[*].id` is removed in v0.3.0, the first release on or after April 30, 2027; use `regions[*].provider` and `regions[*].region_name`.
- `singlestoredb_workspace.created_at` and `singlestoredb_workspace_group.created_at`. The cluster `created_at` is the cluster's own timestamp.
- The workspace data source attribute `last_resumed_at`.

`created_at`, `endpoint`, `group_id`, and `outbound_allow_list` are read from the API after import.

Firewall ranges are compared as a set. The API may return them in a different order, and unrestricted access (`0.0.0.0/0`) may come back as an allow-all flag. A configuration whose ranges are the same set produces no diff.

The provider rejects a plan that changes any of these: `name`, `project_name`, `cloud_provider`, `region_name`, `kai`, `multi_az`, `opt_in_preview_feature`. It also rejects removing `expires_at`, and changing `deployment_type` away from `NON-PRODUCTION` while `opt_in_preview_feature` is true. Fix the configuration so those values match the imported state. The provider does not schedule a replacement for them.

`size`, `scale_factor`, `cache_config`, `firewall_ranges`, `expires_at` (a new timestamp), `deployment_type`, `admin_password`, `auto_scale`, `auto_suspend`, `update_window`, and `suspended` update in place. Resume the cluster before any of those updates. Change `suspended` in an apply by itself.

## Move state

`terraform state rm` drops the resources from state and leaves the deployment running. `terraform destroy` on the workspace resources deletes the deployment and its data.

```shell
terraform state rm singlestoredb_workspace.this
terraform state rm singlestoredb_workspace_group.example
terraform import singlestoredb_cluster.this <workspace-id>
```

On Terraform 1.5 and later, an `import` block can replace the `terraform import` command. Remove the old resources from state before the next plan, or that plan destroys them.

```hcl
import {
  to = singlestoredb_cluster.this
  id = "<workspace-id>"
}
```

Then run `terraform plan`. After the import has been applied, delete the `import` block.

Import reads the deployment and stores an empty `admin_password`, because the API does not return the password.

## Read the plan before you apply

When the configuration matches the deployment, `terraform plan` shows one in-place update of `admin_password`. State holds an empty value, and the apply sends the configured password to the cluster. That is the password you saved. It is the same password the deployment already uses, unless you intentionally set a new one.

Any other change means the configuration does not match the live deployment. A plan that destroys `singlestoredb_workspace`, `singlestoredb_workspace_group`, or `singlestoredb_cluster` deletes that deployment and its data. Stop there. The usual cause is an apply while the old resources are still in state, or a cluster resource that was never imported. Fix the configuration, or restore the backup from [Before you edit configuration](#before-you-edit-configuration), and plan again.

A plan error such as `Cannot update cluster name` means an attribute in the list above does not match imported state. The error prints the current value. Put that value in configuration. Applying is blocked until the plan succeeds.

`terraform state show singlestoredb_cluster.this` after import is the source for `name`, `size`, `cloud_provider`, `region_name`, `project_name`, and the rest of the live attributes. Set `name` from that output. The workspace resource's `name` is a different string.

## Admin password

Workspace groups required only 14 characters. The cluster checks the Management API password rules during plan. A configured password needs at least 14 characters, an uppercase letter, a lowercase letter, a digit, and a special character, at most 2 consecutive sequential characters (such as `ab` or `12`), and at most 3 identical characters in a row.

A saved password can fail that check. Keep it with:

```hcl
resource "singlestoredb_cluster" "this" {
  # ...
  admin_password                 = var.admin_password
  skip_admin_password_validation = true
}
```

Or set a new password that meets the rules. That apply changes the SQL admin password on the deployment.

`skip_admin_password_validation` turns off the plan check. Use it for a password the deployment already has, or when the API rules change before the provider is updated. The [README](README.md#cluster-admin-password) describes the create-time check, which applies to new clusters and does not run on this import.

## A workspace group with several workspaces

Each workspace is its own cluster. Import each workspace id as a separate `singlestoredb_cluster`. Remove the workspace group from state once. The clusters share `group_id`.

```shell
terraform state rm singlestoredb_workspace.one
terraform state rm singlestoredb_workspace.two
terraform state rm singlestoredb_workspace_group.example
terraform import singlestoredb_cluster.one <workspace-one-id>
terraform import singlestoredb_cluster.two <workspace-two-id>
```

After each import, copy that cluster's `name` from `terraform state show`. The clusters API supplies the name, which is usually the workspace group name.

## Data sources

| Existing data source | Replacement |
| --- | --- |
| `singlestoredb_workspace` | `singlestoredb_cluster`, looked up by `id` (the workspace id). The cluster data source has no `name` argument. |
| `singlestoredb_workspaces` | `singlestoredb_clusters`. The filter is an optional `project_id`. There is no `workspace_group_id` filter. Each entry's `group_id` is the old workspace group id, and `id` is the old workspace id. |
| `singlestoredb_workspace_group` | `singlestoredb_cluster`. Group fields (`firewall_ranges`, `region_name`, `expires_at`, `multi_az`, and the rest of the map above) are attributes of the cluster. |
| `singlestoredb_workspace_groups` | `singlestoredb_clusters`. This lists clusters, not groups. |

On the cluster data source, `kai_enabled` is `kai`, `workspace_group_id` is `group_id`, and `high_availability_two_zones` is `multi_az`. `last_resumed_at` and `allow_all_traffic` are absent. Unrestricted access is `firewall_ranges` containing `0.0.0.0/0`.

The workspace data sources still read the v1 API in v0.2.0, and `terraform plan` prints a deprecation warning. New configuration uses the cluster data sources.

## Resources that point at a workspace

Keep the same ids and the same SQL endpoint. The values do not change because the deployment does not change. Update the expressions that produce them.

### Flow

`singlestoredb_flow.workspace_id` is the cluster id. Set it to `singlestoredb_cluster.this.id`. That expression still evaluates to the workspace id, so the value is unchanged. The provider rejects a plan that changes it.

```hcl
resource "singlestoredb_flow" "this" {
  name          = "my-flow-instance"
  workspace_id  = singlestoredb_cluster.this.id
  user_name     = "admin"
  database_name = "my_database"
  size          = "F1"
}
```

### Private connection

Set `cluster_id` to `singlestoredb_cluster.this.id`. That is the same UUID as `workspace_id`. The provider rejects a plan that changes `cluster_id`, `workspace_id`, or `workspace_group_id` to a different value.

`workspace_id` is a deprecated alias for `cluster_id`. `workspace_group_id` is a deprecated alias used only when `cluster_id` and `workspace_id` are unset. A group id on its own selects the cluster when the group contains exactly one non-terminated workspace. A group with several workspaces needs `cluster_id`. Both aliases are removed in v0.3.0, the first release on or after April 30, 2027.

```hcl
resource "singlestoredb_private_connection" "this" {
  allow_list = "651246146166"
  type       = "INBOUND"
  cluster_id = singlestoredb_cluster.this.id
}
```

### SQL

`singlestoredb_sql_execute` and `singlestoredb_sql_query` take the SQL host and the admin password. Point them at the cluster:

```hcl
resource "singlestoredb_sql_execute" "this" {
  endpoint = singlestoredb_cluster.this.endpoint
  username = "admin"
  password = singlestoredb_cluster.this.admin_password

  execute = "CREATE DATABASE IF NOT EXISTS ?"
  # ...
}
```

`endpoint` must stay the same host. Changing it replaces `singlestoredb_sql_execute`, and the replacement runs `revert` against the previous endpoint. The cluster endpoint is the workspace endpoint for this deployment.

## Put the old resources back

`terraform state rm` and a failed import leave the deployment in the cloud and out of state. Restore the previous configuration from version control, then import the old addresses. Use the ids you saved. Put `admin_password` back on the workspace group yourself.

```shell
terraform import singlestoredb_workspace_group.example <workspace-group-id>
terraform import singlestoredb_workspace.this <workspace-id>
```

When the cluster import already succeeded, drop the cluster from state before importing the old addresses. `terraform state rm` leaves the deployment running.

```shell
terraform state rm singlestoredb_cluster.this
```

Apply the restored configuration only after those imports. The plan should show no destroy of the cluster. A destroy deletes the deployment.
