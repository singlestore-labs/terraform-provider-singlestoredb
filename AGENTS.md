# AGENTS.md

## Cursor Cloud specific instructions

This repository is a **Terraform provider** (Go plugin), not a web app. There are no Docker services or local databases to start. Development centers on building the provider binary and exercising it via Go tests or the Terraform CLI.

### Prerequisites (one-time VM setup)

- **Go 1.26+** — required by `go.mod`; the repo uses the Go toolchain directive.
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
| Unit tests (mocked API) | `GOTOOLCHAIN=go1.26.11 make unit` |
| Integration tests (live cloud) | `make integration` — requires `TEST_SINGLESTOREDB_API_KEY`; provisions real resources |
| Lint | `GOTOOLCHAIN=go1.26.11 go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.63.4` then `golangci-lint run --fast ./...` |
| Format | `make format` |
| Git hooks | `make install-hooks` |

### Gotchas

1. **`PATH` must include `~/go/bin`** — `go install` places `terraform-provider-singlestoredb` and dev tools there; `make lint` invokes `golangci-lint` by name.
2. **`GOTOOLCHAIN=go1.26.11`** — avoids `go: no such tool "covdata"` when `make unit` merges coverage across packages, and ensures `golangci-lint` is built with Go ≥ 1.26 (required by `go.mod`).
3. **`make tools` before lint** — if `golangci-lint` was built with an older Go, reinstall it with `GOTOOLCHAIN=go1.26.11 go install ...` before running lint (see `Makefile` `tools` target).
4. **Authentication** — runtime uses `SINGLESTOREDB_API_KEY`; integration tests use `TEST_SINGLESTOREDB_API_KEY`. A read-only hello-world check: `terraform plan` in `examples/data-sources/singlestoredb_regions/`.
5. **No long-running server** — the “application” is the provider plugin invoked by Terraform or the Go test harness; there is no `dev` server to keep running.

## Adding or changing a resource

### Workflow

1. **Probe the Management API first.** Exercise create, read, each update, and delete (including delete while provisioning) by hand. Note quirks: omitted fields, ambiguous states, POST vs PATCH differences.
2. **Write a short spec and get it reviewed before coding.** Cover endpoints and quirks; schema (type, Required/Optional/Computed, default, RequiresReplace, Sensitive, unknown at plan); create/update order of operations; wait behavior; read/drift; import; tests. Settle schema decisions there, not in follow-up commits.
3. **Follow an existing resource.** `internal/provider/workspacegroups/resource.go` (~800 lines) is the reference for structure and size; split anything larger before opening the PR. Comments say why, not what.
4. **Keep PRs focused.** Shared refactors go in their own PR first. CI, coverage, and workflow changes never go in a feature PR.
5. **Review locally against the checklist below, then push.** When review finds a bug, search for the same pattern elsewhere, fix every site, add a test, and push the fixes together.
6. **Commit in reviewable steps** (schema → create/read/delete → update → wait → docs). Subjects say what changed, not "cleanup" or "nits".

### Checklist

**Plan time**
- Any attribute can be unknown, including whole collections. Unknown is neither "changed" nor "empty": handle it in `ModifyPlan`/`ValidateConfig`, never send it to the API, and re-check at apply time anything that was unknown at plan.
- Known-invalid configuration fails in `ModifyPlan`, not at apply.

**State consistency**
- After apply, every known planned value equals the stored value. Normalize API echoes (timestamp formats, dropped defaults, casing).
- Computed values the API omits (e.g. generated passwords) come from prior state. Generated credentials are `Sensitive`; docs note import cannot recover them.
- A failed follow-up call is never stored as success: return an error or keep the planned value so the next apply retries.
- Attribute types match the API (integer seconds → `Int64`).

**Read / drift / import**
- Resources deleted or terminated outside Terraform are removed from state, including terminal states signalled by fields other than `state`.
- Read tolerates 404 for the same grace period as the post-create wait.
- Values that can change outside Terraform are refreshed on every read.
- Import fills everything Read can; anything else is documented.

**Waiting**
- Terminal states stop with a remediation message; 401/403 stop immediately; 404 gets a time-based grace; other errors (5xx, network, non-JSON 200) retry.
- "Stable" means consecutive readings in the same state.
- Wait on every condition the operation changed; ignore settings that change on their own.
- Polls are bounded by the timeout; timeout errors include the last observed state.
- The wait decision table is unit-tested.

**Hygiene**
- No local artifacts committed.
- Regenerate docs with `TF_CLI_CONFIG_FILE=/dev/null make generate`; update `CHANGELOG.md` and `README.md`.
- Tests cover behavior and edge cases, not coverage padding.
