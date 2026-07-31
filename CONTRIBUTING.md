# Contributing to Loom

Thank you for your interest in contributing to Loom! This document provides guidelines and instructions for contributing.

## Code of Conduct

Be respectful and professional. We're all here to build something great.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues. When creating a bug report, include:

- **Clear title** describing the issue
- **Steps to reproduce** the behavior
- **Expected behavior** vs actual behavior
- **Environment details** (OS, Go version, Docker version)
- **Logs** if applicable

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:

- **Clear title** and description
- **Use case** explaining why this would be useful
- **Proposed solution** if you have one
- **Alternatives considered**

### Pull Requests

1. **Fork the repo** and create your branch from `main`
2. **Follow the coding style** of the project
3. **Add tests** if you're adding code that should be tested
4. **Update documentation** if you're changing functionality
5. **Write a clear commit message**
6. **Ensure tests pass** before submitting

## Development Setup

```bash
# Clone your fork
git clone https://github.com/your-username/Loom.git
cd Loom

# Set up environment
cp .env.example .env
# Edit .env with your configuration

# Start services
docker-compose up -d

# Build services
cd trigger-api && go build ./...
cd orchestration-engine && go build ./...
cd node-worker-pool && go build ./...
```

## Coding Guidelines

### Go Code Style

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable names
- Comment exported functions and types
- Keep functions small and focused
- Handle errors explicitly

### Git Commit Messages

- Use present tense ("Add feature" not "Added feature")
- Use imperative mood ("Move cursor to..." not "Moves cursor to...")
- First line max 72 characters
- Reference issues and pull requests

Examples:
```
Add email counter to workflow runs table

- Adds email_dispatch_count column
- Implements atomic increment with limit check
- Updates orchestrator to enforce 100 email limit

Fixes #123
```

### Testing

- Write unit tests for new functionality
- Ensure existing tests pass
- Integration tests for end-to-end flows
- Test error cases

```bash
# Run tests
cd trigger-api && go test ./...
cd orchestration-engine && go test ./...
cd node-worker-pool && go test ./...
```

## Project Structure

```
loom/
├── trigger-api/          # REST API & webhook handling
├── orchestration-engine/ # Workflow orchestration
├── node-worker-pool/     # Node executors
├── shared/              # Shared contracts
├── docs/                # Documentation
├── examples/            # Example workflows
└── scripts/             # Utility scripts
```

## Adding a New Node Type

1. Create executor in `node-worker-pool/internal/nodes/`
2. Implement `Executor` interface
3. Register node type in `init()`
4. Add tests
5. Update documentation

Example:
```go
// nodes/slack.go
package nodes

import (
    "context"
    "encoding/json"
)

func init() {
    Register("SLACK", &SlackExecutor{})
}

type SlackExecutor struct {
    // ...
}

func (e *SlackExecutor) Execute(ctx context.Context, config json.RawMessage) (json.RawMessage, error) {
    // Implementation
}
```

## Database Migrations

When adding/modifying database schema:

1. Create migration files in `migrations/`
2. Name format: `XXX_description.up.sql` and `XXX_description.down.sql`
3. Update `query.sql` if adding queries
4. Run `sqlc generate`

## Documentation

- Update relevant docs in `docs/`
- Update README if adding features
- Include code examples
- Keep docs in sync with code

## Questions?

Open an issue or discussion on GitHub!

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
