# Design Document: Contribution Policies and Release Automation (RUN-27306)

**Author:** KAI Scheduler Team  
**Date:** October 19, 2025  
**Status:** Draft  
**JIRA:** RUN-27306

## Executive Summary

This design document outlines the implementation of comprehensive contribution policies and automated release processes for the KAI Scheduler open source project. The goal is to improve code quality, maintainability, and contributor experience through standardized practices and automation.

## Background

### Problem Statement

The KAI Scheduler project currently has:
- Basic contribution guidelines in `CONTRIBUTING.md`
- Manual changelog management
- Basic GitHub Actions for releases
- No standardized commit message format
- No pull request templates
- Manual release candidate creation process

### Goals

1. Establish clear contribution policies following industry best practices
2. Implement Conventional Commits for structured commit messages
3. Automate release candidate creation and chart publication
4. Automate changelog generation and updates
5. Provide templates for consistent issue and pull request creation

### Non-Goals

- Changing the existing Git branching strategy
- Implementing automated semantic versioning (will be considered for future work)
- Changing the existing CI/CD infrastructure significantly

## Design

### 1. Commit Message Convention

#### Specification

Adopt [Conventional Commits v1.0.0](https://www.conventionalcommits.org/en/v1.0.0/) specification:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

#### Commit Types

- **feat**: A new feature
- **fix**: A bug fix
- **docs**: Documentation only changes
- **style**: Changes that don't affect code meaning (formatting, white-space)
- **refactor**: Code changes that neither fix a bug nor add a feature
- **perf**: Performance improvements
- **test**: Adding or updating tests
- **build**: Changes to build system or dependencies
- **ci**: Changes to CI/CD configuration
- **chore**: Other changes that don't modify src or test files
- **revert**: Reverts a previous commit

#### Scopes (Optional)

Project-specific scopes for KAI Scheduler:
- `scheduler`: Core scheduler logic
- `binder`: Binder component
- `podgrouper`: Pod grouper functionality
- `admission`: Admission webhook
- `operator`: Operator implementation
- `queuecontroller`: Queue controller
- `podgroupcontroller`: PodGroup controller
- `resourcereservation`: Resource reservation
- `fairshare`: Fair share functionality
- `chart`: Helm chart changes
- `crd`: CRD changes
- `api`: API changes

#### Examples

```
feat(scheduler): add dynamic resource allocation support

fix(binder): resolve race condition in pod binding process

docs: update contribution guidelines with conventional commits

chore(ci): upgrade GitHub Actions to use Node 20

BREAKING CHANGE: remove deprecated fairshare algorithm
```

#### Breaking Changes

Breaking changes MUST be indicated:
- Add `!` after type/scope: `feat(api)!: remove deprecated field`
- Or include `BREAKING CHANGE:` in footer

### 2. Changelog Management

#### Format

Follow [Keep a Changelog v1.1.0](https://keepachangelog.com/en/1.1.0/) format (already in use):

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- New features

### Changed
- Changes in existing functionality

### Deprecated
- Soon-to-be removed features

### Removed
- Removed features

### Fixed
- Bug fixes

### Security
- Security fixes

## [x.y.z] - YYYY-MM-DD
...
```

#### Automation Strategy

**Manual Phase (Current):**
- Contributors update `CHANGELOG.md` in their PRs for user-facing changes
- Changes go under `[Unreleased]` section
- Maintainers move entries to versioned sections during release

**Automated Phase (Future Enhancement):**
- Use conventional commits to auto-generate changelog entries
- Tools like `git-cliff` or `conventional-changelog` to parse commits
- Auto-create release notes from commits between tags

### 3. Pull Request Template

Create `.github/pull_request_template.md` with:

#### Sections

1. **Description**: What and why
2. **Type of Change**: Checklist (feature, bugfix, docs, etc.)
3. **Related Issues**: Links to JIRA/GitHub issues
4. **Testing**: How was it tested
5. **Checklist**: Pre-merge requirements
6. **Breaking Changes**: If applicable
7. **Documentation**: Updates needed

### 4. Release Candidate Automation

#### Workflow Trigger

Automate RC creation when:
- PR is merged from feature branch to `main` or version branches (e.g., `v0.9`)
- Commit message contains specific pattern: `chore(release): prepare release candidate vX.Y.Z-rc.N`
- Or via manual workflow dispatch

#### RC Workflow Steps

1. **Validate Inputs**
   - Check if on correct branch (main or version branch)
   - Validate version format (semver + rc suffix)

2. **Build RC Chart**
   - Update chart version in `Chart.yaml` to RC version
   - Update appVersion to RC version
   - Package Helm chart: `helm package ./deployments/kai-scheduler`

3. **Tag RC**
   - Create Git tag: `vX.Y.Z-rc.N`
   - Push tag to repository

4. **Build & Push Docker Images**
   - Build all service images with RC tag
   - Push to `ghcr.io/nvidia/kai-scheduler`

5. **Upload RC Chart**
   - Push Helm chart to OCI registry: `oci://ghcr.io/nvidia/kai-scheduler`
   - Attach chart as artifact to GitHub release

6. **Create GitHub Pre-release**
   - Create pre-release in GitHub
   - Include RC notes and changelog
   - Mark as pre-release

#### RC Versioning

Format: `vX.Y.Z-rc.N`
- X.Y.Z: Target release version
- N: RC iteration number (1, 2, 3, ...)

Example: `v0.10.0-rc.1`, `v0.10.0-rc.2`

### 5. Final Release Tagging

#### When Version is Released

1. **Tag Latest RC**
   - When final release `vX.Y.Z` is created
   - Tag the commit of the latest RC that passed testing
   - Example: If `v0.10.0-rc.3` is approved, tag it as `v0.10.0`

2. **Update Changelog**
   - Move `[Unreleased]` entries to `[vX.Y.Z] - YYYY-MM-DD`
   - Add version comparison link at bottom
   - Commit changelog update

3. **Trigger Release Workflow**
   - Existing `on-release.yaml` workflow handles final release
   - Builds final chart and images
   - Publishes to registries

### 6. Commit Message Validation

#### Implementation Options

**Option A: GitHub Action (Recommended)**
- Use `commitlint` with `@commitlint/config-conventional`
- Run on PR creation and updates
- Validate all commits in PR

**Option B: Git Hooks (Local Development)**
- Install `husky` and `commitlint` for local validation
- Provide optional pre-commit hook script
- Document in contribution guide

**Option C: Bot Integration**
- Use apps like "Semantic Pull Requests" from GitHub Marketplace
- Validates PR title against conventional commits
- Provides immediate feedback

#### Recommendation

Implement both A and C:
- GitHub Action for comprehensive validation
- Bot for immediate PR title feedback
- Document local hooks as optional developer tool

## Implementation Plan

### Phase 1: Documentation & Templates (Week 1)

1. **Update CONTRIBUTING.md**
   - Add Conventional Commits section
   - Add detailed commit message guidelines
   - Add examples and anti-patterns
   - Update PR submission process

2. **Create PR Template**
   - `.github/pull_request_template.md`
   - Include all required sections

3. **Verify Issue Templates**
   - Ensure existing templates are comprehensive
   - Add feature request template if missing

### Phase 2: Commit Validation (Week 1-2)

1. **Add Commitlint GitHub Action**
   - `.github/workflows/commitlint.yaml`
   - Validate all commits in PR
   - Provide clear error messages

2. **Add Semantic PR Bot**
   - Configure via `.github/semantic.yml`
   - Validate PR titles

3. **Documentation**
   - Add troubleshooting guide for failed validations
   - Provide examples of fixing commit messages

### Phase 3: Release Candidate Automation (Week 2-3)

1. **Create RC Workflow**
   - `.github/workflows/create-release-candidate.yaml`
   - Support manual and automated triggers
   - Build and push RC artifacts

2. **Update Existing Workflows**
   - Ensure compatibility with RC workflow
   - Add RC chart handling to push-artifacts.yaml

3. **Testing**
   - Test RC creation on feature branch
   - Verify chart upload
   - Validate tagging

### Phase 4: Changelog Automation (Week 3-4)

1. **Manual Process Enhancement**
   - Enforce changelog updates in PR template
   - Add PR validation for changelog entries
   - Document changelog guidelines

2. **Future: Automated Generation**
   - Evaluate tools (git-cliff, conventional-changelog)
   - Create proof-of-concept
   - Plan migration strategy

### Phase 5: Final Release Enhancement (Week 4)

1. **Update Release Workflow**
   - Automate changelog version bump
   - Tag latest RC as release
   - Generate release notes from commits

2. **Documentation**
   - Document complete release process
   - Create maintainer runbook
   - Update README with badges

## Technical Considerations

### Backwards Compatibility

- Existing commits don't need to be rewritten
- Conventional commits required only for new contributions
- Gradual adoption with warnings before enforcement

### Tooling Requirements

- **commitlint**: Node.js based commit linter
- **Helm**: For chart packaging (already in use)
- **git-cliff** (optional): For changelog generation
- **GitHub CLI** (optional): For release management

### Security

- All workflows use least-privilege tokens
- No secret exposure in logs
- Chart signing considerations (future work)

### Performance

- Commit validation adds <30s to PR checks
- RC creation workflow ~5-10 minutes
- No impact on development velocity

## Acceptance Criteria Validation

| Criterion | Implementation | Status |
|-----------|----------------|--------|
| Defined commit message structure per conventionalcommits.org | Update CONTRIBUTING.md, add validation | ✅ Planned |
| Changelog with guidelines from keepachangelog.com | Already in use, enhance automation | ✅ Partial |
| Pull request template created | Create .github/pull_request_template.md | ✅ Planned |
| Auto-create RC from branch merges | Create RC workflow with triggers | ✅ Planned |
| Auto-create RC chart and upload | Include in RC workflow | ✅ Planned |
| Tag latest RC when version released | Update release workflow | ✅ Planned |
| Auto-update changelog | Manual first, auto later | ⏳ Phased |

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Contributor friction from new rules | Medium | Clear docs, helpful error messages, grace period |
| Breaking existing workflows | High | Thorough testing, backward compatibility |
| Over-automation complexity | Medium | Start manual, automate incrementally |
| RC versioning conflicts | Low | Clear naming convention, validation |

## Future Enhancements

1. **Semantic Versioning Automation**
   - Auto-determine version bumps from commits
   - Implement `semantic-release` or similar

2. **Automated Release Notes**
   - Generate from conventional commits
   - Group by type and scope
   - Include contributor credits

3. **Dependency Update Automation**
   - Renovate or Dependabot configuration
   - Auto-update Go modules and GitHub Actions

4. **Advanced PR Checks**
   - Auto-label based on changes
   - Size labels (XS, S, M, L, XL)
   - Stale PR management

## References

- [Conventional Commits v1.0.0](https://www.conventionalcommits.org/en/v1.0.0/)
- [Keep a Changelog v1.1.0](https://keepachangelog.com/en/1.1.0/)
- [Semantic Versioning 2.0.0](https://semver.org/)
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Helm Chart Best Practices](https://helm.sh/docs/chart_best_practices/)

## Appendix

### Example Conventional Commits

```bash
# Feature addition
git commit -m "feat(scheduler): add GPU topology-aware scheduling"

# Bug fix with issue reference
git commit -m "fix(binder): prevent race condition in concurrent bindings

Resolves race condition when multiple pods are bound simultaneously
to the same node, causing resource over-allocation.

Fixes #1234"

# Breaking change
git commit -m "feat(api)!: change PodGroup status field structure

BREAKING CHANGE: The status.phase field is renamed to status.state
to align with Kubernetes conventions. Users must update their
monitoring and automation scripts."

# Documentation update
git commit -m "docs: add GPU sharing configuration examples"

# Chore
git commit -m "chore(deps): update kubernetes dependencies to v1.29"
```

### Example PR Title

```
feat(scheduler): implement topology-aware GPU scheduling
fix(binder): resolve race condition in pod binding
docs: update installation guide for air-gapped environments
chore(ci): migrate to GitHub Actions composite actions
```

