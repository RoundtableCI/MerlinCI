# Architecture

Source of truth: the MerlinCI team pitch (8 Nov 2026). This document maps every part of that plan
to where it lives in the code.

## The loop (slide 3)

Discover → Decide → Produce → Distribute → Measure → Learn, run continuously by one coordinated
system — not a collection of chatbots.

## System

```
                  ┌──────────────────── merlin-api (control plane) ────────────────────┐
  configs/ ──────►│ config · permissions · approvals · run & artifact inspection       │
  (profile,       │ pause / inspect / revoke                                           │
   roles,         └───────────────┬────────────────────────────────────────────────────┘
   policy,                        │
   budget)        ┌───────────────▼──────────── merlin-worker ─────────────────────────┐
                  │ orchestrator ── creates, orders, schedules tasks (Temporal)        │
                  │      │                                                             │
                  │      ▼          one reusable runtime, roles are configuration      │
                  │ runtime: 01 load config+permissions → 02 retrieve context (memory) │
                  │          03 inference ──► gateway ──► openrouter   (budget caps)   │
                  │          04 tool call ──► policy ──► tools ──► integrations        │
                  │          05 checkpoint (workflow)  06 validate → artifacts         │
                  │                                                                    │
                  │ eval/brandguardian gate · analytics · capabilities (7)             │
                  └────────────────────────────────────────────────────────────────────┘
     PostgreSQL + pgvector (source of truth) · Redis (queue/cache) · S3/R2 (artifact blobs)
     Prometheus + Grafana (monitoring) · PostHog or equivalent (product analytics)
```

## Deck → code

| Deck | Concept | Code |
|---|---|---|
| 3 | The loop | `internal/orchestrator`, `internal/workflow`, `internal/campaign` |
| 4 | North star: activated workspaces | `internal/analytics` |
| 5 | Seven capabilities, not seven services | `internal/capabilities/*` |
| 6 | Autonomy with boundaries | `internal/policy`, `internal/api` |
| 7 | Customer one, claims rule | `configs/examples/roundtableci`, `internal/profile` |
| 8 | 18 logical / 8 runtime roles | `internal/roles`, `configs/roles` |
| 9 | Signal to next cycle | `internal/orchestrator`, `configs/workflows` |
| 9, 13 | Brand Guardian quality gate | `internal/eval/brandguardian` |
| 10 | Agent loop | `internal/runtime` |
| 10 | Credentials | `internal/integrations/credentials` |
| 10 | Delegation | `internal/orchestrator` |
| 10 | Recovery, no repeated publishing | `internal/workflow` |
| 11 | Three tiers of autonomy | `internal/policy` |
| 12 | Go control plane and runtime | `cmd/merlin-api`, `cmd/merlin-worker` |
| 12 | OpenRouter behind provider-neutral interface | `internal/gateway`, `internal/gateway/openrouter` |
| 12 | Temporal | `internal/workflow` |
| 12 | PostgreSQL + pgvector | `internal/store`, `internal/memory`, `migrations` |
| 12 | Redis | `internal/cache` |
| 12 | S3-compatible or R2 | `internal/blob` |
| 12 | React + TypeScript dashboard | `web` |
| 12 | PostHog or equivalent | `internal/integrations/productanalytics` |
| 12 | Prometheus and Grafana | `internal/telemetry`, `deploy/prometheus`, `deploy/grafana` |
| 14 | Artifact provenance | `internal/artifacts` |
| 14 | Research and publishing connectors | `internal/integrations/research`, `internal/integrations/publishing` |
| 14 | Minimal UI or CLI | `cmd/merlinctl`, `web` |
| 15 | Spend capped before it is spent | `internal/budget` |
| 16 | Seven extension points | `pkg/role`, `pkg/tool`, `pkg/provider`, `pkg/connector`, `pkg/workflowtemplate`, `pkg/evaluator`, `pkg/memory` |
| 16 | Open agent configuration format | `schemas` |
| 18 | Open decisions | `docs/decisions` |

## Deployables

Slide 15 budgets a **Go API and worker**:

- `merlin-api` — control plane.
- `merlin-worker` — agent runtime, orchestrator and Temporal workers. Capabilities and roles
  run here as configuration; they are not separate services (slides 5, 8).
- `merlinctl` — CLI.

## As built

The table above maps the plan to folders. This section records the design choices made while
building, and the reason for each. Status as of 13 Nov 2026. Folders in the table that this
section does not mention are still placeholders.

### Layers

Dependencies point one way: `cmd` → `internal/app` → everything else; adapters depend on the
core, never the reverse.

