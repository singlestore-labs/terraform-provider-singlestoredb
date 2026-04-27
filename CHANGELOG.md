# Changelog

## Unreleased

### Changed

- `singlestoredb_team`: `member_users` and `member_teams` are now sets of strings instead of lists. This removes order-sensitive plan diffs when the backend returns members in a different order than configured. Existing state from prior provider versions is read transparently; no manual migration is required.
