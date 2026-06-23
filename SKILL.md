---
name: "go-best-practices"
description: "Guides idiomatic Go development, code review, and refactoring. Invoke when writing, reviewing, or refactoring Go code in this workspace."
---

# Go Best Practices

Follow these guidelines when working with Go in this workspace.

## Core Principles

- Prefer clarity over cleverness.
- Keep functions and methods small and focused.
- Favor composition over inheritance.
- Use the standard library whenever it suffices.
- Avoid premature abstraction.

## Code Style

- Format all Go code with `gofmt`.
- Run `go vet` and `golint` (or `staticcheck`) before committing.
- Use meaningful names. Exported names use PascalCase; unexported names use camelCase.
- Acronyms and initialisms should stay consistent in case (e.g., `URL`, `HTTP`, `ID`).
- Avoid naked returns and avoid unnecessary `else` after `return`.

## Error Handling

- Always check errors explicitly; do not silently ignore them.
- Wrap errors with context using `fmt.Errorf("...: %w", err)`.
- Define sentinel errors for package-level error identification.
- Avoid panics in library code; reserve `panic` for unrecoverable failures.

## Interfaces

- Define interfaces at the point of use (consumer-side), not at implementation time.
- Keep interfaces small; the Go standard library favors single-method interfaces.
- Accept interfaces, return concrete types.

## Concurrency

- Do not share memory by communicating; instead, communicate by sharing memory.
- Prefer channels and goroutines where concurrency adds clarity.
- Always handle goroutine lifecycles; avoid leaking goroutines.
- Use `sync.WaitGroup`, `context.Context`, and cancellation consistently.
- Protect shared state with mutexes only when channels are not a natural fit.

## Packages

- Organize code by responsibility, not by layer.
- Avoid package-level mutable state.
- Minimize exported surface area.
- Do not create `util` or `common` packages; name packages by what they do.

## Testing

- Write table-driven tests using anonymous structs.
- Use `t.Parallel()` for independent tests.
- Name test files `*_test.go` and keep them in the same package.
- Use `testify/assert` or `cmp` only if the project already depends on them; otherwise prefer the standard library.
- Test behavior, not implementation details.

## Context

- Pass `context.Context` as the first argument to functions that need cancellation or deadlines.
- Do not store `context.Context` in structs; pass it through the call stack.

## Performance

- Avoid premature optimization; profile before optimizing.
- Use `sync.Pool` for high-frequency short-lived allocations only when proven necessary.
- Prefer `strings.Builder` for efficient string concatenation in loops.

## Dependencies

- Minimize external dependencies.
- Pin dependency versions in `go.mod`.
- Run `go mod tidy` after adding or removing imports.

## Security

- Validate all external input at system boundaries.
- Avoid constructing SQL or shell commands with string concatenation; use parameterized queries.
- Do not log sensitive data (passwords, tokens, keys).

## Code Review Checklist

- [ ] Code is formatted with `gofmt`.
- [ ] `go vet` and `staticcheck` report no issues.
- [ ] Errors are checked and wrapped appropriately.
- [ ] Tests cover new behavior.
- [ ] Interfaces are consumer-side and minimal.
- [ ] No unnecessary exported symbols.
- [ ] Goroutines have defined lifecycles.
