# Terraform Provider for SingleStoreDB Cloud

[![Unit](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/unit.yml/badge.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions)
[![Integration](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/integration.yml/badge.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/singlestore-labs/terraform-provider-singlestoredb)](https://goreportcard.com/report/github.com/singlestore-labs/terraform-provider-singlestoredb)
[![codecov](https://codecov.io/gh/singlestore-labs/terraform-provider-singlestoredb/branch/master/graph/badge.svg?token=BT65KGONQ6)](https://codecov.io/gh/singlestore-labs/terraform-provider-singlestoredb)
[![License](https://img.shields.io/github/license/singlestore-labs/terraform-provider-singlestoredb.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/blob/master/LICENSE)

`terraform-provider-singlestoredb` is a Terraform provider for managing resources on SingleStoreDB Cloud. Use `singlestoredb_cluster` for deployments. `singlestoredb_workspace` and `singlestoredb_workspace_group` still work in v0.2.0 and are removed in v1.0.0, the first release on or after April 30, 2027; see [Migrating from workspace resources](MIGRATING.md).

## Prerequisites

To use this provider, ensure you have the following:

- [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli) 0.12 or later installed.
- A SingleStoreDB API key. You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.

## Provider Setup

To set up the provider, first export the generated API key to the `SINGLESTOREDB_API_KEY` environment variable:

```bash
export SINGLESTOREDB_API_KEY="paste your generated SingleStoreDB API key here"
```

SQL resources and data sources (`singlestoredb_sql_execute`, `singlestoredb_sql_query`) use separate workspace credentials. When the `password` attribute is unset, the provider reads from `SINGLESTORE_SQL_USER_PASSWORD` at apply/read/destroy time. This is independent of the Management API key.

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

variable "firewall_ranges" {
  description = "CIDR ranges allowed to connect to the cluster."
  type        = list(string)
  default     = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
}

resource "singlestoredb_cluster" "this" {
  name            = "cluster-1"
  project_name    = "Standard Project"
  size            = "S-00"
  firewall_ranges = var.firewall_ranges
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

Management API generates the cluster admin password and ignores a password sent in the request. `admin_password` is read-only. The API returns the generated password only on create. Terraform stores that value, and refresh keeps it when later responses omit the password.

Read the password from state or from the sensitive output in the example above.

## Known limitations

`singlestoredb_cluster` follows `/v2/clusters`, so the API limitations are carried over to the provider. These may be relaxed in the future releases.

- `expires_at` is applied only on create. `PATCH /v2/clusters` ignores `expiresAt`, so a later change is rejected. Import records the current expiration in state but does not add it to your configuration. When the imported cluster has an expiration, copy that timestamp from state (for example, from `terraform state show singlestoredb_cluster.this`) into `expires_at`. When it has no expiration, leave `expires_at` unset. The plan fails if the configuration sets a different time or removes an existing expiration.
- `update_window` is read-only. `POST` and `PATCH /v2/clusters` ignore `updateWindow`. Terraform reports the window the API assigned. Configuring it fails the plan.

## Upgrading from workspace resources

`singlestoredb_workspace` and `singlestoredb_workspace_group` are deprecated, as are the data sources `singlestoredb_workspace`, `singlestoredb_workspaces`, `singlestoredb_workspace_group`, and `singlestoredb_workspace_groups`. They keep their current behavior in v0.2.0, and `terraform plan` prints a warning. They are removed in v1.0.0, the first release on or after April 30, 2027.

Move an existing deployment onto `singlestoredb_cluster` without creating a new one. The cluster `id` is the workspace id. The state steps, attribute mapping, password handling, and resources that reference a workspace are in [Migrating from workspace resources](MIGRATING.md).

## Documentation

For more detailed information about `terraform-provider-singlestoredb`, including advanced usage and configuration options, check out our [official documentation](./docs/index.md).

To move an existing workspace and workspace group onto `singlestoredb_cluster`, see [Migrating from workspace resources](MIGRATING.md).

## Contributing

Contributions from the community are welcomed and appreciated! See our [DEVELOPMENT.md](DEVELOPMENT.md) guide for information on how to get started.

**Note:** Integration tests may require manual approval before running. A maintainer must manually trigger the [integration workflow](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/integration.yml) before a PR from a fork can be merged. Additionally, a maintainer needs to run this workflow manually if you see the message "Missing successful active test deployment." on your PR.

## Code of Conduct

We strive to ensure a safe and positive environment for our community. Please review and adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

`terraform-provider-singlestoredb` is released under the Apache 2.0 license. For more information, see the [LICENSE](LICENSE) file.
