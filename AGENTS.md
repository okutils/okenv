# AGENTS.md

## Rules

- Use only the Go standard library, keeping the project free of external dependencies.
- Use Go standard library APIs directly and preserve their behavior on the target operating system. Add custom business rules or validation only when explicitly requested by the user; do not infer or introduce additional restrictions on your own.
- Validate changes through end-to-end tests in a temporary directory outside the repository. Keep all end-to-end test files there, and omit unit tests.
- Run `okenv --run format` from the repository root after completing each development task to format the code.
