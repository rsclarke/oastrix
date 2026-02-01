# Project Guidelines

## Style

Follow [Effective Go](https://go.dev/doc/effective_go) and [Google Go Style Guide](https://google.github.io/styleguide/go/guide).

Full reference: [docs/go-style-guide.md](docs/go-style-guide.md)

### Rules Not Enforced by Linting

- Comments explain **why**, not **what**
- Error strings: lowercase, no trailing punctuation
- Do not store `context.Context` in structs
- Goroutines must be stoppable via context cancellation
- Prefer `any` over `interface{}`

## Branches

Follow [Conventional Branch](https://conventional-branch.github.io/) when creating branches.

**Format**: `<prefix>/<issue-number>-<description>`

**Prefixes**:
- `feature/` or `feat/` – new features
- `bugfix/` or `fix/` – bug fixes
- `hotfix/` – urgent production fixes
- `release/` – release preparation
- `chore/` – non-code tasks (deps, docs)

**Rules**:
- Use lowercase letters, numbers, and hyphens only
- No consecutive, leading, or trailing hyphens
- Keep descriptions concise but clear

**Examples**:
- `feat/123-add-login`
- `fix/456-header-bug`
- `chore/update-dependencies`

## Commits

Follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) when writing commit messages.

## Commands

```bash
make build    # build binary
make test     # run tests
make lint     # run golangci-lint (enforces style rules)
```
