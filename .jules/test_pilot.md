# Test Pilot Journal ✈️

## 2025-05-14 - Strengthening Reliability and Security

### Discovery: Untested Security Critical Path
Found that `internal/tools/shell.go` lacked tests for its command execution and argument sanitization logic. This is a critical security component that prevents shell injection.

### Fix: Robust Table-Driven Sanitization Tests
Implemented `internal/tools/shell_test.go` with a table-driven approach to verify that forbidden characters (`;`, `&`, `|`, etc.) are correctly rejected. Used `t.Parallel()` to ensure efficient execution.

### Discovery: Zero Coverage in DB Layer
The `internal/db` package had 0% coverage. Testing database operations usually requires complex setup, but SQLite's in-memory mode (`:memory:`) provides a perfect isolated environment for unit testing.

### Fix: In-Memory DB Unit Testing
Created `internal/db/repository_test.go` using `InitDB(":memory:")`. This allowed for 76.7% coverage of the database layer with zero external dependencies and high performance.

### Discovery: Race Condition with Shared Mocks and Parallel Tests
When implementing parallel subtests for `SystemRoleWrapper`, a data race was discovered because the mock object and its state (`lastReq`) were shared across subtests.

### Fix: Isolated Mock Instances
Instantiated the mock and wrapper *inside* each `t.Run` closure. This ensures that each parallel subtest has its own isolated state, preventing concurrent access and satisfying the race detector.

### Learnings
- Always use `t.Parallel()` for independent unit tests in Go.
- When using `t.Parallel()` with subtests, ensure all shared state (especially mocks) is moved into the subtest closure to avoid data races.
- Table-driven tests are essential for validating complex parsers like `extractDDGURL`.
- In-memory SQLite is a powerful tool for testing Go repository layers without mocks.
