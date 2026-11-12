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

1. **Probe the Management API first.** Before writing code, call the live API by hand (create, read, update each field, suspend/resume, delete, delete while still provisioning) and write down what it returns. Record quirks such as fields left out of responses, states that mean more than one thing, or POST and PATCH behaving differently. Use `-short` with `go test` unless you mean to provision real resources.
2. **Write a one-page spec and get it reviewed before coding** (template below). Settle attribute types and Optional/Computed/RequiresReplace in the spec, not in follow-up commits.
3. **Follow an existing resource.** Use `internal/provider/workspacegroups/resource.go` (~800 lines) as the reference for structure and size. Keep comments short: say why, not what. Split a resource before opening the PR once `resource.go` grows past that. The cluster resource shipped with `resource.go` at ~1600 lines plus `update.go` and `wait.go`; the human review was "it's impossible to review all of this," and the bugs found after that approval came from bots.
4. **Split the work into PRs.** Shared refactors (moving helpers into `internal/provider/util`) go in their own PR first. CI, coverage, and workflow changes never go in a feature PR. A Codecov patch-coverage comment on a new resource is not a reason to edit `.github/workflows` or add tests that only call a helper. The cluster PR added both, reverted the workflow the same day, and spent a review round on it.
5. **Review locally before pushing.** Check the diff against the checklist below in a single pass, fix every finding, then push. PR review bots should confirm the work, not find the bugs one round at a time.
6. **Commit in reviewable steps** (schema → create/read/delete → update → wait/polling → docs). Don't commit one large drop followed by "cleanup" commits after review has started. Commit subjects name the checklist item they close. "cleanup", "nits", and "leftover" hide the change from the next reviewer.
7. **Fix the class of bug, then push once.** A review comment names one line. The same mistake is usually in the next function. On the cluster resource, keeping `admin_password` from prior state did not stop an unknown `firewall_ranges` list from being PATCHed as `""`; stopping on `TERMINATED` did not stop 401/403 retries; requiring a stable state did not stop the poll window from treating a mix of `ACTIVE` and `SUSPENDED` as stable. Search for the pattern, fix every site, and add a unit test that would have failed for the sibling. Then stop. CodeRabbit pauses after a stream of commits (`reviews.auto_review.auto_pause_after_reviewed_commits`) and did not review the last four commits of that PR, which contained those fixes. Batch the fixes and ask for one `@coderabbitai review` on that commit.

### One-page spec template

```markdown
# Resource: singlestoredb_<name>

## API
- Endpoints: POST / GET / PATCH / DELETE paths used.
- Quirks found while probing (fields left out of responses, ambiguous states, POST vs PATCH differences).

## Schema
| Attribute | Type | Required/Optional/Computed | Default | RequiresReplace | Sensitive | Unknown at plan? | Notes |
|-----------|------|----------------------------|---------|-----------------|-----------|------------------|-------|

## Create
- Request body; fields deferred to a follow-up PATCH and why.
- What happens when that follow-up fails (apply error, or state keeps the plan so the next apply retries).
- What the wait waits for.

## Update
- Order of operations (e.g. resume → patch → resize → suspend) and which changes cannot be combined.
- Values kept from prior state when the API leaves them out (e.g. generated passwords).

## Wait / polling
| Situation | Behavior |
|-----------|----------|
| Target state reached | Require N consecutive readings in the *same* state |
| FAILED / TERMINATED / terminatedAt set | Stop, return error with remediation |
| 404 | Tolerate for a time-based grace, then stop |
| 401 / 403 | Stop immediately |
| 5xx, network error, non-JSON 200 | Retry until timeout |
| Timeout / context canceled | Error that includes the last observed state |

## Read / drift
- When to remove from state (deleted, terminated).
- 404 grace on Read, and how it matches the post-create wait.
- Fields refreshed from the API vs. kept from config.

## Import
- ID format; attributes that cannot be imported.

## Tests
- Unit (mocked API) cases, one per checklist item below that applies.
- Integration scenarios.
```