| Layer | Packages | May import |
|---|---|---|
| Public contracts | `pkg/tool`, `pkg/provider`, `pkg/evaluator` | Only each other |
| Core | `internal/config`, `policy`, `budget`, `gateway`, `tools`, `eval`, `artifacts`, `runtime` | Public contracts and each other |
| Adapters | `internal/store` (PostgreSQL), `gateway/openrouter`, `integrations/*`, `blob/s3blob` (S3), `workflow` (Temporal), `api` (HTTP) | Core and public contracts |
| Composition | `internal/app`, `cmd/*` | Everything |

The core never names PostgreSQL, Temporal, OpenRouter or S3. It depends on small interfaces
(`runtime.Checkpointer`, `runtime.Effects`, `gateway.Ledger`, `blob.Store`,
`artifacts.Index`, `provider.Provider`), each with an in-memory implementation used by the
tests. `internal/app` is the only place that knows which real service is behind each one.

### How a tool call is decided

In order, and stopping at the first refusal: the tool is one the role was given; it is
registered; its input matches its schema; the action is not public-facing while the profile
is a draft; the role has permission for the action; the most specific policy rule matches
(none matching means deny); a floor raises the tier to explicit if the action is ad spend, a
budget increase or outreach. Automatic runs. Conditional runs if its channel, content and
frequency conditions are met, and otherwise waits for a person. Explicit waits for a person.

### Never doing something twice

Three records make recovery safe, and they are deliberately separate:

- **Checkpoint** (`run_states`): the conversation and counters after every step. A run resumes
  from it.
- **Tool-call record** (`effects`): written before a tool runs and completed after. A completed
  call is answered from the record and not made again. A call left "started" was interrupted:
  it re-runs only if it is in the automatic tier; anything else waits for a person, because
  nobody knows whether it happened.
- **Approval sequence** (`ApprovalSeq`): counts each time a run stops for a person. A decision
  names the stop it answers and is ignored at any other stop. The API requires the caller to
  send the number of the action they were shown, and refuses a decision whose number is not
  the current one, so a late or repeated request cannot decide an action nobody looked at. An approval is marked used, and
  that fact is checkpointed, before the approved call runs.

Temporal retries activities, so every activity is written to be safe to repeat: it resumes
the run from these records instead of starting it again.

### Money

- Configured amounts are decimal strings parsed to integers. No float ever holds money.
- The ledger works in millionths of the budget currency, because one model call can cost less
  than the smallest coin.
- Every model call reserves its worst case first: an over-estimate of the input tokens plus
  the full output allowance, at the allowlist price, converted at the configured rate, all
  rounded up. The call is refused if that does not fit the daily, monthly or per-task limit.
- After the call the reservation is replaced by the actual cost. A call the provider refused,
  or that never reached it, is released. A call that failed in a way that may have been billed
  (a timeout, a dropped connection, an unreadable reply) is counted at its worst case.
- A reservation left behind by a crash keeps counting against the period it was made in.
  Every rounding and every failure mode errs toward counting too much, never too little.
- In PostgreSQL, reservations for one category are serialized with an advisory lock, so two
  workers cannot both take the last of a budget.

### Configuration

- Every file is checked against a JSON Schema and then decoded strictly into Go types, so the
  two cannot drift apart silently.
- A digest of the whole configuration is recorded on every artifact and run.
- A run continues under the configuration in force when it resumes, not the one it started
  under, so a permission revoked while it was stopped stays revoked.
- The programs refuse to start on a configuration with errors, or when an enabled role names
  a tool or evaluator that does not exist.

### Stopping

- **Pause** is a switch in the database that every run checks before each step. Paused runs
  stay where they are; a decision made during a pause is applied when the pause ends.
- **Cancel** writes a cancelled checkpoint. The store refuses to replace a cancelled
  checkpoint with anything else, so a worker in the middle of the task fails at its next
  save and stops. The database, not the workflow engine, is what stops the run.

### Workflows

- A task is one Temporal workflow whose ID is the task ID, so a task cannot be started twice.
- The workflow history holds only a small summary. Conversations and evidence stay in
  PostgreSQL.
- A campaign is a parent workflow. Each role step is a child task workflow; each gate is an
  activity. Steps run in dependency order, one at a time.

### Known gaps in the design

- Steps of a campaign that could run in parallel do not.
- Cancelling a task takes effect at the worker's next checkpoint. The step in progress (one
  model call or one tool call) finishes first; nothing after it runs.
- If the checkpoint after a model call fails, that call's cost is in the budget ledger but
  missing from the task's own total, so the per-task cap can be passed by one call.
- The "at most N per day" count and the start of the counted action are not one atomic step,
  so two tasks at the same instant could both pass. No rule uses the conditional tier yet.
- Temporal has no authentication in the local setup. Anyone who can reach it can signal a
  workflow without the API token, so in production it must be reachable only by the API and
  the workers.
- The tests have not been run under Go's race detector.
- The cost of a campaign gate (Brand Guardian as its own step) is charged to the budget but is
  not added to any single task's total.
- There is one shared API token and no named users.
