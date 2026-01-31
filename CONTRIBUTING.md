# Contributing to ravenbot

First off, thanks for taking the time to contribute! 🎉

The following is a set of guidelines for contributing to ravenbot. These are mostly guidelines, not rules. Use your best judgment, and feel free to propose changes to this document in a pull request.

## Code of Conduct

This project and everyone participating in it is governed by the [ravenbot Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

This section guides you through submitting a bug report. Following these guidelines helps maintainers and the community understand your report, reproduce the behavior, and find related reports.

- **Use a clear and descriptive title** for the issue to identify the problem.
- **Describe the exact steps to reproduce the problem** in as many details as possible.
- **Provide specific examples** to demonstrate the steps.
- **Describe the behavior you observed** after following the steps and point out what exactly is the problem with that behavior.
- **Explain which behavior you expected to see instead and why.**

### Suggesting Enhancements

This section guides you through submitting an enhancement suggestion, including completely new features and minor improvements to existing functionality.

- **Use a clear and descriptive title** for the issue to identify the suggestion.
- **Provide a step-by-step description of the suggested enhancement** in as many details as possible.
- **Explain why this enhancement would be useful** to most ravenbot users.

### Pull Requests

1.  Fork the repo and create your branch from `main`.
2.  If you've added code that should be tested, add tests.
3.  If you've changed APIs, update the documentation.
4.  Ensure the test suite passes (`make test`).
5.  Make sure your code lints (`make lint`).
6.  Issue that pull request!

## Local Development

To set up your local environment for development, we have provided a script to help you get started.

### Prerequisites

- Go 1.25+
- Docker (optional, but recommended)
- Chromium/Chrome (for the browser tool)
- Node.js & npm (for MCP servers)

### Setup

Run the setup script:

```bash
./scripts/setup_local.sh
```

This script will:
1.  Check for necessary dependencies.
2.  Install Go dependencies.
3.  Set up your `.env` file from the example.
4.  Create necessary data directories.

### Running

You can run the bot using the Makefile or the run script:

```bash
./scripts/run_local.sh
```

or

```bash
make build
./ravenbot
```

## Styleguides

### Git Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

### Go Styleguide

- We follow standard Go conventions.
- Run `make fmt` and `make lint` before committing.
