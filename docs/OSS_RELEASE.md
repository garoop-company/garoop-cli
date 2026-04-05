# OSS release flow

## 1. Publish source

1. Make the GitHub repository public.
2. Confirm `LICENSE`, `README.md`, and `.gitignore` are present.
3. Verify no secrets are tracked.

## 2. `go install`

These commands work after the repository is public and tagged:

```bash
go install github.com/garoop-company/garoop-cli/cmd/garoop-cli@latest
go install github.com/garoop-company/garoop-cli/cmd/garuchan-cli@latest
go install github.com/garoop-company/garoop-cli/cmd/garooptv-cli@latest
```

## 3. GitHub Releases

1. Commit and push to `main`.
2. Create a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

3. Create a GitHub Release from that tag.
4. If using GoReleaser locally:

```bash
make release-check
goreleaser release --clean
```

For a dry run:

```bash
make snapshot
```

## 4. Homebrew

`.goreleaser.yaml` is already configured to publish formulas to:

- `garoop-company/homebrew-tap`

Required environment variable:

```bash
export HOMEBREW_TAP_GITHUB_TOKEN=...
```

After a tagged release with GoReleaser, users can install with:

```bash
brew tap garoop-company/homebrew-tap
brew install garoop-cli
brew install garuchan-cli
brew install garooptv-cli
```

GitHub Actions workflow:

- `.github/workflows/release.yml` runs on `v*` tags
- It runs `go test ./...`
- Then it executes `goreleaser release --clean`
- `HOMEBREW_TAP_GITHUB_TOKEN` must be set in repository secrets

Recommended secret setup:

```bash
HOMEBREW_TAP_GITHUB_TOKEN=<a GitHub token that can push to garoop-company/homebrew-tap>
```

Release flow:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This publishes GitHub Release artifacts and updates the Homebrew tap formulas.

## 5. Quick checklist

```bash
git status
git grep -n "API_KEY\\|SECRET\\|TOKEN\\|COOKIE\\|PASSWORD"
go test ./...
make build
```
