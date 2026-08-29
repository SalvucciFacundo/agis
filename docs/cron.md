# Autonomous Cron Scheduler Guide

AGIS includes an autonomous background **Cron Scheduler Engine** (`internal/cron`) capable of running periodic tasks, checks, and summaries on a schedule without human intervention.

Jobs execute autonomous prompts through `core.Brain.Step`, maintain session memory, adhere to the `sandbox` policy guard, and can optionally deliver their responses directly to chat platforms (Telegram, Discord) or application logs.

---

## Configuration Schema

Cron jobs are configured under the `cron` block in `config.yaml`:

```yaml
cron:
  enabled: true
  jobs:
    # Example 1: Hourly system health check with Telegram alert
    - name: "daily-health"
      schedule: "@every 1h"
      prompt: "Check system load, pending tasks, and database integrity. Return a 3-line summary."
      session_id: "cron-health"           # Persistent session across executions
      target:
        adapter: "telegram"
        recipient: "123456789"            # Target Telegram chat ID

    # Example 2: Daily morning briefing at 08:30 AM Monday through Friday
    - name: "morning-briefing"
      schedule: "30 8 * * 1-5"
      prompt: "Provide today's morning briefing: top priorities, weather context, and key reminders."
      session_id: "daily-briefings"
      target:
        adapter: "discord"
        recipient: "channel-general"      # Target Discord channel ID

    # Example 3: Background log-only cleanup task
    - name: "nightly-cleanup"
      schedule: "0 2 * * *"
      prompt: "Review recent session observations and summarize key learnings."
      # session_id omitted -> runs in ephemeral session 'cron:nightly-cleanup'
      # target omitted -> output written to application logs only
```

---

## Schedule Syntax & Supported Formats

AGIS supports both standard 5-field cron syntax and human-friendly duration interval expressions:

### 1. Standard 5-Field Format
```text
 ┌───────────── minute (0 - 59)
 │ ┌───────────── hour (0 - 23)
 │ │ ┌───────────── day of the month (1 - 31)
 │ │ │ ┌───────────── month (1 - 12)
 │ │ │ │ ┌───────────── day of the week (0 - 7) (0 and 7 are Sunday)
 │ │ │ │ │
 * * * * *
```

Examples:
- `*/15 * * * *`: Every 15 minutes.
- `0 9 * * 1-5`: Monday to Friday at 09:00 AM.
- `0 0 1 * *`: First day of every month at midnight.

### 2. Standard Macros
- `@hourly`: Run once an hour at the beginning of the hour (`0 * * * *`).
- `@daily` / `@midnight`: Run once a day at midnight (`0 0 * * *`).
- `@weekly`: Run once a week at midnight on Sunday (`0 0 * * 0`).
- `@monthly`: Run once a month at midnight on the first day (`0 0 1 * *`).
- `@yearly` / `@annually`: Run once a year on January 1st (`0 0 1 1 *`).

### 3. Interval Duration Strings
- `@every 10s`: Every 10 seconds.
- `@every 30m`: Every 30 minutes.
- `@every 2h45m`: Every 2 hours and 45 minutes.

---

## Job Inspection & Execution

### 1. List Configured Jobs
View all active jobs, parsed schedules, and target destinations:

```bash
agis cron list
```

Output:
```text
Configured Cron Jobs (3):
- Name:     daily-health
  Schedule: @every 1h
  Prompt:   Check system load, pending tasks, and database integrity.
  Session:  cron-health
  Target:   telegram -> 123456789

- Name:     morning-briefing
  Schedule: 30 8 * * 1-5
  Prompt:   Provide today's morning briefing...
  Session:  daily-briefings
  Target:   discord -> channel-general

- Name:     nightly-cleanup
  Schedule: 0 2 * * *
  Prompt:   Review recent session observations...
  Session:  cron:nightly-cleanup (ephemeral)
  Target:   (none)
```

### 2. Start the Cron Daemon
Run the autonomous scheduler in the foreground or as a background service:

```bash
agis cron run
```

---

## Session Binding vs Ephemeral Execution

- **`session_id: "<name>"`**: Binds the job to a permanent conversation in SQLite. The agent retains context from previous job executions, allowing it to compare previous outputs with current state.
- **No `session_id` specified**: The scheduler automatically assigns an ephemeral session ID named `cron:<job_name>`, starting fresh on each invocation without polluting main user sessions.

---

## Security & Guardrails

- **AutoDeny in Background**: When executing cron tasks, the brain runs with an `AutoDenyApprover`. Any tool call evaluated by `PolicyGuard` that requires interactive confirmation (`DecisionAsk`) is automatically blocked (`"Action blocked by policy"`), preventing background jobs from hanging or deadlocking.
- **Target Routing**: If the Gateway is enabled in your configuration, outputs are forwarded seamlessly through the Gateway Multiplexer to external chat platforms.
