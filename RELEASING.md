# Releasing the Terraform Provider

Releases are triggered automatically when a new Git tag is pushed.

---

## ✅ Steps to Release

### 1. Review Changes

List version tags (newest first):

```bash
git fetch --tags --prune
git tag --sort=-v:refname
```

Show commits since the last release (example for v0.3.2):

```bash
git log v0.3.2..HEAD --oneline
```

### 2. Tag and Push

Create and push a new version tag, which is typically the last patch version plus one:

```bash
git tag v0.3.3
git push origin v0.3.3
```

Use semantic versioning (vX.Y.Z) — this triggers the release workflow.

### 3. Done

GitHub Actions will:
- Build the provider
- Create a release
- Publish to the Terraform Registry

To verify that the new version is successfully published, visit the [Terraform Registry](https://registry.terraform.io/providers/singlestore-labs/singlestoredb/latest). It typically takes a few minutes for the registry to acknowledge the new release.
