# AGENTS.md

## Cursor Cloud specific instructions

This repository is a **Terraform provider** (Go plugin), not a web app. There are no Docker services or local databases to start. Development centers on building the provider binary and exercising it via Go tests or the Terraform CLI.

### Prerequisites (one-time VM setup)

- **Go 1.25+** — required by `go.mod`; the repo uses the Go toolchain directive.
- **Terraform CLI** — install from HashiCorp apt (`terraform` package) or [official install docs](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli).
- **`make`** — used for all common tasks (see `Makefile` and `DEVELOPMENT.md`).

### Terraform dev overrides

Local development uses Terraform `dev_overrides` so `terraform plan` loads the locally built provider from `~/go/bin`. Configure once per session:

```bash
export HOME="${HOME:-/home/ubuntu}"
sed "s|\$HOME|${HOME}|g" .terraformrc_template > ~/.terraformrc
export TF_CLI_CONFIG_FILE="$HOME/.terraformrc"
export PATH="$HOME/go/bin:$PATH"
```

`direnv` (`.envrc`) does the same via `envsubst`, but `gettext`/`envsubst` may not be installed in all environments — `sed` works as a drop-in.

**Note:** `terraform init` is incompatible with `dev_overrides`; run `terraform plan` directly in `examples/` directories (see `DEVELOPMENT.md`).

### Common commands

| Task | Command |
|------|---------|
| Install provider binary | `make install` |
| Unit tests (mocked API) | `GOTOOLCHAIN=go1.25.0 make unit` |
| Integration tests (live cloud) | `make integration` — requires `TEST_SINGLESTOREDB_API_KEY`; provisions real resources |
| Lint | `GOTOOLCHAIN=go1.25.0 go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.63.4` then `golangci-lint run --fast ./...` |
| Format | `make format` |
| Git hooks | `make install-hooks` |

### Gotchas

1. **`PATH` must include `~/go/bin`** — `go install` places `terraform-provider-singlestoredb` and dev tools there; `make lint` invokes `golangci-lint` by name.
2. **`GOTOOLCHAIN=go1.25.0`** — avoids `go: no such tool "covdata"` when `make unit` merges coverage across packages, and ensures `golangci-lint` is built with Go ≥ 1.25 (required by `go.mod`).
3. **`make tools` before lint** — if `golangci-lint` was built with an older Go, reinstall it with `GOTOOLCHAIN=go1.25.0 go install ...` before running lint (see `Makefile` `tools` target).
4. **Authentication** — runtime uses `SINGLESTOREDB_API_KEY`; integration tests use `TEST_SINGLESTOREDB_API_KEY`. A read-only hello-world check: `terraform plan` in `examples/data-sources/singlestoredb_regions/`.
5. **No long-running server** — the “application” is the provider plugin invoked by Terraform or the Go test harness; there is no `dev` server to keep running.
