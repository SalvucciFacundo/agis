# Proposal: config-cli

## Intent
Provide a built-in command-line interface (`agis config`) for inspecting and safely modifying the AGIS configuration without requiring manual edits to YAML files. This ensures type safety, preserves 0600 file permissions for security, and protects sensitive credentials from accidental terminal exposure.

## Scope
- New CLI root subcommand: `agis config`
- **Subcommands:**
  - `show [--json] [--reveal]`: Displays the active configuration. Masks API keys by default to prevent leaking credentials in terminal history.
  - `get <key>`: Retrieves a specific key's value using dot notation (e.g., `llm.provider`, `db.path`, `memory.recall_limit`).
  - `set <key> <value>`: Updates a specific configuration key. Enforces type validation (string, int, bool, duration), performs atomic writes, and strictly applies `0600` file permissions to secure sensitive data.
  - `path`: Prints the resolved absolute or relative configuration file path.
- **POSIX exit codes:** `0` (Success), `1` (Domain error, value not found, validation error), `2` (CLI flag or usage error).
- **Stream separation:** Output data strictly to `stdout`, errors/logs/diagnostics to `stderr`.

## Affected Areas
- Command line interface: `cmd/agis/` (creation of `config.go` and associated subcommand files).
- Configuration handling module (enhancements to read/write, parse dot notation, validate types, and apply file permissions).

## Risks
- **Data Corruption:** Writing the configuration file fails mid-flight. (Mitigated by atomic file renaming).
- **Security Exposure:** `0600` permissions are inadvertently bypassed depending on the OS, or `show` fails to mask new credential fields correctly.
- **Formatting Loss:** Rewriting the configuration via code may strip user comments from the YAML file.

## Rollback
- Revert the `cmd/agis/config*.go` additions.
- If a user config is corrupted, they can manually restore their configuration. 

## Success Criteria
- Users can view and manage `agis` configurations exclusively via CLI without a text editor.
- API keys remain masked during standard `show` commands.
- `set` prevents injecting invalid data types for known configuration keys.
- Configuration files are reliably locked down to `0600`.
- All subcommands follow standard Go CLI conventions (cobra/viper) and output cleanly to stdout/stderr.

## Proposal question round
To ensure this proposal aligns with product expectations and edge cases, please review these questions:
1. **Key validation:** Should `set` be strictly limited to predefined keys in the AGIS configuration struct, or can users add arbitrary dynamic keys?
2. **Comment preservation:** Is it acceptable if `set` rewrites the YAML file and strips existing manual comments, or is comment preservation a hard requirement?
3. **Precedence handling:** Should `agis config get` return the raw value currently saved in the file, or the "effective" value (which might be overridden by an Environment Variable)?
4. **Backup behavior:** Should `set` automatically create a `.bak` backup file of the previous configuration before applying atomic writes?
