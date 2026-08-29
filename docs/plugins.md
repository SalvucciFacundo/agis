# External Plugin System Guide

AGIS provides an extensible **Plugin Architecture** (`internal/plugins`) that allows developers to add custom tools and specialized skills to the agent without modifying the core Go binary.

Plugins are distributed as directory bundles placed in `$AGIS_HOME/plugins/<plugin-name>/` containing a `plugin.json` manifest.

---

## Plugin Directory Structure

```text
~/.agis/plugins/
├── state.json                  # Managed state (enabled/disabled plugins)
├── weather/                    # Example: weather plugin
│   ├── plugin.json             # Manifest definition
│   ├── bin/                    # Executable or script entrypoint
│   │   └── weather-cli
│   └── skills/                 # Bundled Markdown skills
│       └── weather-forecast.md
└── github-tools/               # Example: GitHub integration plugin
    ├── plugin.json
    └── index.js
```

---

## 1. Plugin Manifest Schema (`plugin.json`)

Every plugin directory must contain a valid `plugin.json` adhering to this schema:

```json
{
  "name": "weather",
  "version": "1.0.0",
  "description": "Real-time weather forecast and atmospheric condition tools",
  "entrypoint": "bin/weather-cli",
  "tools": [
    {
      "name": "get_weather",
      "description": "Fetch current weather and 3-day forecast for a given city",
      "parameters": {
        "type": "object",
        "properties": {
          "city": {
            "type": "string",
            "description": "City name or zip code"
          },
          "unit": {
            "type": "string",
            "enum": ["celsius", "fahrenheit"],
            "description": "Temperature unit"
          }
        },
        "required": ["city"]
      }
    }
  ],
  "skills": [
    "skills/weather-forecast.md"
  ],
  "permissions": [
    "network:outbound"
  ]
}
```

### Manifest Fields:
- **`name`** *(string, required)*: Unique identifier matching regex `^[a-z0-9-_]+$`.
- **`version`** *(string, required)*: Semantic version string (e.g. `1.0.0`).
- **`description`** *(string, optional)*: Human-readable summary.
- **`entrypoint`** *(string, optional)*: Relative path to the executable or script called when a tool is invoked.
- **`tools`** *(array, optional)*: Tool definitions with names, descriptions, and JSON Schema parameters.
- **`skills`** *(array of strings, optional)*: Relative paths to Markdown skill files to load into the Skill Hub.
- **`permissions`** *(array of strings, optional)*: Permission categories required by the plugin.

---

## 2. Tool Execution Protocol (Stdio Bridge)

When the model calls a tool provided by a plugin:
1. AGIS executes the plugin's `entrypoint` via standard command execution:
   ```bash
   <entrypoint> --tool <tool_name> '<json_arguments>'
   ```
2. The script or executable reads the arguments, processes the request, and outputs the result to **`stdout`**.
3. AGIS captures `stdout` and returns it directly to `core.Brain.Step` as the tool response.
4. If the script exits with non-zero status, `stderr` is captured and returned as a tool failure.

### Example Python Entrypoint (`bin/weather-cli`):
```python
#!/usr/bin/env python3
import sys
import json

def get_weather(args):
    city = args.get("city", "Unknown")
    return f"Weather in {city}: 22°C, Sunny, Humidity 45%"

if __name__ == "__main__":
    tool_name = sys.argv[2] if len(sys.argv) > 2 else ""
    raw_args = sys.argv[3] if len(sys.argv) > 3 else "{}"
    args = json.loads(raw_args)

    if tool_name == "get_weather":
        print(get_weather(args))
    else:
        sys.stderr.write(f"Unknown tool: {tool_name}\n")
        sys.exit(1)
```

---

## 3. Managing Plugins via CLI

The `agis plugins` subcommands provide lifecycle control:

```bash
# List all discovered plugins and their status
agis plugins list

# Inspect detailed manifest information
agis plugins inspect weather

# Enable an installed plugin
agis plugins enable weather

# Disable a plugin (unregisters tools and skills dynamically)
agis plugins disable weather
```

### State Persistence:
Plugin activation state (`enabled`/`disabled`) is persisted automatically in `$AGIS_HOME/plugins/state.json`, preserving your configuration across application restarts without mutating plugin files.

---

## 4. Policy Guard Integration

Plugin tool executions are bound to the `core.ToolRunner` interface under backend prefix `plugin-<plugin_name>`. All plugin executions pass through AGIS's `PolicyGuard`, allowing you to set persistent allow or deny rules:

```bash
# Allow a plugin tool permanently
agis policy set "plugin-weather:get_weather" allow

# Deny a risky plugin action
agis policy set "plugin-shell:execute" deny
```
