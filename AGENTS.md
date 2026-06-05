# AGENTS.md

## Cursor Cloud specific instructions

This repository is the **Mattermost Microsoft Teams Plugin** — a Go server plugin with a React webapp bundle. It is not a standalone app; runtime requires a Mattermost Server (≥ 10.7.0) with PostgreSQL. See `README.md` for build/deploy details.

### Prerequisites

| Tool | Version | Notes |
|------|---------|-------|
| Go | 1.24.6+ | Pre-installed on the VM |
| Node.js | 20.11 | Pinned in `.nvmrc`; use nvm (see below) |
| Make | any | Primary dev entry point |
| Docker | 28.x | Required for `make test` (Postgres testcontainers) |

### Node.js version

The VM ships Node 22 at `/exec-daemon/node`, which takes precedence over nvm in `PATH`. **Always prepend the nvm Node 20.11 bin directory** before running npm/make webapp targets:

```bash
export PATH="$HOME/.nvm/versions/node/v20.11.1/bin:$PATH"
```

Install via nvm if missing: `nvm install 20.11`.

### Docker for tests

Go unit tests spin up an embedded Mattermost server with Postgres via testcontainers (`server/main_test.go`). Docker must be running:

```bash
# Start daemon if not running (Cloud Agent VMs use fuse-overlayfs storage driver)
sudo dockerd &
docker info   # verify connectivity
```

First test run pulls `postgres:15.2-alpine` (~80 MB) and takes ~3 minutes.

### Common commands

All commands run from the repo root:

| Task | Command |
|------|---------|
| Build plugin bundle | `MM_SERVICESETTINGS_ENABLEDEVELOPER=true make dist` |
| Lint (Go + webapp) | `make check-style` |
| Unit tests | `make test` |
| Deploy to local MM | `make deploy` (requires running Mattermost + credentials) |
| Live reload webapp | `make watch` (requires running Mattermost) |

Set `MM_SERVICESETTINGS_ENABLEDEVELOPER=true` on Linux dev machines to build only for the local arch (faster). Without it, `make server` cross-compiles for both `linux-amd64` and `linux-arm64`.

### What is NOT in this repo

- No `docker-compose` for Mattermost — use a local or remote Mattermost instance.
- E2E tests (`make e2e`) require live Mattermost + Microsoft Teams + Azure AD credentials in `server/e2e/testconfig.json`.
- `make deploy` / `make watch` need `MM_SERVICESETTINGS_SITEURL` and admin credentials or a personal access token.

### Lint/test scope

- **Go**: golangci-lint, go vet, mattermost-govet (license checker)
- **Webapp**: ESLint, TypeScript (`tsc`), Jest
- CI delegates to the shared Mattermost plugin workflow (`.github/workflows/ci.yml`)
