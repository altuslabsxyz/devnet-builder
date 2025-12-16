# Feature Specification: Binary Replace Command

**Feature Branch**: `001-binary-replace-command`
**Created**: 2025-12-16
**Status**: Draft
**Input**: User description: "Add a command to replace the stabled binary without going through governance upgrade process"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Replace Binary with Version Tag (Priority: P1)

Developer wants to quickly test a new release version on their running devnet without going through the full governance upgrade process.

**Why this priority**: This is the core use case - quickly swapping to a tagged release version for testing. Most common scenario for developers testing new releases.

**Independent Test**: Can be fully tested by running `devnet-builder replace --version v1.2.0` on a running devnet and verifying nodes restart with new binary.

**Acceptance Scenarios**:

1. **Given** a running devnet with version v1.1.3, **When** user runs `devnet-builder replace --version v1.2.0`, **Then** the system builds v1.2.0 binary, stops all nodes, replaces the binary at `~/.stable-devnet/bin/stabled`, restarts all nodes, and updates metadata to reflect new version.

2. **Given** a running devnet, **When** user runs `devnet-builder replace --version v1.2.0 -y`, **Then** the replacement proceeds without confirmation prompt.

3. **Given** a running devnet, **When** user runs `devnet-builder replace --version v1.2.0` and types "n" at confirmation, **Then** the operation is cancelled and no changes are made.

---

### User Story 2 - Replace Binary with Branch (Priority: P2)

Developer wants to test a feature branch on their running devnet before it's merged or tagged.

**Why this priority**: Common development workflow - testing feature branches before merge. Second most common scenario after tagged releases.

**Independent Test**: Can be tested by running `devnet-builder replace --version feat/my-feature` and verifying the branch is built and deployed.

**Acceptance Scenarios**:

1. **Given** a running devnet, **When** user runs `devnet-builder replace --version feat/gas-waiver`, **Then** the system clones the stable repo, checks out the branch, builds the binary via goreleaser, and deploys it.

2. **Given** a running devnet, **When** user runs replace with a non-existent branch, **Then** the system displays an error message and no changes are made.

---

### User Story 3 - Replace Binary with Commit Hash (Priority: P3)

Developer wants to test a specific commit on their running devnet for debugging purposes.

**Why this priority**: Less common but valuable for debugging - pinpointing which commit introduced an issue.

**Independent Test**: Can be tested by running `devnet-builder replace --version abc1234` with a valid short commit hash.

**Acceptance Scenarios**:

1. **Given** a running devnet, **When** user runs `devnet-builder replace --version abc1234def567`, **Then** the system checks out the specific commit, builds the binary, and deploys it.

---

### User Story 4 - JSON Output Mode (Priority: P3)

Developer wants to integrate replace command into CI/CD scripts with structured output.

**Why this priority**: Automation support - allows programmatic parsing of replace results.

**Independent Test**: Can be tested by running `devnet-builder replace --version v1.2.0 --json -y` and parsing the JSON output.

**Acceptance Scenarios**:

1. **Given** a running devnet, **When** user runs `devnet-builder replace --version v1.2.0 --json -y`, **Then** the output is valid JSON containing status, previous_version, new_version, commit_hash, and binary_path fields.

2. **Given** a replace operation that fails, **When** `--json` flag is used, **Then** the output is JSON with status="error" and an error message.

---

### Edge Cases

- What happens when no devnet exists? → Return error: "no devnet found at {home}"
- What happens when devnet is already stopped? → Still proceed with build and binary replacement
- What happens when build fails? → Return error before any node operations, no state change
- What happens when node restart fails? → Return partial success with failed nodes listed
- What happens when chain state is incompatible with new binary? → Nodes fail to start, user sees error in status/logs
- What happens when disk is full? → Build fails with goreleaser error
- What happens when stable repo clone fails? → Build fails with network/git error

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST require `--version` flag specifying the target ref (tag, branch, or commit)
- **FR-002**: System MUST build the binary using goreleaser via the existing builder package
- **FR-003**: System MUST stop all running nodes before replacing the binary
- **FR-004**: System MUST copy the built binary to `{homeDir}/bin/stabled`
- **FR-005**: System MUST restart all nodes after binary replacement
- **FR-006**: System MUST update metadata.json with the new current_version
- **FR-007**: System MUST display confirmation prompt before proceeding (unless `-y` flag)
- **FR-008**: System MUST support `--json` flag for structured JSON output
- **FR-009**: System MUST support `--health-timeout` flag for configuring restart health check duration
- **FR-010**: System MUST work with both local and docker execution modes
- **FR-011**: System MUST preserve chain state during binary replacement

### Key Entities

- **ReplaceResult**: Represents the outcome of replace operation (status, previous_version, new_version, commit_hash, binary_path, error)
- **DevnetMetadata**: Updated with new current_version after successful replacement

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Replace command completes full cycle (build → stop → replace → start) within the health-timeout period (default 5 minutes)
- **SC-002**: After successful replace, `devnet-builder status` shows new version in current_version field
- **SC-003**: After successful replace, all nodes are running and producing blocks
- **SC-004**: Replace command with `--json` flag returns valid parseable JSON in all scenarios (success, partial, error)
- **SC-005**: Chain state (accounts, balances, history) is preserved after binary replacement
