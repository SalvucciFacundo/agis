# Tools Backends Spec

## Purpose

Execution backends for tool calls. Local shell, Docker ephemeral containers and SSH remotes share the ToolRunner port; the registry selects enabled backends at startup with graceful degradation when binaries are missing.

## Requirements

tools-backends (NEW)

### Requirement: Tool port and registry
A Tool MUST expose Name, Description, and Execute(ctx, args, guard). The registry MUST register enabled backends' tools at startup; a backend whose binary is unavailable MUST be skipped with a warning when enabled, never fatal.

#### Scenario: Docker missing degrades
- GIVEN docker.enabled true and no docker binary
- THEN startup continues without docker tools and warns

### Requirement: Local backend
The local backend MUST execute shell commands and filesystem reads/writes on the host. In sandbox posture it MUST refuse destructive classes (writes outside allowlist, network commands, package removal) even when patterns would otherwise match.

#### Scenario: Sandbox blocks network command
- GIVEN sandbox posture
- WHEN `curl example.com` executes locally
- THEN it is denied

### Requirement: Docker backend
The docker backend MUST run each command inside an ephemeral container from a configured image, requiring the docker binary. Container teardown MUST occur after execution even on failure.

#### Scenario: Ephemeral execution
- WHEN a command runs on the docker backend
- THEN no container survives the call

### Requirement: SSH backend
The ssh backend MUST execute commands on a configured host (host, user, key path) per call, surfacing connection failures as tool errors.

#### Scenario: Connection failure surfaces
- GIVEN an unreachable host
- THEN Execute returns an error and the audit logs a failed attempt
