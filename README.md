# Terraform Provider for SingleStoreDB Cloud

[![Unit](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/unit.yml/badge.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions)
[![Integration](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/integration.yml/badge.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/singlestore-labs/terraform-provider-singlestoredb)](https://goreportcard.com/report/github.com/singlestore-labs/terraform-provider-singlestoredb)
[![codecov](https://codecov.io/gh/singlestore-labs/terraform-provider-singlestoredb/branch/master/graph/badge.svg?token=BT65KGONQ6)](https://codecov.io/gh/singlestore-labs/terraform-provider-singlestoredb)
[![License](https://img.shields.io/github/license/singlestore-labs/terraform-provider-singlestoredb.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/blob/master/LICENSE)

`terraform-provider-singlestoredb` is a Terraform provider for managing resources on SingleStoreDB Cloud. Use `singlestoredb_cluster` for deployments. `singlestoredb_workspace` and `singlestoredb_workspace_group` still work and are deprecated; see [Upgrading from workspace resources](#upgrading-from-workspace-resources).

## Prerequisites

To use this provider, ensure you have the following:

- [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli) 0.12 or later installed.
- A SingleStoreDB API key. You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.

## Provider Setup

To set up the provider, first export the generated API key to the `SINGLESTOREDB_API_KEY` environment variable:

```bash
export SINGLESTOREDB_API_KEY="paste your generated SingleStoreDB API key here"
```

SQL resources and data sources (`singlestoredb_sql_execute`, `singlestoredb_sql_query`) use the database user password. When the `password` attribute is unset, the provider reads from `SINGLESTORE_SQL_USER_PASSWORD` at apply/read/destroy time. This is independent of the Management API key.

Then, to specify the SingleStoreDB provider for use in your Terraform configuration, you will need to add a `required_providers` block. The easiest way to get the correct `required_providers` block is to visit the [SingleStoreDB provider page on the Terraform Registry](https://registry.terraform.io/providers/singlestore-labs/singlestoredb/latest). Click the "USE PROVIDER" button to see and copy the `required_providers` block with the latest version of the provider. Paste this block into your Terraform configuration file.

Here is a general template of how the `required_providers` block and provider block might look in your Terraform configuration:

```hcl
terraform {
  required_providers {
    singlestoredb = {
      source = "singlestore-labs/singlestoredb"
      # Visit the link above to get the latest version.
    }
  }
}

provider "singlestoredb" {
  # Configuration options.
}
```

The `required_providers` block specifies that your configuration will be using the SingleStoreDB provider, and the `provider` block is where you can specify any necessary configuration options for the provider.

## Example Usage

The provider offers a variety of data sources and resources for managing SingleStoreDB. Here's a sample usage that demonstrates creating and managing a cluster:

```hcl
provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

resource "singlestoredb_cluster" "this" {
  name            = "cluster-1"
  project_name    = "Standard Project"
  size            = "S-00"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  cloud_provider  = "AWS"
  region_name     = "us-east-1"
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

To try this example, follow these steps:

1. **Create the cluster:**

   ```shell
   terraform apply
   ```

2. **Connect to the new cluster:**

   ```shell
   export endpoint=$(terraform output -raw endpoint)
   export admin_password=$(terraform output -raw admin_password)
   mysql -u admin -h $endpoint -P 3306 --default-auth=mysql_native_password --password=$admin_password -e 'select 1'
   ```

3. **Terminate the cluster:**

   ```shell
   terraform destroy
   ```

### Cluster admin password

If you omit `admin_password`, the Management API generates a password and Terraform stores it in `admin_password`. If you set `admin_password`, it must have at least 14 characters, an uppercase letter, a lowercase letter, a digit, and a special character, at most 2 consecutive sequential characters (such as "ab" or "12"), and at most 3 identical characters in a row.

The provider checks these rules during `terraform plan` and fails with "Invalid admin password" when a password breaks them. If the API still does not accept the configured password, it generates a different password on create. The provider then deletes the new cluster and fails with "Admin password was rejected", so Terraform never stores a password that cannot log in. Both errors list the rules and the ones the password fails. Fix `admin_password`, or remove it, and apply again.

If the API password rules change and the plan check rejects a password the API accepts, set `skip_admin_password_validation = true` on the cluster until a provider release updates the rules. The create check still applies.

```terraform
resource "singlestoredb_cluster" "this" {
  # ...
  admin_password                 = var.admin_password
  skip_admin_password_validation = true
}
```

## Upgrading from workspace resources

`singlestoredb_cluster` is the resource for deployments. `singlestoredb_workspace` and `singlestoredb_workspace_group` keep their current behavior in this release. `terraform plan` prints a deprecation warning and does not change what those resources do.

The next minor release rejects a plan that still contains `singlestoredb_workspace` or `singlestoredb_workspace_group`, unless the provider block sets `allow_deprecated_workspace_resources = true`. That argument does not exist yet. Leave it unset until that release, then set it only to keep an existing configuration running for one more release:

```hcl
provider "singlestoredb" {
  allow_deprecated_workspace_resources = true
}
```

New configurations use `singlestoredb_cluster`. These resources are removed in the first release on or after April 30, 2027.

Move an existing deployment without creating a new one. The cluster `id` is the workspace id. A `moved` block cannot merge the two resources into one.

```shell
terraform state show singlestoredb_workspace.this
terraform state rm singlestoredb_workspace.this
terraform state rm singlestoredb_workspace_group.example
terraform import singlestoredb_cluster.this <workspace-id>
terraform plan
```

Copy the workspace group `admin_password` before `terraform state rm`. The API does not return it again. Set it as the cluster `admin_password`. Workspace groups required only 14 characters, so the copied password can fail the cluster plan check described in [Cluster admin password](#cluster-admin-password). In that case, set `skip_admin_password_validation = true` to keep the existing password, or set a new password that meets the rules. Set `name` to the name `/v2/clusters` returns, which is usually the workspace group name.

After the other attributes match the imported state, `terraform plan` shows a single in-place update of `admin_password`. Import cannot read the password, so state holds an empty value, and the apply sends the configured password to the cluster. Any other change means the configuration does not match the live deployment. A plan that replaces the cluster means an identity field does not match. Fix the configuration before you apply. Applying a replacement destroys the cluster and its data.

| Existing attribute | Cluster attribute |
| --- | --- |
| `singlestoredb_workspace.id` | `id` (import this value) |
| `singlestoredb_workspace_group.id` | `group_id` (computed) |
| `singlestoredb_workspace_group.name` | `name` (use the live API name) |
| `singlestoredb_workspace.size` | `size` |
| `singlestoredb_workspace.suspended` | `suspended` |
| `singlestoredb_workspace_group.admin_password` | `admin_password` |
| `singlestoredb_workspace_group.firewall_ranges` | `firewall_ranges` |
| `singlestoredb_workspace_group.expires_at` | `expires_at` |
| `singlestoredb_workspace_group.project_name` | `project_name` |
| `singlestoredb_workspace_group.cloud_provider` | `cloud_provider` |
| `singlestoredb_workspace_group.region_name` | `region_name` |

## Documentation

For more detailed information about `terraform-provider-singlestoredb`, including advanced usage and configuration options, check out our [official documentation](./docs/index.md).

## Contributing

Contributions from the community are welcomed and appreciated! See our [DEVELOPMENT.md](DEVELOPMENT.md) guide for information on how to get started.

**Note:** Integration tests may require manual approval before running. A maintainer must manually trigger the [integration workflow](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/integration.yml) before a PR from a fork can be merged. Additionally, a maintainer needs to run this workflow manually if you see the message "Missing successful active test deployment." on your PR.

## Code of Conduct

We strive to ensure a safe and positive environment for our community. Please review and adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

`terraform-provider-singlestoredb` is released under the Apache 2.0 license. For more information, see the [LICENSE](LICENSE) file.
