# Feature Specification: Complete E2E Test Suite for devnet-builder

**Feature Branch**: `feat/e2e-test-suite`
**Created**: 2025-12-27
**Status**: Draft
**Input**: User description: "senior golang 개발자이자 Cosmos SDK 전문가로써, 현재 devnet-builder 프로그램의 로직을 하나 하나 다 천천히 살피고, 버그가 발생하지 않도록 완벽하게 e2e test를 작성해. 모든 command 에 대해서 한 번 씩은 무조건 테스트를 하도록 하고, 시간이 오래걸리는 snapshot 다운로드해서 export 하는 로직은 mocking 해서 처리해. devnet-builder-main worktree를 기준으로 새로운 worktree를 만들고 작업해."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Core Command Lifecycle Testing (Priority: P1)

As a devnet-builder maintainer, I need comprehensive end-to-end tests for all core lifecycle commands (deploy, init, start, stop, destroy) to ensure the most critical workflows work correctly across different configurations and modes.

**Why this priority**: These commands represent the essential user journey - creating, managing, and cleaning up devnets. If these fail, the tool is unusable. They account for 80% of actual user interactions.

**Independent Test**: Can be fully tested by executing each command with various flag combinations and verifying filesystem state, process state, and command outputs. Delivers immediate value by catching critical bugs in the most-used functionality.

**Acceptance Scenarios**:

1. **Given** no existing devnet, **When** user runs `deploy` with default settings, **Then** devnet is created with 4 validators, all nodes are running, and status shows healthy
2. **Given** no existing devnet, **When** user runs `init` followed by `start`, **Then** devnet is initialized and started successfully with all nodes running
3. **Given** a running devnet, **When** user runs `stop`, **Then** all nodes are gracefully stopped within timeout and no processes remain
4. **Given** a stopped devnet, **When** user runs `start`, **Then** all nodes resume from previous state and chain continues from last block
5. **Given** a devnet exists, **When** user runs `destroy --force`, **Then** all devnet data is removed, processes are killed, and docker resources are cleaned up
6. **Given** a running devnet, **When** user runs `deploy` again, **Then** command fails with clear error message about existing devnet
7. **Given** no devnet, **When** user runs `start`, **Then** command fails with clear error message about missing configuration

---

### User Story 2 - Monitoring and Diagnostics Testing (Priority: P2)

As a developer debugging devnet issues, I need reliable tests for monitoring commands (status, logs, node) to ensure I can always inspect and diagnose the state of running devnets.

**Why this priority**: While not critical for basic operation, these commands are essential for troubleshooting and development workflows. They are heavily used during debugging sessions.

**Independent Test**: Can be fully tested by deploying a test devnet and verifying status output, log streaming, and individual node control. Delivers value by ensuring developers can effectively debug issues.

**Acceptance Scenarios**:

1. **Given** a running devnet, **When** user runs `status`, **Then** output shows chain ID, block height, all node statuses, and overall health summary
2. **Given** a running devnet, **When** user runs `status --json`, **Then** output is valid JSON with all expected fields
3. **Given** a running devnet, **When** user runs `logs --follow node0`, **Then** logs stream continuously until interrupted
4. **Given** a running devnet, **When** user runs `logs --tail 50`, **Then** exactly 50 most recent log lines are displayed
5. **Given** a running devnet, **When** user runs `node stop 0` then `node start 0`, **Then** node0 is stopped and restarted while other nodes continue running
6. **Given** a stopped devnet, **When** user runs `status`, **Then** output indicates devnet is stopped with clear state information
7. **Given** no devnet, **When** user runs `status`, **Then** command outputs clear message indicating no devnet exists

---

### User Story 3 - Advanced Workflow Testing (Priority: P2)

As a blockchain developer, I need tests for advanced commands (upgrade, replace, export, build, reset) to ensure complex workflows like software upgrades and state management work reliably.

**Why this priority**: These commands are used less frequently but are critical for specific workflows like testing upgrades, migrating state, and recovery scenarios. Failures here can block important development work.

