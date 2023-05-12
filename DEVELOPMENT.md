# Developing `terraform-provider-singlestoredb`

This document provides information on how to develop `terraform-provider-singlestoredb`.

## Prerequisites

1. [Go 1.20](https://go.dev/doc/install) or later

2. [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli) 0.12 or later

3. [direnv](https://direnv.net/docs/installation)

## Installation

1. In a terminal, clone the `terraform-provider-singlestoredb` repository.

    ```shell
    git clone https://github.com/singlestore-labs/terraform-provider-singlestoredb
    ```

2. Navigate to the `terraform-provider-singlestoredb` directory and enable direnv.

    ```shell
    cd terraform-provider-singlestoredb
    direnv allow
    ~~~
    ```

3. Build and install the binary.
    ```shell
    make install
    ```

## Examples

Try out any example in the [examples](examples) directory!

Please note that `terraform init` is not compatible with `dev_overrides`, so run `terraform plan` directly.

## Reference

- [Configuring Terraform](https://learn.hashicorp.com/tutorials/terraform/providers-plugin-framework/providers-plugin-framework-provider#locally-install-provider-and-verify-with-terraform)

- [Terraform Init with Dev Overrides](https://github.com/hashicorp/terraform/issues/27459)