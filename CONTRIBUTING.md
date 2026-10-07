# Contributing to Calisero Go

Thank you for considering contributing to the Calisero Go SMS API library! We welcome contributions from the community.

## Development Setup

You need Go 1.22 or later (CI checks the code style with Go 1.27) and, for the linters,
[golangci-lint](https://golangci-lint.run) 2.

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/calisero-go.git`
3. Check the module: `go mod download && go mod verify` (the library has no dependency)
4. Create a new branch: `git checkout -b feature/your-feature-name`

## Code Standards

This project follows strict coding standards:

- **gofmt and goimports**: the code is formatted as they write it, imports grouped with the module's own last
- **golangci-lint**: the linters of `.golangci.yml`, `revive` and `errorlint` among them, report nothing
- **Go 1.22 compatible**: `go.mod` says `go 1.22.0`, and the library may use only what Go 1.22 offers: CI runs the
  tests on every release from 1.22 to 1.27
- **No dependency**: the library and its tests use only the Go standard library
- **Full test coverage** for new features: CI fails below 98% of the statements
- **The API's OpenAPI document** (`resources/openApi/api-v1.json`) is the reference: field names, types and status
  codes come from it, and the contract tests check them against it

### Running Quality Assurance

```bash
# Everything CI runs
test -z "$(gofmt -l .)"         # The formatting: lists the files gofmt would change, so must print nothing
go vet ./...                    # Vet the library, the tests and the examples
golangci-lint run               # The linters of .golangci.yml
go mod tidy -diff               # go.mod is tidy (Go 1.23 or later)
go test -race ./...             # The tests, with the race detector
go build ./...                  # The examples compile
govulncheck ./...               # Vulnerabilities of the standard library the code calls

# And:
golangci-lint fmt               # Fix the formatting (gofmt, goimports)
go test -cover .                # The tests with the coverage of the library (the examples have no tests)
```

## Testing

- Write tests for all new functionality, in the `*_test.go` file of the code it covers
- Ensure existing tests continue to pass: CI runs them on Go 1.22 to 1.27, with the race detector
- Use meaningful test names and assertions
- Cover both success and error scenarios
- Build fixtures from what the API really sends (`helpers_test.go`, the OpenAPI examples)

No test may reach the network: services run against a stub HTTP client, the HTTP layer against local servers.
[TESTING.md](TESTING.md) describes the suite.

## Pull Request Process

1. Ensure all QA checks pass (see above)
2. Update the README.md if needed
3. Update CHANGELOG.md following [Keep a Changelog](https://keepachangelog.com/) format
4. Create a Pull Request with:
   - Clear title and description
   - Reference to any related issues
   - Examples if applicable

## Releasing

A Go module is not uploaded anywhere: the Go module proxy and [pkg.go.dev](https://pkg.go.dev/github.com/calisero/calisero-go)
read its versions from the repository's tags.

1. Set the new version in `version.go` (`Version`, sent in the `User-Agent` header): a test checks that it matches
   the latest version of CHANGELOG.md
2. Move the changes from `[Unreleased]` to the new version in CHANGELOG.md
3. Merge, then publish a GitHub release whose tag is the version with a `v`: `v1.2.3` (Go reads no other form)

The `publish` job of the CI workflow then checks that the tag matches `Version`, once every other check passed, and
fetches the version through `proxy.golang.org`, which makes it available to `go get` and lists it on pkg.go.dev at
once.

A published version cannot change: the Go checksum database keeps the content of every tag it has seen, so never move
or delete a released tag. To withdraw a broken version, release a new one whose `go.mod` holds a `retract` directive
for it.

A version 2 or later with breaking changes needs a new module path, `github.com/calisero/calisero-go/v2`, as Go's
semantic import versioning requires.

## Code Style

`gofmt` formats the code and `golangci-lint` lints it (`.golangci.yml`). Key points:

- Idiomatic Go: a context first in every call, errors returned rather than panics, options as functions
- Every exported identifier documented, its comment starting with its name
- One resource and its service per file (`messages.go`, `optouts.go`…), with its tests next to it
- Errors that say what is wrong and with which field, prefixed with `calisero: `, and wrapped with `%w`
- Comments that explain why, not what

## Commit Messages

Use clear, descriptive commit messages:

```
feat: add support for custom HTTP headers
fix: handle null values in message responses
docs: update README with new examples
test: add integration tests for opt-out service
```

## Reporting Issues

When reporting issues, please include:

- Go version (`go version`)
- Library version
- Minimal code example reproducing the issue
- Expected vs actual behavior
- Any error messages, with the trace ID (`TraceID` of the `*calisero.APIError`)

## Security

If you discover a security vulnerability, please send an email to support@calisero.ro instead of using the issue tracker.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