**Independent Test**: Can be tested independently for each workflow (upgrade path, export/import, reset scenarios) with mocked external dependencies. Delivers value by validating complex orchestration logic.

**Acceptance Scenarios**:

1. **Given** a running devnet, **When** user runs `upgrade --name v2-upgrade --binary /path/to/new-binary`, **Then** upgrade proposal is submitted, voted on, and executed at correct height
2. **Given** a running devnet, **When** user runs `export`, **Then** current blockchain state is exported to file with metadata
3. **Given** an exported genesis file, **When** user runs `build --validators 4 --output /tmp/test-devnet`, **Then** new devnet is built from exported state
4. **Given** a running devnet, **When** user runs `reset --force`, **Then** chain data is reset while preserving keys and configuration
5. **Given** a running devnet, **When** user runs `reset --hard --force`, **Then** all data including keys is removed
6. **Given** a running devnet, **When** user runs `replace --version v1.2.3 --yes`, **Then** nodes are stopped, binary is replaced, and nodes restart with new binary
7. **Given** a running devnet, **When** user runs `export-keys --type validators`, **Then** all validator mnemonics and addresses are exported to JSON

---

### User Story 4 - Configuration and Utility Testing (Priority: P3)

As a user setting up devnet-builder for the first time, I need tests for configuration and utility commands (config, cache, versions, networks) to ensure the tool can be properly configured and maintained.

**Why this priority**: These are supporting commands that enhance usability but aren't critical to core functionality. They are important for user experience but failures here don't prevent basic devnet operations.

**Independent Test**: Can be tested independently by verifying config management, cache operations, and informational commands return correct data. Delivers value by ensuring smooth onboarding and maintenance.

**Acceptance Scenarios**:

1. **Given** no config file exists, **When** user runs `config init`, **Then** sample config.toml is created with default values
2. **Given** a config file exists, **When** user runs `config show`, **Then** all configuration settings are displayed
3. **Given** configuration exists, **When** user runs `config set network testnet`, **Then** network setting is updated and persisted
4. **Given** binaries are cached, **When** user runs `cache list`, **Then** all cached binaries are listed with versions and sizes
5. **Given** cache has old binaries, **When** user runs `cache clean --force`, **Then** all cached binaries are removed
6. **Given** network configuration, **When** user runs `versions --list`, **Then** available versions from GitHub releases are displayed
7. **Given** multiple network modules, **When** user runs `networks`, **Then** all registered networks with metadata are listed
8. **Given** any context, **When** user runs `version`, **Then** devnet-builder version and build info are displayed

---

### User Story 5 - Multi-Mode and Multi-Configuration Testing (Priority: P2)

As a devnet operator, I need tests covering different execution modes (docker vs local), validator counts (1-100), and network types (mainnet vs testnet) to ensure the tool works across all supported configurations.

**Why this priority**: Mode and configuration variations are common in real usage. Docker mode is preferred for production, while local mode is essential for debugging. Testing both prevents mode-specific bugs.

**Independent Test**: Can be tested by running the same command sequences in different modes and configurations. Delivers value by ensuring consistency across deployment options.

**Acceptance Scenarios**:

1. **Given** no devnet, **When** user runs `deploy --mode docker --validators 10`, **Then** 10 docker containers are started with proper networking
2. **Given** no devnet, **When** user runs `deploy --mode local --validators 4`, **Then** 4 local processes are started with proper ports
3. **Given** no devnet, **When** user runs `deploy --network testnet`, **Then** devnet is created using testnet snapshot and configuration
4. **Given** a docker devnet exists, **When** user runs `stop` then `start --mode local`, **Then** command fails or switches mode with warning
5. **Given** no devnet, **When** user runs `deploy --validators 100 --mode docker`, **Then** 100 validators are deployed successfully
6. **Given** no devnet, **When** user runs `deploy --validators 5 --mode local`, **Then** command fails with error about local mode validator limit (max 4)
7. **Given** docker devnet running, **When** user runs `status`, **Then** output shows docker container IDs and network info
8. **Given** local devnet running, **When** user runs `status`, **Then** output shows process IDs and port allocations

