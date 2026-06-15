# Developing `terraform-provider-singlestoredb`

This document provides information on how to develop `terraform-provider-singlestoredb`.

## Prerequisites

1. [Go 1.20](https://go.dev/doc/install) or later

2. [Terraform](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli) 0.12 or later

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

4. Install git hooks for automated pre-commit checks.
    ```shell
    make install-hooks
    ```

## Examples

Try out any example in the [examples](examples) directory!

Please note that `terraform init` is not compatible with `dev_overrides`, so run `terraform plan` directly.

## Debugging

The provider supports interactive debugging with [Delve](https://github.com/go-delve/delve) via VS Code or Cursor. The `main` function accepts a `-debug` flag that enables the Terraform reattach protocol.

Install the [Go extension](https://marketplace.visualstudio.com/items?itemName=golang.go), then create a local `.vscode/` directory (gitignored) with the files below.

### `.vscode/extensions.json`

```json
{
  "recommendations": ["golang.go"]
}
```

### `.vscode/settings.json`

Overrides any global `go.testEnvFile` setting so debug sessions load credentials from this repo instead of a missing file on your machine.

```json
{
  "go.testEnvFile": "${workspaceFolder}/.vscode/private.env"
}
```

### `.vscode/private.env`

Copy from the example below and set your API key. This file is gitignored.

```shell
cp .vscode/private.env.example .vscode/private.env
```

`.vscode/private.env.example`:

```
# Copy to private.env for the "Debug Terraform Provider" launch config.
# SINGLESTOREDB_API_KEY=your-api-key
```

### `.vscode/launch.json`

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug Terraform Provider",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}",
      "args": ["-debug"],
      "showLog": true,
      "envFile": "${workspaceFolder}/.vscode/private.env"
    },
    {
      "name": "Debug Unit Tests (short)",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${workspaceFolder}",
      "args": ["-test.v", "-test.short"]
    },
    {
      "name": "Debug Current Package Tests",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${fileDirname}",
      "args": ["-test.v", "-test.short"]
    }
  ]
}
```

### Debug with Terraform

1. Set breakpoints in provider code (for example under `internal/provider/`).
2. Run **Debug Terraform Provider** from the Run and Debug panel.
3. Copy the `TF_REATTACH_PROVIDERS=...` line from the Debug Console.
4. In a terminal, export it and run Terraform against an example:

    ```shell
    export TF_REATTACH_PROVIDERS='{"registry.terraform.io/singlestore-labs/singlestoredb":{...}}'
    cd examples/resources/singlestoredb_workspace_group_advanced
    terraform plan
    ```

Terraform routes provider calls to the debugger; breakpoints will halt execution.

### Debug unit tests

Open a `*_test.go` file, set breakpoints, and run **Debug Current Package Tests** or **Debug Unit Tests (short)**. No Terraform step is required.

## Reference

- [Configuring Terraform](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider#locally-install-provider-and-verify-with-terraform)

- [Terraform Init with Dev Overrides](https://developer.hashicorp.com/terraform/tutorials/providers-plugin-framework/providers-plugin-framework-provider#prepare-terraform-for-local-provider-install)
