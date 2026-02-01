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

## Commands

```bash
make build    # build binary
make test     # run tests
make lint     # run golangci-lint (enforces style rules)
```