---

### User Story 6 - Error Handling and Edge Cases (Priority: P1)

As a user, I need tests that verify proper error handling and edge cases to ensure the tool fails gracefully with helpful error messages when things go wrong.

**Why this priority**: Error handling is critical for user experience and debugging. Poor error messages lead to support burden and user frustration. This is nearly as important as happy path testing.

**Independent Test**: Can be tested by deliberately triggering error conditions and validating error messages and exit codes. Delivers value by preventing confusing failures and improving troubleshooting.

**Acceptance Scenarios**:

1. **Given** insufficient disk space, **When** user runs `deploy`, **Then** command fails early with clear error about disk space
2. **Given** docker is not running, **When** user runs `deploy --mode docker`, **Then** command fails with clear error about docker availability
3. **Given** invalid validator count, **When** user runs `deploy --validators 0`, **Then** command fails with validation error
4. **Given** conflicting flags, **When** user runs `deploy --image custom:tag --version v1.2.3`, **Then** command fails explaining flag conflict
5. **Given** a devnet is running, **When** user runs `destroy` without --force, **Then** user is prompted for confirmation
6. **Given** network timeout during snapshot download, **When** download fails, **Then** command retries or fails with clear error
7. **Given** corrupted cache file, **When** cache is read, **Then** file is ignored/removed and operation continues or fails gracefully
8. **Given** port conflicts exist, **When** user runs `deploy --mode local`, **Then** command fails with clear error about which ports are in use

---

### Edge Cases

- What happens when snapshot download is interrupted mid-transfer?
- How does system handle running out of disk space during genesis export?
- What happens when a node crashes immediately after start during health check?
- How does system handle corrupted metadata files?
- What happens when user tries to upgrade to same version already running?
- How does system handle docker network conflicts with existing networks?
- What happens when user runs multiple devnet-builder commands simultaneously?
- How does system handle binary cache corruption or missing files?
- What happens when node ports are already in use by other processes?
- How does system handle invalid or malformed configuration files?
- What happens when GitHub API rate limit is hit during version listing?
- How does system handle permissions issues writing to home directory?

## Requirements *(mandatory)*

### Functional Requirements

#### Core Test Infrastructure

- **FR-001**: Test suite MUST be able to execute all 23 main commands and 9 subcommands (32 total test targets)
- **FR-002**: Test suite MUST support running tests in isolated environments to prevent interference between tests
- **FR-003**: Test suite MUST clean up all resources (processes, files, docker containers/networks) after each test
- **FR-004**: Test suite MUST provide mocked snapshot download to avoid long-running network operations
- **FR-005**: Test suite MUST validate command exit codes (0 for success, non-zero for failure)
- **FR-006**: Test suite MUST validate both text and JSON output formats where applicable
- **FR-007**: Test suite MUST be able to run in both docker and local execution modes
- **FR-008**: Test suite MUST verify filesystem state changes (files created/modified/deleted)
- **FR-009**: Test suite MUST verify process state (containers/processes running/stopped)
- **FR-010**: Test suite MUST verify docker resource state (networks created/removed, ports allocated/released)

#### Command Coverage

