# Terraform Provider for SingleStoreDB Cloud

[![Unit](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/unit.yml/badge.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions)
[![Integration](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions/workflows/integration.yml/badge.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/singlestore-labs/terraform-provider-singlestoredb)](https://goreportcard.com/report/github.com/singlestore-labs/terraform-provider-singlestoredb)
[![codecov](https://codecov.io/gh/singlestore-labs/terraform-provider-singlestoredb/branch/master/graph/badge.svg?token=BT65KGONQ6)](https://codecov.io/gh/singlestore-labs/terraform-provider-singlestoredb)
[![License](https://img.shields.io/github/license/singlestore-labs/terraform-provider-singlestoredb.svg)](https://github.com/singlestore-labs/terraform-provider-singlestoredb/blob/master/LICENSE)

`terraform-provider-singlestoredb` is a Terraform provider for managing resources on SingleStoreDB Cloud. This provider enables you to manage resources such as Workspace Groups and Workspaces seamlessly with your Terraform workflow.

## Prerequisites

To use this provider, ensure you have the following:

- [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli) 0.12 or later installed.
- A SingleStoreDB API key. You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.

## Provider Setup

To set up the provider, first export the generated API key to the `SINGLESTOREDB_API_KEY` environment variable:

```bash
export SINGLESTOREDB_API_KEY="paste your generated SingleStoreDB API key here"
```

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

The provider offers a variety of data sources and resources for managing SingleStoreDB. Here's a sample usage that demonstrates creating and managing a workspace:

```hcl
provider "singlestoredb" {
  // The SingleStoreDB Terraform provider uses the SINGLESTOREDB_API_KEY environment variable for authentication.
  // Please set this environment variable with your SingleStore Management API key.
  // You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.
}

data "singlestoredb_regions" "all" {}

resource "singlestoredb_workspace_group" "example" {
  name            = "group"
  firewall_ranges = ["0.0.0.0/0"] // Ensure restrictive ranges for production environments.
  expires_at      = "2222-01-01T00:00:00Z"
  region_id       = data.singlestoredb_regions.all.regions.0.id // Prefer specifying the explicit region ID in production environments as the list of regions may vary.
}

resource "singlestoredb_workspace" "this" {
  name               = "workspace"
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

To try this example, follow these steps:

1. **Create the workspace:**

    ```shell
    terraform apply
    ```

2. **Connect to the new workspace:**

    ```shell
    export endpoint=$(terraform output -raw endpoint)
    export admin_password=$(terraform output -raw admin_password)
    mysql -u admin -h $endpoint -P 3306 --default-auth=mysql_native_password --password=$admin_password -e 'select 1'
    ```

3. **Terminate the workspace:**

    ```shell
    terraform destroy
    ```

## Documentation

For more detailed information about `terraform-provider-singlestoredb`, including advanced usage and configuration options, check out our [official documentation](./docs/index.md).

## Contributing

Contributions from the community are welcomed and appreciated! See our [DEVELOPMENT.md](DEVELOPMENT.md) guide for information on how to get started.

## Code of Conduct

We strive to ensure a safe and positive environment for our community. Please review and adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

`terraform-provider-singlestoredb` is released under the Apache 2.0 license. For more information, see the [LICENSE](LICENSE) file.
