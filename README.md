# Terraform Provider for SingleStoreDB Cloud

`terraform-provider-singlestoredb` is a Terraform provider for managing resources on SingleStoreDB Cloud. This provider enables you to manage resources such as Workspace Groups and Workspaces seamlessly with your Terraform workflow.

> **Important:** This Terraform provider is currently unpublished on the Terraform Registry and must be run in a local environment. It is currently in the preview phase and is recommended for experimental use only.

## Prerequisites

To use this provider, ensure you have the following:

- [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli) 0.12 or later installed.
- A SingleStoreDB API key. You can generate this key from the SingleStore Portal at https://portal.singlestore.com/organizations/org-id/api-keys.

## Provider Setup

To set up the provider, export the generated API key to the `SINGLESTOREDB_API_KEY` environment variable:

```bash
export SINGLESTOREDB_API_KEY="paste your generated SingleStoreDB API key here"
```

## Example Usage

The provider offers a variety of data sources and resources for managing SingleStoreDB. Here's a sample usage that demonstrates creating and managing a workspace:

[Example](examples/resources/singlestoredb_workspace/resource.tf ':include type=code')

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

## Contributing

Contributions from the community are welcomed and appreciated! See our [DEVELOPMENT.md](DEVELOPMENT.md) guide for information on how to get started.

## Code of Conduct

We strive to ensure a safe and positive environment for our community. Please review and adhere to our [Code of Conduct](CODE_OF_CONDUCT.md).

## License

`terraform-provider-singlestoredb` is released under the Apache 2.0 license. For more information, see the [LICENSE](LICENSE) file.