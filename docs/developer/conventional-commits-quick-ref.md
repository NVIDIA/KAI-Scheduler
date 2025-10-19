# Quick Reference: Conventional Commits

A quick guide for contributors to write proper commit messages for KAI Scheduler.

## Format

```
<type>[optional scope]: <description>

[optional body]

[optional footer]
```

## Common Types

| Type | Use For | Example |
|------|---------|---------|
| `feat` | New features | `feat(scheduler): add GPU topology support` |
| `fix` | Bug fixes | `fix(binder): resolve race condition` |
| `docs` | Documentation | `docs: update installation guide` |
| `refactor` | Code refactoring | `refactor(scheduler): simplify cache logic` |
| `test` | Tests | `test(binder): add integration tests` |
| `chore` | Maintenance | `chore(deps): update k8s to v1.29` |
| `perf` | Performance | `perf(scheduler): optimize node filtering` |
| `ci` | CI/CD changes | `ci: add coverage reporting` |

## Common Scopes

| Scope | Component |
|-------|-----------|
| `scheduler` | Core scheduler |
| `binder` | Binder service |
| `admission` | Admission webhook |
| `operator` | Operator |
| `podgrouper` | Pod grouper |
| `chart` | Helm chart |
| `api` | API changes |

## Quick Examples

```bash
# Feature
git commit -m "feat(scheduler): add topology-aware scheduling"

# Bug fix
git commit -m "fix(binder): prevent null pointer in node selection"

# Documentation
git commit -m "docs: add GPU sharing examples"

# Breaking change
git commit -m "feat(api)!: remove deprecated status field

BREAKING CHANGE: The status.phase field is removed.
Use status.state instead."

# Multiple paragraphs
git commit -m "fix(scheduler): prevent memory leak in cache

The pod cache was not releasing references properly.
This fix ensures cleanup on pod deletion.

Fixes #1234"
```

## Tips

✅ **DO**
- Use imperative mood: "add" not "added"
- Keep subject under 72 characters
- Capitalize subject line
- Reference issues: "Fixes #123"
- Explain WHY in body, not HOW

❌ **DON'T**
- End subject with period
- Use past tense
- Be vague: "fix bug" or "update code"
- Forget scope for large projects

## Validation

Your commits are automatically validated in PRs. If validation fails:

1. Check the error message in PR comments
2. Use `git rebase -i` to edit commits
3. Update commit messages to follow format
4. Force push: `git push --force-with-lease`

## More Info

- [Full Guidelines](../../CONTRIBUTING.md#commit-message-guidelines)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [Release Process](release-process.md)