- **FR-011**: Test suite MUST test `deploy` command with: default flags, custom validator counts (1-100 docker, 1-4 local), both networks (mainnet/testnet), both modes (docker/local)
- **FR-012**: Test suite MUST test `init` command followed by `start` to verify two-phase deployment
- **FR-013**: Test suite MUST test `stop` command graceful shutdown and force kill timeout scenarios
- **FR-014**: Test suite MUST test `destroy` command with and without cache cleanup option
- **FR-015**: Test suite MUST test `status` command output format and health check information
- **FR-016**: Test suite MUST test `logs` command with follow mode, tail options, and specific node targeting
- **FR-017**: Test suite MUST test `node start/stop/logs` subcommands for individual node control
- **FR-018**: Test suite MUST test `upgrade` command workflow including proposal submission, voting, and binary swap
- **FR-019**: Test suite MUST test `replace` command for hard binary replacement without governance
- **FR-020**: Test suite MUST test `export` command and verify exported genesis file integrity
- **FR-021**: Test suite MUST test `build` command creating devnet from exported genesis
- **FR-022**: Test suite MUST test `reset` command both soft (preserve keys) and hard (remove all) modes
- **FR-023**: Test suite MUST test `export-keys` command output format and key filtering
- **FR-024**: Test suite MUST test `config` subcommands (init, show, set) for configuration management
- **FR-025**: Test suite MUST test `cache` subcommands (list, clean, info) for binary cache management
- **FR-026**: Test suite MUST test `versions` command listing GitHub releases
- **FR-027**: Test suite MUST test `networks` command listing available blockchain networks
- **FR-028**: Test suite MUST test `version` command displaying build information
- **FR-029**: Test suite MUST test `restart` command (stop + start sequence)

#### Workflow Coverage

- **FR-030**: Test suite MUST test full lifecycle workflow: deploy → status → logs → stop → start → destroy
- **FR-031**: Test suite MUST test upgrade workflow: deploy → upgrade → verify → destroy
- **FR-032**: Test suite MUST test export workflow: deploy → export → build → deploy-from-build → destroy
- **FR-033**: Test suite MUST test reset workflows: soft reset preserving keys, hard reset removing all data
- **FR-034**: Test suite MUST test multi-validator scenarios (10+ validators in docker mode)
- **FR-035**: Test suite MUST test mode switching behavior and validation

#### Error Handling

- **FR-036**: Test suite MUST verify error messages for missing prerequisites (docker not running, insufficient disk space)
- **FR-037**: Test suite MUST verify error messages for invalid flag combinations
- **FR-038**: Test suite MUST verify error messages for already-exists scenarios (deploy when devnet exists)
- **FR-039**: Test suite MUST verify error messages for not-found scenarios (start when no devnet)
- **FR-040**: Test suite MUST verify timeout handling in health checks and graceful shutdown
- **FR-041**: Test suite MUST verify port conflict detection and error reporting
- **FR-042**: Test suite MUST verify validator count validation (1-100 docker, 1-4 local)

#### Mocking and Performance

- **FR-043**: Test suite MUST mock snapshot download operations to complete tests in reasonable time
- **FR-044**: Test suite MUST mock GitHub API calls for version listing to avoid rate limits
- **FR-045**: Test suite MUST provide pre-generated test genesis files to avoid repeated export operations
- **FR-046**: Test suite MUST complete full test run in under 30 minutes on standard hardware
- **FR-047**: Test suite MUST support parallel test execution where possible to reduce total runtime

#### Test Quality

- **FR-048**: Each test MUST be idempotent (can be run multiple times without side effects)
- **FR-049**: Each test MUST have clear test names describing what is being tested
- **FR-050**: Each test MUST use assertion helpers for consistent error reporting
- **FR-051**: Test suite MUST provide setup and teardown hooks for resource management
- **FR-052**: Test suite MUST log all command executions and outputs for debugging failed tests
- **FR-053**: Test suite MUST support running individual tests or test groups for rapid iteration

### Key Entities

