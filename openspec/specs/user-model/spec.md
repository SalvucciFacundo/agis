# User Model Spec

## Purpose

Aggregate user/* observations into user-model rows with blended confidence so the agent keeps an evolving model of the user.

## Requirements

user-model (NEW)

### Requirement: Aggregation
Pure function. Only `topic_key` prefix `user/` included. `key`=full `topic_key`. First write: `confidence=clamp(importance/5,0,1)`. Update: `clamp(0.7*old+0.3*new,0,1)`.

#### Scenario: First write
- GIVEN `topic_key=user/pref/coffee`, importance=4 → confidence=0.8

#### Scenario: Update blend
- GIVEN old=0.8, new importance=3(0.6) → 0.74

#### Scenario: Non-user excluded
- GIVEN `topic_key=project/arch` → no row

---
