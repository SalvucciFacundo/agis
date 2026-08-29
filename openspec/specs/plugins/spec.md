# Plugin Manager Spec

## Purpose

Manage external plugins defined via `plugin.json` manifests, bridging plugin tools to the tool registry and policy guard, and registering declared skill files to the skill hub with atomic state persistence.

## Requirements

### Requirement AGIS-M6-PLG-001: Plugin Manifest Schema (`plugin.json`)
The system MUST recognize external plugins placed in `$AGIS_HOME/plugins/<plugin_name>/` containing a `plugin.json` manifest. The manifest MUST validate against the following JSON schema:
- `name` (string, required): Unique lowercase identifier matching `^[a-z0-9-_]+$`.
- `version` (string, required): Semantic version string (e.g. `"1.0.0"`).
- `description` (string, optional): Short summary of plugin capabilities.
- `entrypoint` (string, optional): Executable or script name relative to plugin root for CLI tool bridges.
- `tools` (array of objects, optional): Tool definitions with `name`, `description`, and `parameters`.
- `skills` (array of strings, optional): Skill markdown file names located inside the plugin directory.
- `permissions` (array of strings, optional): Declared permission categories requested by the plugin.

#### Scenario: Valid plugin manifest parses successfully
- GIVEN a plugin directory containing a compliant `plugin.json`
- WHEN the Plugin Manager reads the manifest
- THEN all metadata, tools, and skills are extracted into memory structures without errors

#### Scenario: Malformed manifest rejected
- GIVEN a `plugin.json` missing the required `name` or `version` field
- WHEN the Plugin Manager inspects the directory
- THEN loading fails with a schema validation error and the plugin is marked invalid

### Requirement AGIS-M6-PLG-002: Plugin Manager Lifecycle (`Load`, `List`, `Enable`, `Disable`)
The Plugin Manager (`internal/plugins/`) MUST manage the discovery and lifecycle of plugins:
- `Load(dir string) error`: Scans the plugin root directory and loads all valid plugin manifests.
- `List() []PluginInfo`: Returns all discovered plugins, their status (`enabled` or `disabled`), version, and registered tools.
- `Enable(name string) error`: Activates a plugin and registers its tools and skills into AGIS registries.
- `Disable(name string) error`: Deactivates a plugin and unregisters its tools and skills.
- State (`enabled`/`disabled`) MUST persist across restarts in `$AGIS_HOME/plugins/state.json` or `config.yaml`.

#### Scenario: Discovered plugin enabled dynamically
- GIVEN a discovered plugin `"weather"` in disabled state
- WHEN `Enable("weather")` is executed
- THEN the plugin status becomes `enabled`, its state is persisted, and its tools become available in the tool registry

#### Scenario: Disabling plugin removes tools
- GIVEN an enabled plugin `"weather"` providing tool `"get_weather"`
- WHEN `Disable("weather")` is executed
- THEN `"get_weather"` is deregistered from the active tool registry and unavailable for subsequent turns

### Requirement AGIS-M6-PLG-003: Plugin Tool and Skill Registration
When a plugin is enabled:
- Its declared tools MUST be registered with the AGIS Tool Registry, executing the plugin's `entrypoint` executable via JSON-RPC or standard stdin/stdout command interface with arguments and receiving structured JSON results.
- Its declared skills (`.md` files) MUST be registered with the AGIS Skill Hub.
- Plugin tool executions MUST pass through `PolicyGuard` under the standard execution guardrails.

#### Scenario: Brain calls a plugin tool
- GIVEN an enabled plugin providing tool `"github_search"`
- WHEN the model emits a tool call for `"github_search"` with arguments
- THEN the tool runner executes the plugin entrypoint with the arguments and returns the standard tool output to the brain
