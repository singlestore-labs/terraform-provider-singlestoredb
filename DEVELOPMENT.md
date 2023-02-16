# Developing `terraform-provider-singlestore`

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

4. Override the `~/.terraformrc`
    ~~~ tf
    provider_installation {

    dev_overrides {
            "registry.terraform.io/singlestoredb/singlestore" = "<PATH>/go/bin"
    }

    # For all other providers, install them directly from their origin provider
    # registries as normal. If you omit this, Terraform will _only_ use
    # the dev_overrides block, and so no other providers will be available.
    direct {}
    }
    ~~~

**Note: `terraform init` is not compatible with `dev_overrides`, run `terraform plan` directly.**

## Reference

- [configuring terraform](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider#locally-install-provider-and-verify-with-terraform)
- [terraform init with dev overrides](https://github.com/hashicorp/terraform/issues/27459)