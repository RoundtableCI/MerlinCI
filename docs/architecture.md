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
