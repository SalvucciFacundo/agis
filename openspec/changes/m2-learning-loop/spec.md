# M2 Learning Loop — Delta Specs

## memory-curator (NEW)

### AGIS-M2-CUR-001: LLM-driven curation
System MUST use ONE `Provider.Chat` call per nudge/close returning JSON `{topic_key, type, content, importance}`. Parse failures MUST log+skip, never fail the turn. Fence stripping MUST precede parsing.

#### Scenario: Valid JSON
- GIVEN provider returns `[{topic_key:"a",importance:4}]`
- WHEN parsed → `SaveObservations` called, importance=4

#### Scenario: Malformed response
- GIVEN provider returns prose
- WHEN parsed → error logged, zero rows, turn continues

#### Scenario: Missing importance
- GIVEN JSON without importance → defaults to 3

### AGIS-M2-CUR-002: Nudge cadence
MUST trigger curator every `nudge_every` assistant msgs (default 10). Each nudge writes `session_events(kind='nudge')`. `nudge_every:0` disables.

#### Scenario: Boundary trigger
- GIVEN 10 assistant msgs, `nudge_every=10`
- WHEN 11th Step begins → curator runs, nudge row written

### AGIS-M2-CUR-003: Kill switch
`memory.learning_enabled:false` MUST suppress all curator/summarizer Chat calls.

#### Scenario: Disabled → zero LLM calls, immediate exit

---

## session-summarizer (NEW)

### AGIS-M2-SUM-001: Combined close call
At close, ONE `Chat` MUST return `{summary, observations[]}`. `UpdateConversationSummary` MUST NOT bump `conversations.updated_at`.

#### Scenario: One call produces both
- WHEN CloseSession → `UpdateConversationSummary` + `SaveObservations` called

#### Scenario: Ordering preserved
- WHEN UpdateConversationSummary → `updated_at` unchanged

### AGIS-M2-SUM-002: Close timeout
Close MUST be synchronous, bounded by `close_timeout` (default 30s). Errors non-fatal.

#### Scenario: Timeout
- GIVEN provider hangs → ctx cancels at 30s, logged, exit continues

---

## user-model (NEW)

### AGIS-M2-USR-001: Aggregation
Pure function. Only `topic_key` prefix `user/` included. `key`=full `topic_key`. First write: `confidence=clamp(importance/5,0,1)`. Update: `clamp(0.7*old+0.3*new,0,1)`.

#### Scenario: First write
- GIVEN `topic_key=user/pref/coffee`, importance=4 → confidence=0.8

#### Scenario: Update blend
- GIVEN old=0.8, new importance=3(0.6) → 0.74

#### Scenario: Non-user excluded
- GIVEN `topic_key=project/arch` → no row

---

## repository-memory (MODIFIED)

### AGIS-M2-REPO-001: Extended port
MUST add `SaveObservations`, `Observations`, `UpdateConversationSummary`, `UpsertUserModel`. Upsert on UNIQUE `topic_key`, preserve `created_at`, bump `updated_at`. FTS delete+insert same-tx.
(Previously: 5 methods, no observation writes.)

#### Scenario: Upsert
- GIVEN topic_key=X, created_at=T1 → re-save: created_at=T1, updated_at>T1

#### Scenario: FTS delete-sync
- GIVEN "coffee" indexed → upsert "tea" → "coffee" returns nothing

#### Scenario: Batch atomicity
- GIVEN 3 obs, 2nd invalid → zero persisted

### AGIS-M2-REPO-002: Multi-word AND search
`Search` MUST split on whitespace, quote each term, join AND.
(Previously: M1 exact-phrase wrap — behavior change.)

#### Scenario: AND semantics
- GIVEN msg1="coffee", msg2="preference" → Search("coffee preference") returns zero

#### Scenario: Both terms match
- GIVEN msg="coffee preference noted" → returned

### AGIS-M2-REPO-003: Migration 0002
MUST: (1) ADD updated_at+backfill, (2) UNIQUE topic_key index, (3) CREATE user_model, (4) CREATE session_events CHECK kind IN('nudge','summary','skill'). Idempotent via user_version.

#### Scenario: v1→v2
- GIVEN user_version=1 → 0002 applies, version=2

#### Scenario: Idempotent
- GIVEN user_version=2 → no SQL, version=2

---

## brain-loop (MODIFIED)

### AGIS-M2-BRN-001: Recall injection
`Step` MUST load top-N observations (`recall_limit`, default 10) into system prompt.
(Previously: no recall — write-only memory.)

#### Scenario: Observations prepended
- GIVEN 5 obs → Step includes them in provider request

### AGIS-M2-BRN-002: CloseSession
MUST: (1) load msgs, (2) Chat→summary+obs, (3) UpdateConversationSummary, (4) SaveObservations, (5) UpsertUserModel. Respects ctx deadline.

#### Scenario: Order
- GIVEN fakes → calls: summary→obs→user_model

#### Scenario: Stream cancel
- GIVEN streaming Step, ctx canceled → stream drains, CloseSession runs

---

## minimal-tui (MODIFIED)

### AGIS-M2-TUI-001: Close hook
CtrlC/Esc MUST call `CloseSession` with status line. Synchronous, bounded by `close_timeout`. Streaming: 1st CtrlC cancels stream; 2nd quits immediately.
(Previously: quit immediately, no close hook.)

#### Scenario: Idle quit
- CtrlC → "closing…" → CloseSession → quit

#### Scenario: Streaming cancel
- CtrlC → cancel stream → drain → close

#### Scenario: Force quit
- CtrlC×2 → immediate quit, no close
