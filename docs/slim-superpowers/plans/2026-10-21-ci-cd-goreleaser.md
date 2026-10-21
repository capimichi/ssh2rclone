# CI/CD & GoReleaser Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Automate testing, linting, multi-platform compilation, and GitHub releases for `ssh2rclone` using GitHub Actions and GoReleaser v2, with version metadata injection and explicit guidance in `README.md` and `AGENTS.md`.

**Architecture:** Add package-level version variables and a `--version` flag in `main.go`. Configure GoReleaser v2 in `.goreleaser.yaml` to cross-compile for Linux, Darwin, and Windows (amd64/arm64) and package archives with SHA-256 checksums. Implement separate GitHub Actions workflows for CI (tests + golangci-lint) and Releases (GoReleaser on `v*` tags). Add a root `Makefile` for developer workflows and provide documentation in `README.md` and `AGENTS.md`.

**Tech Stack:** Go 1.23+, GoReleaser v2, GitHub Actions (`actions/setup-go@v5`, `golangci/golangci-lint-action@v6`, `goreleaser/goreleaser-action@v6`), Make.

## Global Constraints

- Target OS/Architectures: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`, `windows/arm64`.
- Version injection ldflags: `-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}`.
- Delimiter safety and isolation principles of `ssh2rclone` must remain intact.
- Tests must pass with `-race` enabled.

---

### Task 1: CLI Version Flag & Metadata Variables

**Files:**
- Modify: `main.go`
- Test: `main_test.go`

**Interfaces:**
- Consumes: Standard `flag` package, `os`, `fmt`.
- Produces: Package-level vars `version`, `commit`, `date`; flag `-version` / `--version` which prints version info and exits 0 before any file path validation.

- [x] **Step 1: Write failing test in `main_test.go`**

Add `TestVersionFlag` to `main_test.go`:

```go
func TestVersionFlag(t *testing.T) {
	cmd := exec.Command("go", "run", "main.go", "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected -version to exit with 0, got error: %v, output: %s", err, string(output))
	}
	outStr := string(output)
	if !strings.Contains(outStr, "ssh2rclone version") {
		t.Errorf("expected output to contain 'ssh2rclone version', got: %s", outStr)
	}
}
```

- [x] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestVersionFlag .`  
Expected: FAIL (`flag provided but not defined: -version` or exits with non-zero).

- [x] **Step 3: Update `main.go` with version variables and flag handling**

In `main.go`:
1. Add variables:
```go
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)
```
2. In `main()`, add flag:
```go
showVersion := flag.Bool("version", false, "Print version information and exit")
```
3. Immediately after `flag.Parse()`:
```go
if *showVersion {
	fmt.Printf("ssh2rclone version %s (commit: %s, date: %s)\n", version, commit, date)
	os.Exit(0)
}
```

- [x] **Step 4: Run test to verify it passes**

Run: `go test -v -run TestVersionFlag .`  
Expected: PASS.

---

### Task 2: GoReleaser Configuration (`.goreleaser.yaml`)

**Files:**
- Create: `.goreleaser.yaml`

**Interfaces:**
- Consumes: Git tags, Go 1.23+ build environment.
- Produces: Multi-platform binaries, tar.gz/zip archives, SHA-256 checksums, and release changelog.

- [x] **Step 1: Create `.goreleaser.yaml`**

```yaml
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: ssh2rclone
    main: ./main.go
    binary: ssh2rclone
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.Commit}}
      - -X main.date={{.Date}}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64

archives:
  - id: default
    format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip
    files:
      - README.md
      - LICENSE*

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
  groups:
    - title: "Features"
      regexp: '^.*?feat(\([[:word:]]+\))??!?:.+$'
      order: 0
    - title: "Bug Fixes"
      regexp: '^.*?fix(\([[:word:]]+\))??!?:.+$'
      order: 1
    - title: "Others"
      order: 999
```

- [x] **Step 2: Add `dist/` to `.gitignore`**

Ensure `.gitignore` contains `dist/` so local release builds are ignored.

- [x] **Step 3: Validate configuration syntax**

Run: `git check-ignore dist/`  
Expected: `dist/` is ignored.

---

### Task 3: Developer Tooling (`Makefile`)

**Files:**
- Create: `Makefile`

**Interfaces:**
- Consumes: Shell / Git commands.
- Produces: Make targets `build`, `test`, `lint`, `snapshot`, `clean`.

- [x] **Step 1: Create `Makefile`**

```makefile
BINARY_NAME=ssh2rclone
BIN_DIR=bin
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE?=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS=-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: all build test lint snapshot clean

all: test build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) main.go

test:
	go test -v -race ./...

lint:
	@which golangci-lint > /dev/null || (echo "golangci-lint is not installed" && exit 1)
	golangci-lint run

snapshot:
	@which goreleaser > /dev/null || (echo "goreleaser is not installed" && exit 1)
	goreleaser release --snapshot --clean

clean:
	rm -rf $(BIN_DIR) dist
```

- [x] **Step 2: Run `make test` and `make build`**

Run: `make test && make build`  
Expected: Tests pass, binary `./bin/ssh2rclone` is created.

- [x] **Step 3: Test local binary version output**

Run: `./bin/ssh2rclone -version`  
Expected: Prints `ssh2rclone version ... (commit: ..., date: ...)` with code 0.

---

### Task 4: GitHub Actions Workflows

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: GitHub Actions runner, `secrets.GITHUB_TOKEN`.
- Produces: CI automated testing on pull requests/pushes; automated GitHub Release with attached binaries on `v*` tag push.

- [x] **Step 1: Create `.github/workflows/ci.yml`**

```yaml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

permissions:
  contents: read

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'
          cache: true

      - name: Run tests with race detector
        run: go test -v -race ./...

  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'
          cache: true

      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

- [x] **Step 2: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  goreleaser:
    name: Release with GoReleaser
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: 'go.mod'
          cache: true

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [x] **Step 3: Validate workflow syntax locally**

Verify valid YAML structure and paths.

---

### Task 5: Documentation & AI Agent Guidance (`README.md` & `AGENTS.md`)

**Files:**
- Modify: `README.md`
- Create: `AGENTS.md`

**Interfaces:**
- Consumes: Release specifications from design doc.
- Produces: Clear developer instructions in `README.md` and automated guidelines for AI agents in `AGENTS.md`.

- [x] **Step 1: Update `README.md`**

Add sections for:
1. Makefile commands (`make build`, `make test`, `make lint`).
2. `--version` CLI flag usage.
3. Release and Versioning workflow using Git tags (`vX.Y.Z`).

- [x] **Step 2: Create `AGENTS.md`**

Create `AGENTS.md` defining:
1. Architecture & layout summary of the repository.
2. Coding standards (idempotency, delimiter isolation, zero mutation outside delimiter block).
3. Quality gates before making changes: always run `make test`.
4. Release & Tagging guidelines for agents:
   - SemVer rules (`vMAJOR.MINOR.PATCH`).
   - Prerequisites before tagging: clean git working tree, `make test` passing.
   - Tag creation and push command syntax:
     ```bash
     git tag -a vX.Y.Z -m "Release vX.Y.Z"
     git push origin vX.Y.Z
     ```
   - Automated workflow trigger notification.

- [x] **Step 3: Verification**

Ensure all tests pass: `go test -v ./...` and `make build`.