### Predictable Terraform bugs checklist

Check every item against the diff before pushing.

**Unknown values at plan time**
- [ ] Every attribute in `ModifyPlan` / `ValidateConfig` handles `IsUnknown()` (e.g. a value from another resource's output). Unknown is not "changed" and not "empty".
- [ ] Whole collections can be unknown, not just their elements. Native Go slices (`[]types.String`) cannot hold an unknown list; check before decoding or use `types.List`.
- [ ] Conflict checks ("cannot resize and suspend together") compare only *known* changes, and run again at apply time when the values are known.
- [ ] Unknown values are never sent to the API in a PATCH. A guard on the noop path still has to be on the body builder and the wait conditions.
- [ ] Known-invalid configuration fails in `ModifyPlan` (e.g. `suspended = true` on create, a size the API rejects). `Create` repeats the check for values that were unknown at plan time.

**Computed values and state consistency**
- [ ] Computed-only values (generated passwords, IDs, timestamps) are unknown in the plan; take them from **prior state**, not the plan, when the API response leaves them out.
- [ ] After apply, every known planned value equals the stored value exactly. Normalize API echoes, e.g. timestamps (equivalent RFC 3339 forms), defaults the API drops (sensitivity at scale factor 1), and case/format differences.
- [ ] Optional+Computed attributes with defaults don't cause a perpetual diff when the API returns something else.
- [ ] Attribute types match the API (integer seconds → `Int64`, not `Float`). Settle the type in the spec; changing it after review rewrites state mapping, docs, and tests.
- [ ] A follow-up PATCH that fails does not get stored as success. Keep the planned value, or return the error, so the next apply retries. Deriving that attribute from the API hides the miss (autoscale applied after create).
- [ ] Generated credentials are `Sensitive`, copied from prior state when the API omits them, and the docs say import cannot recover them.

**Read / refresh / drift**
- [ ] Resources deleted or terminated outside Terraform are removed from state, including terminal states signalled by a field (e.g. `terminatedAt`) rather than `state`.
- [ ] Read uses the same 404 grace as the post-create wait. Create stores the id before the API shows the cluster; the next refresh must not drop state on the first 404, or the deployment is left running with no resource.
- [ ] Values that can change outside Terraform (e.g. a project name looked up by ID) are refreshed on every read, not cached from the first read.
- [ ] Import fills in every attribute that Read can supply; anything that can't be imported is documented. Import instructions cover each case (copy `expires_at` only when the API returned one).

**Wait / polling**
- [ ] Terminal states (FAILED, TERMINATED, terminatedAt) stop the wait with a clear remediation message.
- [ ] 401/403 stop immediately; 404 uses a time-based grace (poll intervals back off, so a count of 404s is not a fixed duration); other errors retry.
- [ ] A 200 without a JSON body is handled (retried), not dereferenced.
- [ ] A "stable" result needs consecutive readings in the **same** state, not a mix of acceptable states.
- [ ] Every condition the update changed is waited on, so stored state reflects the new values.
- [ ] Poll requests use a context bounded by the wait timeout; timeout and cancel errors include the last observed state.
- [ ] Settings that may change on their own (e.g. auto-suspend) don't make an update wait fail.
- [ ] The wait decision table is a unit test. A wrong condition runs until the timeout (cluster create is 5 hours, update is 6 hours, delete is 1 hour). Integration tests confirm the API response, not which status ends the wait.

**Hygiene**
- [ ] No local artifacts committed (`*.local/`, generated `.tf`, coverage files, binaries).
- [ ] `docs/` regenerated with `TF_CLI_CONFIG_FILE=/dev/null make generate` (with `dev_overrides` active, tfplugindocs leaves new resources out), `CHANGELOG.md` and `README.md` updated. Docs link only to files this change adds.
- [ ] Unit tests cover behavior and the checklist edge cases, not coverage padding.
