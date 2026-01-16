# Pre-commit Hooks

Pre-commit hooks automatically run checks and fixes before each commit to ensure code quality and prevent common mistakes.

## Installation

1. Install pre-commit:
   ```sh
   pip install pre-commit
   # or
   brew install pre-commit
   ```

2. Install hooks:
   ```sh
   pre-commit install
   ```

3. (Optional) Run on all files:
   ```sh
   pre-commit run --all-files
   ```

## Prerequisites

- Go 1.24.4+ installed and in PATH
- goimports: `go install golang.org/x/tools/cmd/goimports@latest`
- golangci-lint: `go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8`

## What Gets Checked

### General File Checks
- Trailing whitespace removal
- End-of-file newline enforcement
- YAML/JSON syntax validation
- Large file detection (>1000KB)
- Merge conflict marker detection
- Case conflict detection
- Line ending enforcement (LF)
- **Private key detection** - Prevents accidental commit of secrets

### Go-Specific Checks
- **goimports** - Formats code and organizes imports
- **go mod tidy** - Ensures dependencies are in sync
- **golangci-lint** - Fast linting checks

## Usage

Hooks run automatically on `git commit`. To run manually:

```sh
# Run all hooks on staged files
pre-commit run

# Run on all files
pre-commit run --all-files

# Run specific hook
pre-commit run goimports
```

## Skipping Hooks

To skip hooks for a single commit (use sparingly):

```sh
git commit --no-verify
```

## Troubleshooting

### Tools Not Found

If hooks fail with "Executable not found":

1. Verify tools are installed:
   ```sh
   goimports --version
   golangci-lint --version
   ```

2. Ensure Go bin directory is in PATH:
   ```sh
   export PATH="$(go env GOBIN):$(go env GOPATH)/bin:$HOME/go/bin:$PATH"
   ```

### Private Key Detected

If `detect-private-key` fails, review the flagged file and remove any secrets before committing.

## Configuration

Configuration is in `.pre-commit-config.yaml`. See the [pre-commit documentation](https://pre-commit.com) for details.
