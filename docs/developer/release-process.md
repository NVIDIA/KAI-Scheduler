# Release Process Guide

This document describes the release process for KAI Scheduler, including creating release candidates, publishing final releases, and managing the changelog.

## Table of Contents

- [Overview](#overview)
- [Conventional Commits](#conventional-commits)
- [Creating a Release Candidate](#creating-a-release-candidate)
- [Publishing a Final Release](#publishing-a-final-release)
- [Changelog Management](#changelog-management)
- [Troubleshooting](#troubleshooting)

## Overview

KAI Scheduler follows a structured release process:

1. **Development**: Code changes are merged to `main` or version branches (e.g., `v0.9`)
2. **Release Candidate**: Create RC for testing (`vX.Y.Z-rc.N`)
3. **Testing**: Validate RC in test environments
4. **Final Release**: Tag the validated RC as the final release (`vX.Y.Z`)
5. **Changelog Update**: Automatically update CHANGELOG.md

## Conventional Commits

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

### Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Examples

```bash
feat(scheduler): add GPU topology-aware scheduling
fix(binder): resolve race condition in pod binding
docs: update contribution guidelines
chore(deps): upgrade kubernetes to v1.29
```

See [CONTRIBUTING.md](../CONTRIBUTING.md#commit-message-guidelines) for complete guidelines.

## Creating a Release Candidate

Release candidates (RCs) are pre-release versions used for testing before the final release.

### Prerequisites

- You must be a maintainer with write access to the repository
- The code should be in a stable state on the target branch
- All CI/CD checks should pass

### Process

1. **Navigate to GitHub Actions**
   - Go to: https://github.com/NVIDIA/KAI-Scheduler/actions
   - Select "Create Release Candidate" workflow

2. **Trigger the Workflow**
   - Click "Run workflow"
   - **Version**: Enter the RC version (e.g., `v0.10.0-rc.1`)
   - **Source Branch**: Select the branch to create RC from (e.g., `main` or `v0.9`)
   - Click "Run workflow"

3. **Version Format**
   - Format: `vX.Y.Z-rc.N`
   - Example: `v0.10.0-rc.1`, `v0.10.0-rc.2`
   - X.Y.Z: Target release version
   - N: RC iteration number (1, 2, 3, ...)

### What Happens Automatically

The workflow will:
1. ✅ Validate version format
2. 🏗️ Build all Docker images (amd64 and arm64)
3. 📦 Package Helm chart with RC version
4. 🚀 Push images to `ghcr.io/nvidia/kai-scheduler`
5. 📤 Push Helm chart to OCI registry
6. 🏷️ Create Git tag (e.g., `v0.10.0-rc.1`)
7. 📝 Create GitHub pre-release with notes
8. 📋 Attach Helm chart to release

### Testing the RC

```bash
# Install RC version
helm install kai-scheduler \
  oci://ghcr.io/nvidia/kai-scheduler/kai-scheduler \
  --version v0.10.0-rc.1

# Or pull specific image
docker pull ghcr.io/nvidia/kai-scheduler/scheduler:v0.10.0-rc.1
```

### Creating Additional RCs

If issues are found:
1. Fix the issues and merge to the source branch
2. Create a new RC with incremented number (e.g., `v0.10.0-rc.2`)
3. Repeat testing

## Publishing a Final Release

Once an RC is validated and approved for release:

### Process

1. **Create Release from GitHub**
   - Go to: https://github.com/NVIDIA/KAI-Scheduler/releases
   - Click "Draft a new release"

2. **Configure Release**
   - **Tag**: Enter the final version (e.g., `v0.10.0`)
   - **Target**: Select the commit of the approved RC
     - Usually the commit tagged with the latest RC (e.g., `v0.10.0-rc.3`)
   - **Title**: `v0.10.0`
   - **Description**: Copy release notes from the RC or generate new ones

3. **Release Options**
   - ❌ Uncheck "Set as a pre-release" (for final releases)
   - ✅ Check "Set as the latest release" (if applicable)

4. **Publish**
   - Click "Publish release"

### What Happens Automatically

When the release is published:

1. **Build & Publish Workflow** (`on-release.yaml`):
   - Packages Helm chart with final version
   - Attaches chart to GitHub release
   
2. **Changelog Update Workflow** (`update-changelog-on-release.yaml`):
   - Moves entries from `[Unreleased]` to `[vX.Y.Z]` section
   - Adds release date
   - Creates a PR with changelog updates
   - **Action Required**: Review and merge the changelog PR

### Manual Steps After Release

1. **Review Changelog PR**
   - A bot will create a PR titled: `chore(release): update CHANGELOG for vX.Y.Z`
   - Review the changes
   - Merge the PR

2. **Announce Release**
   - Announce in Slack channel: #batch-wg
   - Update documentation if needed
   - Notify users of breaking changes

## Changelog Management

### During Development

Contributors should update `CHANGELOG.md` when making user-facing changes:

1. **Add entries under `[Unreleased]`**
   ```markdown
   ## [Unreleased]
   
   ### Added
   - New GPU topology-aware scheduling feature
   
   ### Fixed
   - Race condition in pod binding process
   ```

2. **Use appropriate categories**:
   - `Added`: New features
   - `Changed`: Changes in existing functionality
   - `Deprecated`: Soon-to-be removed features
   - `Removed`: Removed features
   - `Fixed`: Bug fixes
   - `Security`: Security fixes

3. **Skip internal changes**: Don't log refactoring, test updates, or internal changes that don't affect users

### Automated Updates

The changelog is automatically updated on release:
- Entries move from `[Unreleased]` to versioned section
- Release date is added
- A PR is created for review

### Manual Updates (if needed)

If the automated PR needs adjustments:

```bash
# Edit CHANGELOG.md manually
vim CHANGELOG.md

# Commit changes
git commit -m "chore(release): adjust CHANGELOG for vX.Y.Z"

# Push to the changelog PR branch or create new PR
git push
```

## Troubleshooting

### RC Creation Fails

**Problem**: Invalid version format

**Solution**: Ensure version follows `vX.Y.Z-rc.N` format (e.g., `v0.10.0-rc.1`)

---

**Problem**: Build fails

**Solution**: 
1. Check GitHub Actions logs for specific errors
2. Ensure all tests pass on source branch
3. Verify `make build` works locally

---

**Problem**: Helm chart push fails

**Solution**: Verify GITHUB_TOKEN has correct permissions for packages

### Commit Validation Fails

**Problem**: Commit messages don't pass validation

**Solution**: Follow Conventional Commits format. See [CONTRIBUTING.md](../CONTRIBUTING.md#commit-message-guidelines)

```bash
# Bad
git commit -m "fixed bug"

# Good
git commit -m "fix(scheduler): resolve race condition in cache updates"
```

### Changelog PR Not Created

**Problem**: No changelog PR after release

**Solution**: 
1. Check if `[Unreleased]` section exists and has content
2. Check GitHub Actions logs for errors
3. Manually create PR if needed

## Best Practices

### Release Cadence

- **Major releases** (X.0.0): Every 3-6 months
- **Minor releases** (X.Y.0): Every 4-6 weeks
- **Patch releases** (X.Y.Z): As needed for critical fixes

### RC Testing

Before approving an RC:
- ✅ Deploy to staging environment
- ✅ Run full test suite
- ✅ Verify upgrade path from previous version
- ✅ Test critical workflows
- ✅ Check resource usage and performance
- ✅ Validate documentation updates

### Version Numbering

Follow [Semantic Versioning](https://semver.org/):
- **MAJOR** (X.0.0): Incompatible API changes
- **MINOR** (0.Y.0): Backwards-compatible functionality
- **PATCH** (0.0.Z): Backwards-compatible bug fixes

### Release Notes

Include in release notes:
- Summary of major changes
- Breaking changes with migration guide
- Deprecation notices
- Installation instructions
- Known issues
- Contributors acknowledgment

## Emergency Hotfix Process

For critical production issues:

1. **Create Hotfix Branch**
   ```bash
   git checkout -b hotfix/vX.Y.Z+1 vX.Y.Z
   ```

2. **Fix Issue**
   ```bash
   git commit -m "fix(component): critical issue description"
   ```

3. **Create RC** (optional for validation)
   - Use RC workflow: `vX.Y.Z+1-rc.1`

4. **Release**
   - Create release from hotfix branch
   - Tag as `vX.Y.Z+1`

5. **Backport to Main**
   ```bash
   git checkout main
   git cherry-pick <commit-hash>
   ```

## Additional Resources

- [Conventional Commits](https://www.conventionalcommits.org/)
- [Keep a Changelog](https://keepachangelog.com/)
- [Semantic Versioning](https://semver.org/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Contributing Guide](../CONTRIBUTING.md)

## Questions?

- Open an issue: https://github.com/NVIDIA/KAI-Scheduler/issues
- Slack: #batch-wg channel
- Email maintainers: See [MAINTAINERS.md](../MAINTAINERS.md)
