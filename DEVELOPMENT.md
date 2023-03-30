# Developing `terraform-provider-singlestore`

## Prerequisites

1. [go 1.20](https://go.dev/doc/install)

2. [terraform](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli)

## Installation

1. In a terminal clone the `terraform-provider-singlestore` repository

    ~~~ shell
    git clone https://github.com/singlestore-labs/terraform-provider-singlestore
    ~~~

2. Navigate to the `terraform-provider-singlestore` directory

    ~~~ shell
    cd terraform-proivder-singlestore
    ~~~

3. Build and install the binary
    ~~~ shell
    make install
    ~~~

4. Override the `~/.terraformrc` just as in [here](./.terraformrc) but with the `$HOME` variable replaced

## Examples

Try out any example in [examples](examples), e.g., the [workspace resource example](examples/workspaces/resource)

**Note: `terraform init` is not compatible with `dev_overrides`, run `terraform plan` directly.**

## Reference

- [configuring terraform](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider#locally-install-provider-and-verify-with-terraform)
- [terraform init with dev overrides](https://github.com/hashicorp/terraform/issues/27459)