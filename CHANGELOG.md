# Changelog

## Unreleased

### Added

- New `singlestoredb_cluster` resource and `singlestoredb_cluster` / `singlestoredb_clusters` data sources for the Management API `/v2/clusters` endpoint. A cluster creates a workspace and its workspace group in a single API call.
- `cluster_id` attribute on `singlestoredb_private_connection` (preferred) and `singlestoredb_flow` for the Management API v2 cluster terminology.

### Changed

- Bump `github.com/singlestore-labs/singlestore-go/management` from v1.2.158 to v1.2.176. The Management API client now targets v2 endpoints; `/v1/workspaces` and `/v1/workspaceGroups` are replaced by `/v2/clusters`.
- `singlestoredb_workspace_group` and `singlestoredb_workspace` remain first-class resources with the same Terraform UX; they now call `/v2/clusters` under the hood because the SDK no longer exposes `/v1/workspaces` and `/v1/workspaceGroups`. Workspace group create provisions a starter workspace in the group; `project_name` is effectively required because the API requires a project ID. `name` and `update_window` cannot be updated after create (not supported by `/v2/clusters` PATCH).
- Because `/v2/clusters` ignores `GroupID` on create, the first `singlestoredb_workspace` in a group adopts the group's starter cluster (rename/resize) instead of creating a second unreachable cluster. Additional workspaces in the same group still POST a new cluster and do not share the group's admin password.
- Workspace create copies sibling firewall allowlists via `allowAllTraffic` → `0.0.0.0/0` so unrestricted groups are not recreated as deny-all.
- Examples omit configured `admin_password` so `/v2/clusters` can generate one; Terraform state then holds the working password (configured sensitive values cannot diverge from plan after apply).
- Role grants with `resource_type = "Cluster"` continue to accept `singlestoredb_workspace_group.id`; identity-roles responses that use `ClusterGroup` are normalized back to `Cluster`.
- `singlestoredb_regions` now returns region code names (`region_name`) via `/v2/regions` instead of region UUIDs (`id` nested attribute removed). Prefer `singlestoredb_regions_v2` / `cloud_provider` + `region_name` for new configurations.
- Existing Management API resources and data sources (projects, teams, users, invitations, flow, private connections, organization access controls, secrets) call the corresponding `/v2/...` endpoints.

### Breaking

- `singlestoredb_flow`: replace `workspace_id` with `cluster_id` (same UUID value; cluster is the Management API v2 name for a workspace).
- `singlestoredb_private_connection`: prefer `cluster_id`; `workspace_id` and `workspace_group_id` remain as deprecated aliases.
- `singlestoredb_regions` nested `id` (region UUID) is no longer available; use `region_name` / `provider` instead.

## v0.1.19 - 2026-07-31

### Fixed

- Fixed "Provider produced inconsistent result after apply" errors on `singlestoredb_workspace_group.firewall_ranges` when the Management API returns ranges in a different order, reports unrestricted access via `allowAllTraffic`, or has not yet applied a firewall update. Updates now wait for the reported allowlist to converge, and Terraform state keeps the configured order when the API reports an equivalent set (#130).

### Added

- New `singlestoredb_sql_execute` resource for DDL/DML against Helios workspaces via the Data API, with optional read-back for drift detection (#118).
- New `singlestoredb_sql_query` data source for read-only SQL queries at plan/apply time (#118).
- `SINGLESTORE_SQL_USER_PASSWORD` environment variable for SQL user password / JWT fallback on SQL resources and data sources (#118).

### Changed

- Rewrote `singlestoredb_workspace_with_sql` example to use `singlestoredb_sql_execute` resources instead of `null_resource` and the `mysql` CLI (#118).

### Dependencies

- Bump `google.golang.org/grpc` from 1.79.3 to 1.82.1 (#129).
- Bump `golang.org/x/crypto` from 0.51.0 to 0.52.0 (#127).
- Bump `golang.org/x/net` from 0.48.0 to 0.55.0 (#123).

## v0.1.18 - 2026-07-02

### Fixed

- Fixed a panic (nil pointer dereference) when updating a `singlestoredb_private_connection` and the Management API returned a private connection with a null `allow_list` while waiting for the update to converge (#121).

## v0.1.17 - 2026-07-01

### Fixed

- Fixed a bug where Flow instances would report erroneous data in the plan (#112).
- Fixed a bug where Flow instances would report as ready when they were not (#112).

### Changed

- Updated 403 response to mention credits where applicable (#117).

## v0.1.16 - 2026-05-18

### Fixed

- Fixed a bug where workspace group update resulted in empty password being sent in a patch request, so users were getting the error "password must contain at least 14 characters" (#108).

## v0.1.15 - 2026-04-28

### Added

- New `singlestoredb_project` resource with generated documentation (#100, #102).
- New `singlestoredb_roles` resource and data source (#98).

### Changed

- `singlestoredb_team`: `member_users` and `member_teams` are now sets of strings instead of lists. This removes order-sensitive plan diffs when the backend returns members in a different order than configured. Existing state from prior provider versions is read transparently; no manual migration is required (#101).

### Build

- Upgrade Go version to 1.25 (#103).

### Dependencies

- Bump `google.golang.org/grpc` from 1.67.1 to 1.79.3 (#95).
- Bump `github.com/cloudflare/circl` from 1.6.1 to 1.6.3 (#88).

## v0.1.14 - 2026-03-31

### Added

- Allow assigning a project to a cluster (#94).

### Changed

- Pin releaser version (#99).

## v0.1.11 - 2026-03-23

### Added

- Autoscale on workspace creation.

### Fixed

- `singlestoredb_flow`: add validation for `user_name` and `database_name` fields (#97).

### Tests

- Fix `testGrantRevokeUserRole(s)Integration` (#87).

## v0.1.10 - 2026-03-06

### Added

- `update_window` field on `singlestoredb_workspace_group` for create/update (#86).

### Fixed

- Miscellaneous fixes for parsing and documentation (#93).

### Tests

- Use unique team names in tests (#81).

## v0.1.9 - 2026-02-16

### Added

- Support for Flow instances (#84).
- `make format` command (#85).

### Dependencies

- Bump `singlestore-go` to 1.2.144 (#79).
- Bump `github.com/cloudflare/circl` from 1.3.7 to 1.6.1 (#82).
- Bump `golang.org/x/net` from 0.28.0 to 0.38.0 (#59).

## v0.1.8 - 2025-12-12

### Changed

- Clean up resources on not-found responses (#78).

## v0.1.7 - 2025-12-04

### Added

- Configurable client timeout (#77).

## v0.1.6 - 2025-11-03

### Added

- Look up workspaces by name (#76).
- Look up workspace groups by name (#75).

## v0.1.5 - 2025-10-15

### Added

- Documentation for importing private connections (#73).
- Documentation for importing users and teams (#74).
- SQL documentation (#72).

## v0.1.4 - 2025-08-27

### Fixed

- Azure region case mismatch (#71).

## v0.1.3 - 2025-08-08

### Added

- Import documentation (#70).

## v0.1.2 - 2025-08-01

### Added

- Look up teams by name (#69).
- Release and GPG documentation (#68).

## v0.1.1 - 2025-07-08

Re-tag of `v0.1.0` — no code changes.
