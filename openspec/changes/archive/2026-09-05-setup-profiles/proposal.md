# SDD Proposal: Setup Wizard & Multi-Profile Management

## Intent
To significantly improve the onboarding experience for new users through an interactive setup wizard, and to unlock advanced workflows by introducing isolated multi-profile management. Users will be able to easily configure AGIS from scratch, validate their LLM credentials immediately, and maintain distinct agent identities, memory databases, and tool configurations for different contexts (e.g., `work`, `personal`, `research`).

## Scope
1. **Interactive Setup Wizard (`agis setup` / `agis init`)**
   - Provide an interactive terminal wizard (using `charmbracelet/bubbletea` or standard prompt loops) to configure AGIS.
   - Prompt for LLM Provider (Ollama, OpenAI, OpenRouter, Anthropic-compatible), API key, model selection, and tool defaults.
   - Execute a live connectivity probe to validate credentials and provider reachability before persisting.
   - Support a `--non-interactive` flag combined with configuration flags for automated deployments.
   - Ensure the generated configuration file (`config.yaml`) is written atomically with strict `0600` permissions to protect API keys.

2. **Multi-Profile Management (`agis profile`)**
   - Introduce profile isolation under `$AGIS_HOME/profiles/<name>/`.
   - Each profile will encapsulate its own:
     - `config.yaml` (Overrides/specifics)
     - `agis.db` (Conversations, vector embeddings)
     - `SOUL.md` (Persona and identity)
     - `skills/` (Profile-specific abilities)
     - `policy.yaml` (Security and tool policies)
   - Add a global `--profile <name>` flag and support for the `$AGIS_PROFILE` environment variable across all AGIS CLI subcommands.
   - Provide explicit subcommands:
     - `agis profile list`: Enumerate available profiles.
     - `agis profile create <name> [--clone <source>]`: Scaffold a new profile.
     - `agis profile show [name]`: Print current profile context and configuration paths.
     - `agis profile switch/use <name>`: Persist a global default profile (e.g., via a `.active_profile` symlink or pointer file).
     - `agis profile delete <name>`: Safely remove a profile and its state.

3. **Observability & Diagnostics**
   - Extend the existing `internal/doctor` package.
   - Add health checks to report the active profile, resolve configuration paths, and verify isolation boundaries (e.g., permission checks on profile directories and config files).

## Affected Areas
- **`cmd/agis` CLI Layer**: Addition of `setup` and `profile` subcommands. Modification of the `root.go` command to parse the global `--profile` flag early in the execution chain (`PersistentPreRunE`).
- **Configuration & Path Resolution**: Refactor logic defining `$AGIS_HOME` to dynamically append `profiles/<name>` when a profile is active.
- **Database & Storage Initialization**: SQLite/Vector DB connections must defer instantiation until the active profile path is resolved.
- **Diagnostic/Doctor Module**: New checks specifically for directory structures and file permissions (`0600`).

## Risks & Mitigations
- **Risk:** Existing commands might bypass profile-aware path resolution, writing `agis.db` to the global `$AGIS_HOME` unintentionally.
  **Mitigation:** Centralize path resolution in a single package (e.g., `internal/paths` or `internal/config`) that strictly enforces the active profile context. No component should construct paths directly using `os.Getenv("AGIS_HOME")`.
- **Risk:** Security regression exposing API keys.
  **Mitigation:** File creation functions will enforce `0600` (`os.FileMode(0600)`) strictly, checked via unit tests and reported by the doctor probe.
- **Risk:** Dropped connections during the connectivity probe causing wizard hang.
  **Mitigation:** Wrap the LLM validation probe in a strict `context.WithTimeout` (e.g., 5-10 seconds) to ensure graceful failure.

## Success Criteria
- A user can run `agis setup` on a fresh machine, answer prompts, and have a fully functioning, connected AGIS instance without manually editing YAML.
- An automated CI/CD environment can execute `agis setup --provider openai --model gpt-4o --api-key $KEY --non-interactive` successfully.
- A user can run `agis profile create work` and immediately run `agis --profile work session`, resulting in a completely distinct memory database and persona from their default profile.
- `agis doctor` outputs a passing "Profile Health" check validating the `0600` permissions on the active profile's `config.yaml`.
- All newly added CLI commands adhere to Viper/Cobra best practices, specifically checking `SilenceUsage` and providing correct exit codes.
