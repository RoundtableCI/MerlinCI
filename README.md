# MerlinCI

**Marketing that shows its work.**

MerlinCI is an open-source, self-hostable autonomous marketing operating system. It runs the
whole loop — discover, decide, produce, distribute, measure, learn — inside explicit permissions
that a person can pause, inspect and revoke.

RoundtableCI is its first customer and reference deployment. Brand, products, objectives,
channels and policies are configuration, so other companies can run their own.

> Status: the first-release workflow runs end to end in tests, on a real database and workflow
> server, with stand-ins for the AI model and the web. It has not yet run with a real model,
> and it publishes nothing. See [docs/project-guide.md](docs/project-guide.md) for what exists,
> [docs/checklist.md](docs/checklist.md) for what is left, and [handjobs.md](handjobs.md) for
> what needs a person.

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
| Durable workflows | Temporal (open decision 03) |
| Database | PostgreSQL + pgvector (source of truth) |
| Cache | Redis (not used yet) |
| Object storage | S3-compatible or R2 |
| Dashboard | React + TypeScript (not started) |
| Analytics / monitoring | PostHog, Prometheus, Grafana (not connected yet) |

## Repository layout

```
cmd/            merlin-api (control plane), merlin-worker (runtime + workflows), merlinctl (CLI)
internal/       built: config, runtime, policy, budget, gateway, tools, eval, artifacts, blob,
                store, workflow, api, app, integrations (credentials, research)
                placeholders for later phases: orchestrator, memory, cache, profile, campaign,
                analytics, telemetry, roles, capabilities/ (the seven product capabilities)
pkg/            public extension points. Built: tool, provider, evaluator. Placeholders:
                role, connector, workflowtemplate, memory
schemas/        open agent configuration format and core records
configs/        roles, workflow templates, examples/roundtableci
migrations/     PostgreSQL migrations
deploy/         docker-compose for local dependencies, Prometheus, Grafana
web/            React + TypeScript dashboard
docs/           project guide, checklist, architecture, development, decisions
handjobs.md     what needs a person, and the placeholder in use until then
```

See [docs/architecture.md](docs/architecture.md) for how each part of the plan maps to code, and
[docs/development.md](docs/development.md) to run it locally.

## Phase 0 — start here

1. ~~Create the repository~~
2. Decide license and contribution policy
3. ~~Write profile schema and seed config~~
4. ~~Define agent, task and artifact schemas~~
5. ~~Build minimal Go runtime and OpenRouter gateway~~
6. ~~Add tool registry, authorization and logging~~
7. ~~Add PostgreSQL and durable workflows~~
8. ~~Ship the Market Researcher first~~ (built and tested with stand-ins; not yet run with a real model)

## License

Undecided. To be chosen, with ownership and contributor policy, before the first public release.
