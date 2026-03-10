# CLAUDE.md

## Project Overview

`libk8soci` is a shared Go library providing OCI (Open Container Initiative) registry and Kubernetes utilities used by [sbom-operator](https://github.com/ckotzbauer/sbom-operator) and [vulnerability-operator](https://github.com/ckotzbauer/vulnerability-operator). It is a pure library (no `main` package, no binary output) published as a Go module at `github.com/ckotzbauer/libk8soci`.

Licensed under MIT. Author: Christian Kotzbauer.

## Tech Stack

- **Language:** Go 1.24.1
- **Kubernetes client:** `k8s.io/client-go` v0.32.3, `k8s.io/api` v0.32.3, `k8s.io/apimachinery` v0.32.3
- **Git operations:** `github.com/go-git/go-git/v5` v5.14.0
- **OCI/Container registry:** `github.com/google/go-containerregistry` v0.20.3, `github.com/anchore/stereoscope` v0.1.2, `github.com/novln/docker-parser` v1.0.0
- **Docker config parsing:** `github.com/docker/cli` v29.2.1
- **JWT (GitHub App auth):** `github.com/golang-jwt/jwt` v3.2.2
- **Logging:** `github.com/sirupsen/logrus` v1.9.3
- **Testing:** `github.com/stretchr/testify` v1.10.0
- **Linting:** golangci-lint v2.0.2, gosec v2.22.3
- **Dependency management:** Renovate (monthly schedule, Kubernetes packages grouped)

## Project Structure

```
libk8soci/
├── go.mod                          # Go module definition
├── go.sum                          # Dependency checksums
├── Makefile                        # Build, test, lint targets
├── README.md                       # Brief project description
├── LICENSE                         # MIT license
├── renovate.json                   # Renovate dependency bot config
├── .gitignore                      # Ignores .tmp/
├── cover.out                       # Test coverage output
├── hack/
│   └── prepare-auth-files.sh       # Helper to create dockerconfigjson/dockercfg test secrets
├── pkg/
│   ├── git/
│   │   ├── git.go                  # Git clone/pull/commit/push operations via go-git
│   │   └── auth/
│   │       ├── auth.go             # GitAuthenticator interface
│   │       ├── token.go            # Token-based git auth
│   │       ├── basic.go            # Username/password git auth
│   │       └── github.go           # GitHub App JWT-based git auth
│   ├── oci/
│   │   ├── types.go                # KubeCreds and RegistryImage types
│   │   ├── registry.go             # Registry auth resolution, pull secret handling, proxy registry mapping
│   │   ├── configfile.go           # Legacy Docker config file parsing (ported from docker/cli)
│   │   └── configfile_test.go      # Tests for legacy config parsing
│   ├── kubernetes/
│   │   ├── types.go                # ContainerInfo and PodInfo types
│   │   └── kubernetes.go           # KubeClient: namespace/pod listing, secret loading, pod informer
│   └── util/
│       ├── util.go                 # Unescape helper function
│       └── util_test.go            # Tests for Unescape
├── .tmp/                           # Downloaded lint tool binaries (gitignored)
└── .github/
    ├── label-commands.json         # Issue/PR label bot commands
    └── workflows/
        ├── test.yml                # Test workflow (push to main)
        ├── code-checks.yml         # Lint workflows (gosec + golangci-lint, all branches + PRs)
        ├── stale.yml               # Daily stale issue/PR cleanup
        ├── label-issues.yml        # Auto-labeling on issues/PRs
        └── size-label.yml          # PR size labeling
```

## Architecture & Patterns

### Package Design

The library is organized into four packages under `pkg/`, each with a focused responsibility:

1. **`pkg/git`** - Git repository operations (clone, pull, commit, push) using `go-git`. The `GitAccount` struct holds credentials and identity (name/email). Authentication is resolved via a chain of `GitAuthenticator` implementations (token, basic, GitHub App), tried in order until one reports `IsAvailable()`.

2. **`pkg/git/auth`** - Pluggable authentication for git operations. Defines the `GitAuthenticator` interface with `IsAvailable() bool` and `ResolveAuth() (*http.BasicAuth, error)`. Three implementations:
   - `TokenGitAuthenticator` - uses a personal access token
   - `BasicGitAuthenticator` - uses username/password
   - `GitHubAuthenticator` - obtains a short-lived token via GitHub App JWT flow (base64-encoded PEM private key, app ID, installation ID)

3. **`pkg/oci`** - OCI registry authentication. Resolves Docker auth configs from Kubernetes pull secrets (both modern `dockerconfigjson` and legacy `dockercfg` formats). `ConvertSecrets` translates Kubernetes pull secrets into `stereoscope` `RegistryCredentials`, with support for proxy registry URL mapping.

4. **`pkg/kubernetes`** - Kubernetes API interactions. `KubeClient` wraps `client-go` `Clientset` for listing namespaces, pods, and secrets. `ExtractPodInfos` extracts container image information from pod statuses (including init and ephemeral containers). `CreatePodInformer` sets up a `SharedIndexInformer` for watching pod changes across all namespaces.

5. **`pkg/util`** - Small utility functions. Currently contains `Unescape` for stripping backslashes and double-quotes from strings (used for label selectors).

### Key Patterns

- **Strategy pattern** for git authentication (`GitAuthenticator` interface with multiple implementations)
- **No `main` package** - this is a library consumed by other projects
- **Logrus** used consistently for structured logging throughout all packages
- **`fmt.Errorf` with `%w`** for error wrapping in most code; `github.com/pkg/errors` used in `configfile.go`
- **Kubernetes client-go informer pattern** for pod watching

## Build & Development

### Prerequisites

- Go 1.24.1+
- For linting: `make bootstrap-tools` downloads golangci-lint and gosec to `.tmp/`

### Makefile Targets

| Target | Command | Description |
|--------|---------|-------------|
| `all` / `build` | `go fmt ./... && go vet ./... && go test $(go build ./...)` | Format, vet, and build |
| `fmt` | `go fmt ./...` | Format all Go files |
| `vet` | `go vet ./...` | Run go vet |
| `test` | `go test $(go list ./...) -coverprofile cover.out` | Run all tests with coverage |
| `lint` | `.tmp/golangci-lint run --timeout 5m` | Run golangci-lint |
| `lintsec` | `.tmp/gosec ./...` | Run gosec security scanner |
| `bootstrap-tools` | Downloads golangci-lint v2.0.2 and gosec v2.22.3 to `.tmp/` | Install lint tools |

## Testing

- **Framework:** Standard Go testing with `github.com/stretchr/testify` for assertions
- **Test files:**
  - `pkg/util/util_test.go` - table-driven tests for `Unescape` function
  - `pkg/oci/configfile_test.go` - tests for legacy Docker config file parsing (JSON deserialization, save/load round-trip)
- **Run tests:** `make test`
- **Coverage output:** `cover.out` (generated by `make test`)
- **Test pattern:** Table-driven tests using `testify/assert`

## Linting & Code Style

- **golangci-lint v2.0.2** - general Go linting (no custom `.golangci.yml` config file present; uses defaults)
- **gosec v2.22.3** - security-focused static analysis
- **`go fmt`** - standard Go formatting enforced via Makefile `fmt` target
- **`go vet`** - standard Go static analysis enforced via Makefile `vet` target
- **`// nolint` comments** used sparingly to suppress specific warnings (e.g., in `kubernetes.go` for secret type checks, in `configfile_test.go` for deferred cleanup)
- **No `.golangci.yml`** configuration file - relies on default linter settings

## CI/CD

All CI workflows use reusable workflows from `ckotzbauer/actions-toolkit@0.47.0`.

| Workflow | File | Trigger | Purpose |
|----------|------|---------|---------|
| **test** | `.github/workflows/test.yml` | Push to `main` | Runs `make test`, reports coverage. Installs Go, cosign, goreleaser. |
| **code-checks** | `.github/workflows/code-checks.yml` | Push to any branch + PRs | Two jobs: `gosec` (security lint) and `golint` (golangci-lint). Both run `make bootstrap-tools` first. |
| **stale** | `.github/workflows/stale.yml` | Daily cron (`0 0 * * *`) | Marks stale issues/PRs |
| **label-issues** | `.github/workflows/label-issues.yml` | Issue/PR comments, opened issues/PRs | Auto-labels via commands (e.g., `/hold`, `/kind bug`, `/lifecycle stale`) |
| **size-label** | `.github/workflows/size-label.yml` | PR opened/reopened/synchronized | Labels PRs by size |

## Key Commands

```bash
# Run all tests with coverage
make test

# Format code
make fmt

# Run go vet
make vet

# Build (format + vet + test)
make build

# Install lint tools (one-time setup)
make bootstrap-tools

# Run golangci-lint
make lint

# Run gosec security scan
make lintsec
```

## Important Conventions

- **Module path:** `github.com/ckotzbauer/libk8soci` - all internal imports use this prefix
- **Package layout:** All library code lives under `pkg/` with no `internal/` packages
- **No binary:** This is a library-only module; there is no `cmd/` directory or `main.go`
- **Default branch:** `main`
- **Logging:** Use `logrus` for all logging (not `log` or `fmt.Println`); use `logrus.WithError(err)` for error context
- **Error handling:** Wrap errors with `fmt.Errorf("context: %w", err)` for proper error chains
- **Authentication chain:** Git auth resolves by trying token, then basic, then GitHub App - first available wins
- **Docker config compatibility:** Both modern `dockerconfigjson` and legacy `dockercfg` secret formats are supported
- **Kubernetes secrets:** Only `kubernetes.io/dockerconfigjson` and `kubernetes.io/dockercfg` secret types are processed for image pull credentials
- **Dependency updates:** Managed via Renovate on a monthly schedule; Kubernetes packages (`k8s.io/api`, `k8s.io/apimachinery`, `k8s.io/client-go`) are grouped together
- **Test style:** Table-driven tests with `testify/assert`
- **Proxy registry support:** `ConvertSecrets` accepts a `proxyRegistryMap` to remap registry URLs in credentials
