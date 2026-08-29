# Cron Scheduler Spec

## Purpose

Provide a background periodic job scheduler supporting 5-field cron expressions, interval macros, non-interactive execution through Brain.Step under sandbox policy, and delivery of responses to gateway notification targets.

## Requirements

### Requirement AGIS-M6-CRN-001: Cron Scheduler Engine
The system MUST provide a Cron Scheduler (`internal/cron/`) capable of executing periodic background tasks defined in `config.yaml`.
- The scheduler MUST support standard 5-field cron expressions (e.g. `"0 9 * * *"` or `"*/15 * * * *"`) and duration intervals (e.g. `"@every 1h"`).
- Invalid cron expressions MUST be rejected at config validation time before the scheduler starts.
- The scheduler MUST run in a background goroutine and shut down cleanly when its parent context is canceled.

#### Scenario: Scheduler parses and registers valid cron jobs
- GIVEN a config with two jobs: `"@every 30m"` and `"0 8 * * 1-5"`
- WHEN the cron engine initializes
- THEN both jobs are scheduled with their calculated next run timestamps

#### Scenario: Invalid cron expression fails validation
- GIVEN a job configured with schedule `"invalid-cron-format"`
- WHEN the cron scheduler attempts initialization
- THEN initialization returns a descriptive parsing error and the scheduler does not start

### Requirement AGIS-M6-CRN-002: Job Execution via Brain
When a cron job triggers, the scheduler MUST execute the job's configured prompt through `core.Brain.Step`:
- If `session_id` is configured, the job MUST bind to that session; otherwise, it MUST execute in an isolated ephemeral session named `cron:<job_name>`.
- The execution MUST run non-interactively under the `sandbox` policy guard with auto-deny on unapproved tool actions.
- Job execution outcomes (start time, duration, status, error if any) MUST be logged.

#### Scenario: Cron job executes prompt via Brain
- GIVEN a configured cron job named `"daily-summary"` with prompt `"Summarize pending tasks"`
- WHEN the job trigger time arrives
- THEN the scheduler invokes `Brain.Step` with the prompt, loads repository context, and produces a summary output

### Requirement AGIS-M6-CRN-003: Gateway Notification Delivery
A cron job MAY define an optional `target` block containing `adapter` (e.g. `"telegram"` or `"discord"`) and `recipient` (chat ID or channel ID). Upon successful job completion, the cron engine MUST forward the resulting brain response to the Gateway Multiplexer to deliver to the configured recipient. If no target is specified, the output MUST be written to the application log.

#### Scenario: Cron job output delivered to Telegram
- GIVEN a job configured with target adapter `"telegram"` and recipient `"123456789"`
- WHEN the cron job executes and generates text output
- THEN the scheduler invokes the Gateway Multiplexer `Send(ctx, "telegram", "123456789", output)` and the message is delivered to Telegram

#### Scenario: Cron job without target logs output
- GIVEN a job configured without a target block
- WHEN the cron job executes successfully
- THEN the output is recorded in the scheduler log and no gateway send is triggered
