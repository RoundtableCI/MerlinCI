# MerlinCI

**Marketing that shows its work.**

MerlinCI is an open-source, self-hostable autonomous marketing operating system. It runs the
whole loop — discover, decide, produce, distribute, measure, learn — inside explicit permissions
that a person can pause, inspect and revoke.

RoundtableCI is its first customer and reference deployment. Brand, products, objectives,
channels and policies are configuration, so other companies can run their own.

> Status: Phase 0 (foundation). Not yet usable.

## Principles

- **North star:** activated workspaces — not likes, impressions or assets generated.
- **The model proposes, MerlinCI controls execution.** Credentials stay in the integration
  layer; only permitted tool calls run; publishing actions are never repeated on recovery.
- **Bounded autonomy.** A policy engine enforces three tiers: automatic, conditional, and
  explicit authorization.
- **Capped spend.** Budgets are enforced before money is spent; agents can never raise their
  own limits. No autonomous ad spend.
- **No unsubstantiated claims.** Agents work from an approved product profile.

## Planned stack

| Layer | Choice |
|---|---|
| Control plane and agent runtime | Go |
| Inference | OpenRouter, behind a provider-neutral interface |
| Durable workflows | Temporal or equivalent (open decision) |
| Database | PostgreSQL + pgvector (source of truth) |
| Queue and cache | Redis |
| Object storage | S3-compatible or R2 |
| Dashboard | React + TypeScript |
| Analytics / monitoring | PostHog, Prometheus, Grafana |

## Repository layout

```
cmd/            merlin-api (control plane), merlin-worker (runtime + workflows), merlinctl (CLI)
internal/       runtime, orchestrator, gateway, policy, budget, tools, workflow, integrations,
                store, memory, blob, cache, artifacts, profile, campaign, eval, analytics,
                telemetry, config, capabilities/ (the seven product capabilities)
pkg/            the seven public extension points
schemas/        open agent configuration format and core records
configs/        roles, workflow templates, examples/roundtableci
migrations/     PostgreSQL migrations
deploy/         docker-compose for local dependencies, Prometheus, Grafana
web/            React + TypeScript dashboard
docs/           architecture, development, decisions
```

See [docs/architecture.md](docs/architecture.md) for how each part of the plan maps to code, and
[docs/development.md](docs/development.md) to run it locally.

## Phase 0 — start here

1. ~~Create the repository~~
2. Decide license and contribution policy
3. Write profile schema and seed config
4. Define agent, task and artifact schemas
5. Build minimal Go runtime and OpenRouter gateway
6. Add tool registry, authorization and logging
7. Add PostgreSQL and durable workflows
8. Ship the Market Researcher first

## License

Undecided. To be chosen, with ownership and contributor policy, before the first public release.
