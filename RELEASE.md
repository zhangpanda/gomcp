# Release Rules

## ⛔ Never force-push tags

Once a tag is pushed, it is immutable. Go module proxy caches the content
and checksum — overwriting a tag breaks existing users.

## Version Bumping

Follow [Semantic Versioning](https://semver.org):

```
v{major}.{minor}.{patch}
```

- **patch** (v1.0.1): bug fixes, no API changes
- **minor** (v1.1.0): new features, backward compatible
- **major** (v2.0.0): breaking API changes

## Release Checklist

```bash
# 1. Ensure all tests pass
go test -race ./... -count=1

# 2. Ensure vet is clean
go vet ./...

# 3. Tag (NEVER use -f)
git tag v1.x.x

# 4. Push
git push origin main --tags
git push github main --tags
```

## Current Version

- v1.0.0 — initial stable release