- **Test Context**: Represents isolated test environment with temporary directories, mock servers, and cleanup handlers
- **Mock Snapshot Server**: Simulates snapshot download endpoints with pre-generated test data
- **Mock GitHub API**: Simulates GitHub release API for version listing without rate limits
- **Test Devnet Instance**: Represents a deployed devnet under test with associated metadata and resources
- **Command Runner**: Executes devnet-builder commands with environment isolation and output capture
- **State Validator**: Verifies expected filesystem, process, and docker resource states
- **Cleanup Manager**: Tracks and releases all test resources (processes, containers, temporary files)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All 32 command targets have at least one passing E2E test
- **SC-002**: Test suite covers at least 90% of command flag combinations actually used in documentation
- **SC-003**: Full test suite completes in under 30 minutes on CI infrastructure
- **SC-004**: Each individual test completes in under 3 minutes (excluding deployment time)
- **SC-005**: No test leaves behind orphaned processes, containers, or temporary files after completion
- **SC-006**: Test suite achieves 0% false positive rate (no flaky tests that randomly fail)
- **SC-007**: Test failure messages clearly indicate which assertion failed and provide debugging context
- **SC-008**: Mocked snapshot downloads complete in under 5 seconds (vs 2-10 minutes for real downloads)
- **SC-009**: Test suite can be run in parallel with at least 4 concurrent tests without interference
- **SC-010**: All error handling tests verify both exit code AND error message content
- **SC-011**: Test coverage includes at least 3 multi-validator scenarios (4, 10, 50+ validators)
- **SC-012**: Test coverage includes both docker and local mode for all mode-applicable commands
- **SC-013**: 100% of tests clean up successfully on both pass and fail scenarios
- **SC-014**: Test suite documentation allows new contributor to run tests within 5 minutes of setup

## Assumptions

1. **Test Environment**: Tests assume availability of docker daemon and Go 1.23+ runtime
2. **Network Access**: Tests assume ability to create isolated docker networks (172.x.0.0/16 range)
3. **Port Availability**: Tests assume availability of ephemeral port ranges for local mode testing
4. **Disk Space**: Tests assume at least 10GB free disk space for test artifacts
5. **Mock Data**: Pre-generated test genesis files and snapshots are committed to repository
6. **Isolation**: Each test runs in isolated temporary directory under /tmp or system equivalent
7. **Cleanup Priority**: Test cleanup uses best-effort approach but prioritizes preventing resource leaks
8. **Docker Permissions**: Tests assume user has docker permissions (no sudo required)
9. **GitHub API**: Mock GitHub API provides static list of common versions for testing
10. **Binary Builds**: For tests requiring custom binaries, pre-built test binaries are used instead of compiling from source
11. **Health Checks**: Test health checks use shorter timeouts than production (30s vs 5m)
12. **Snapshot Format**: Mock snapshots use minimal valid snapshot format to reduce storage requirements

## Dependencies

1. **Go Testing Framework**: Standard Go testing package for test execution
2. **Docker**: Docker daemon for container mode testing
3. **Temporary Storage**: /tmp or equivalent for test artifacts
4. **Test Fixtures**: Pre-generated genesis files, snapshots, and configuration templates
5. **Mock Servers**: HTTP servers for mocking snapshot download and GitHub API
6. **Assertion Library**: testify/assert or similar for readable test assertions
7. **Cleanup Utilities**: Helper functions for killing processes, removing containers, cleaning directories

## Out of Scope

1. **Performance Benchmarking**: Tests focus on correctness, not performance optimization
2. **Load Testing**: Tests use minimal validator counts for speed, not stress testing
3. **Network Partition Testing**: Tests assume reliable network, no chaos engineering
4. **Security Testing**: Tests do not validate security properties of blockchain consensus
5. **Cross-Platform Testing**: Initial implementation focuses on Linux/macOS, Windows support is future work
6. **Legacy Version Testing**: Tests validate current version only, not backwards compatibility
7. **Plugin Testing**: Tests cover built-in networks only, not third-party plugins
8. **UI Testing**: Tests are CLI-only, no terminal UI interactions tested
9. **Concurrent Multi-Devnet**: Tests focus on single devnet at a time, not concurrent devnets
10. **Upgrade Compatibility**: Tests validate upgrade mechanics, not cross-version compatibility

## Notes

- Test implementation will use table-driven tests for flag combinations to maximize coverage with minimal code
- Mock implementations should be as minimal as possible while still validating command behavior
- Tests should fail fast on setup errors to avoid wasting CI time
- Each test failure should log full command output and relevant state for debugging
- Tests should use deterministic test mnemonics to enable reproducible test scenarios
- Docker mode tests may require elevated timeouts in CI environments with shared resources
