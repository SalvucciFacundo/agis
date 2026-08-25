# Persona

AGIS separates who it is from how it behaves right now. Three layers compose the identity slot of every prompt, in this order (spec §8):

1. **SOUL.md** — the durable identity.
2. **Personality overlay** — a session-scoped mode switch (`/personality`).
3. **Evolution layer** — derived guidance learned from your history.

## SOUL.md (durable identity)

Lives at `$AGIS_HOME/SOUL.md` (`~/.agis/SOUL.md`). It is seeded automatically on first run from a built-in default and is **never overwritten** by AGIS afterwards: edit it freely.

- Empty or unreadable file → the built-in default is used instead.
- Contents pass through the injection scanner; flagged lines are dropped.
- The file is only ever read from `$AGIS_HOME`, never from the working directory — the identity belongs to the agent instance, not to whatever project you launched it from.

## Personality overlays

`/personality <name>` switches the assistant's mode for the current session only:

| Preset | Behavior |
|---|---|
| `concise` | Minimum words, no preamble |
| `teacher` | Answer + brief why + underlying concept |
| `technical` | Exact terminology, code-first, no pleasantries |
| `creative` | Vivid language, substance intact |
| custom | Defined in config under `agent.personalities` |

`/personality none` (or `default`, `neutral`) returns to the SOUL baseline. Unknown names report an error and change nothing.

## Evolution (seed, not cage)

The persona starts as a seed and evolves through the learning loop: curated observations about you are aggregated into user-model rows, and the top five by confidence become a "how to work with this user" guidance block appended to the identity.

Commands:

- `/persona freeze` — exclude the evolution layer for this session.
- `/persona reset` — clear the derived user-model rows (rebuildable from observations; long-term memory is untouched).
- `/persona status` — show mode and row count.

Set `agent.evolution_enabled: false` in config to disable evolution entirely — the traditional static-identity mode. Evolution never rewrites SOUL.md.
