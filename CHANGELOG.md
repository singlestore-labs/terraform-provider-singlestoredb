# Changelog

## Unreleased

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
