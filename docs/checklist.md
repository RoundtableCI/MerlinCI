# MerlinCI checklist

Everything between today and a finished MerlinCI, from the team pitch (8 Nov 2026).
Status as of 13 Nov 2026. `[x]` is done, `[ ]` is not.

"Done" for code means it is written and its automated tests pass. Where something has only
been proven with stand-ins and not with the real outside service, the list says so. Items
that need a person are also in [handjobs.md](../handjobs.md), with the placeholder in use.

## Where we are

- The whole first-release workflow runs end to end in tests: research, brief, content,
  Brand Guardian, distribution and analytics, on a real PostgreSQL and a real Temporal server.
- The three programs start. A person can start tasks and campaigns, inspect them, approve or
  refuse actions, and pause everything, from the command line or the API.
- **Nothing has run with a real AI model or a real search provider yet.** There are no API
  keys. In every test the model, the search provider and the web are stand-ins.
- Nothing is published anywhere. Distribution prepares a package for a person to publish.
- The dashboard is not started.
- None of this work is committed to git yet.

## 1. Decisions the team has to make (deck slide 18)

- [ ] 01 Open-source licence and contributor policy
- [x] 02 Organization and repository naming (`RoundtableCI/MerlinCI`)
- [ ] 03 Temporal or another durable engine. Built and tested on Temporal; confirm it
- [ ] 04 Hosting provider and topology
- [ ] 05 RoundtableCI activation event (what counts as an "activated workspace")
- [ ] 06 Competitor list and audience segments. The deck's five audience hypotheses are in; the competitor list is empty
- [ ] 07 Search and research API provider. Placeholder: Brave Search
- [ ] 08 First publishing connector and its permissions
- [ ] 09 Image and video generation providers
- [ ] 10 Email provider and consent model
- [ ] 11 Model allowlist and budget limits. The deck's limits are in; the allowlist is a placeholder
- [ ] 12 Which publishing actions run without a person. Placeholder: none do
- [ ] 13 Dashboard in v1, or CLI and API first. Built CLI and API first; confirm it

## 2. Things only you or the team can do

- [ ] Create an OpenRouter account and put the key in `.env`
- [ ] Create a search provider account and put the key in `.env`
- [ ] Choose a long random `MERLIN_API_TOKEN` and put it in `.env`
- [ ] Write the RoundtableCI product profile: summary, features, approved claims with evidence
- [ ] Approve the profile (it is marked draft, which blocks all public output)
- [ ] Confirm the rupee-to-dollar rate and the cost per search in the budget file
- [ ] Create the hosting account (after decision 04)
- [ ] Authorize the first publishing channel as the brand owner (after decision 08)
- [ ] Create a product analytics account, PostHog or equivalent
- [ ] Legal review: licence, email consent, each platform's terms
- [ ] Agree the direction as a team (the deck's "proposed ask")

## 3. Phase 0: Foundation (deck slides 13 and 19)

Goal: one agent takes a task, uses a tool, stores a validated artifact and recovers.

### Step 1. Create the repository

- [x] Repository under the company organization
- [x] Project structure laid out from the pitch
- [x] Automated checks on every push (format, vet, build, test, configuration)
- [x] Local services file (PostgreSQL, Redis, Temporal, object storage, Prometheus, Grafana)

### Step 2. Licence and contribution policy

- [ ] Choose the licence (decision 01)
- [ ] Add `LICENSE` and `CONTRIBUTING.md`

### Step 3. Profile schema and seed config

- [x] Profile schema: company, products, features, claims, north star, objectives, channels
- [x] Profile approval status: draft or approved
- [x] Audience schema and the deck's five audience hypotheses
- [x] Policy schema: three tiers of autonomy, default deny
- [x] Budget schema: limits, alert thresholds, exchange rates, cost per paid call
- [x] Model allowlist schema, with prices
- [x] RoundtableCI seed files: profile, audiences, policy, budget, models
- [x] `merlinctl validate` checks every file and the links between them
- [x] A digest that identifies the exact configuration a run used
- [ ] Real profile content (waiting on the team, section 2)

### Step 4. Agent, task and artifact schemas

- [x] Role schema: runtime roles and logical roles
- [x] The eight runtime roles from the deck
- [x] Workflow template schema and the first-release workflow
- [x] Task record schema
- [x] Artifact record schema with sources, evaluations, cost and config digest

### Step 5. Minimal Go runtime and OpenRouter gateway

- [x] Provider interface (the contract for talking to AI model companies)
- [x] Gateway: only allowed models, worst-case cost reserved before every call
- [x] Model allowlist read from configuration; a role can name its model
- [x] OpenRouter client. Tested against a local stand-in only
- [ ] OpenRouter tested against the real service (needs the API key)
- [x] The six-step agent loop from deck slide 10
- [x] Step limit and per-task spending cap for every role
- [x] A missing or rejected API key ends a run at once, with the reason
- [x] Evaluator interface, with rejected output sent back to the agent with feedback
- [x] First evaluator: output must cite a saved source and make no prohibited claim
- [ ] "Degrade at the limit": fall back to a cheaper model instead of stopping
- [ ] Inference caching and model routing (deck slide 17)

### Step 6. Tool registry, authorization and logging

- [x] Tool interface and registry; unregistered tools cannot be called
- [x] Tool input checked against a schema before the tool sees it
- [x] Policy engine: role permissions, three tiers, most specific rule wins
- [x] Safety floors in code: ad spend, budget increases and cold outreach always need a person
- [x] Approve and reject for actions that need a person; one approval covers one call
- [x] An approval names the action the person was shown, and is refused for any other
- [x] Public actions blocked while the profile is a draft
- [x] Tool results marked as untrusted data, against prompt injection
- [x] Log line for every model call, tool call, refusal and evaluation
- [x] Permanent, searchable record of those events in the database
- [x] Daily counter for the "at most N per day" rule
- [x] Credentials layer: one place that reads keys

### Step 7. PostgreSQL and durable workflows

- [x] Database tables and a migration runner
- [x] Run checkpoints in PostgreSQL
- [x] Record of every tool call, so a finished call is never made again
- [x] An interrupted publish waits for a person instead of repeating
- [x] Artifact records in PostgreSQL
- [x] Artifact content in S3-compatible storage
- [x] Budget ledger in PostgreSQL, safe when many workers spend at once
- [x] Temporal workflow: run, retry, wait for a person's decision, wait while paused
- [x] Recovery across a restart proven on a real PostgreSQL
- [x] Workflow proven on a real Temporal server
- [x] `merlin-worker` starts and runs tasks
- [x] `merlin-api` starts: create, inspect, approve or reject, cancel, pause
- [x] Cancel stops a run at its next checkpoint, and a cancelled run cannot continue
- [x] The event record and the spend record cannot be changed or deleted
- [x] `merlinctl` commands for all of the above
- [x] Budget alerts logged and recorded as events
- [ ] Budget alerts delivered to a person (email or chat)
- [ ] Redis for caching (the local services file starts it; nothing uses it yet)
- [ ] Metrics for Prometheus and a Grafana dashboard
- [ ] Automated checks confirmed green on GitHub with the new service steps
- [ ] Tests run once under the race detector (needs a C compiler this machine lacks)

### Step 8. Ship the Market Researcher (phase 01)

- [x] Role written, configured and switched on
- [x] Research connector (Brave Search). Tested against a local stand-in only
- [x] Page fetcher that refuses internal addresses and obeys robots.txt
- [x] Search and page-fetch tools
- [x] Each search charged to the ₹500 monthly research ceiling before it runs
- [x] End-to-end run through the API, with stand-ins for the model, search and web
- [ ] Search provider tested against the real service (needs the API key)
- [ ] First real run: an evidence-backed market opportunity from a real model
- [ ] Company and campaign context retrieval (memory, using pgvector)

### Phase 0 exit

- [x] Task, tool, validated artifact and recovery proven on a real database and workflow server
- [ ] The same proven with a real model

## 4. The remaining roles (deck slide 13)

Each role ships only when its tools, permissions and evaluation are ready.

- [x] 02 Competitor Analyst: role written and switched on. Not yet run with a real model
- [x] 03 Strategist: turns research into a campaign brief. Not yet run with a real model
- [x] 04 Content: writes the asset from the brief. Not yet run with a real model
- [ ] 04 Social: channel-specific posts
- [x] 05 Brand Guardian: reviews content against the approved profile, as a gate. Not yet run with a real model
- [ ] 06 SEO Specialist: search research, technical audits, content planning
- [x] 07 Distribution, part one: prepares a publication package for a person to publish
- [ ] 07 Distribution, part two: a publishing connector, scheduling, conditional-tier rules
- [ ] 08 Creative and Demo: graphics, video scripts, real product demonstrations
- [x] 09 Analytics, part one: records the campaign and how to measure it
- [ ] 09 Analytics, part two: real results, attribution by campaign, data-quality reporting
- [ ] 10 CMO orchestration: ranking opportunities, accepting or rejecting briefs, setting the next cycle
- [ ] 11 Leads and Growth Lab: transparent lead scoring, draft-only outreach, experiments judged on activation

## 5. First release: one workflow, end to end (deck slide 14)

Proven in tests with stand-ins for the model, the search provider and the web.

- [x] Researcher finds a public opportunity
- [x] Evidence and sources are saved
- [x] Strategist writes a campaign brief
- [x] Content role creates an asset
- [x] Brand Guardian checks accuracy and quality, and stops the campaign when it rejects
- [x] Distribution prepares the asset (it does not publish)
- [x] Analytics records the campaign (it has no results to report)
- [x] The run is inspectable, costed and recoverable
- [ ] The same workflow run with a real model, and its output read and judged by a person
- [ ] Distribution publishes through a real connector
- [ ] Analytics reports real results
- [ ] A self-hoster can follow the documentation from nothing to a finished run

## 6. Dashboard (frontend)

To be built together. Nothing is started. The API it will use exists.

- [ ] Agree what the first version shows (decision 13)
- [ ] Design
- [ ] Runs list and run detail: steps, evidence, cost
- [ ] Campaign view: each step and its artifact
- [ ] Approvals inbox: approve or reject waiting actions
- [ ] Budget view: spend against limits, alerts
- [ ] Artifact viewer with sources
- [ ] Profile and policy view
- [ ] Pause and cancel controls
- [ ] Sign-in and named users (today there is one shared API token)

## 7. Open source and self-hosting (deck slide 16)

- [x] Extension point interfaces: tools, model providers, evaluators
- [ ] Extension point interfaces: agent roles, channel connectors, workflow templates, memory providers
- [ ] Self-hosting documentation
- [ ] Documentation of the configuration format
- [ ] Security policy and a way to report vulnerabilities
- [ ] First public release

## 8. Deployment and operations

- [ ] Production deployment (after decision 04)
- [ ] Container images for the API and the worker
- [ ] Secrets management
- [ ] Database backups and restore test
- [ ] Monitoring and alerting in production
- [ ] First production run watched by a person

## 9. Review

- [x] Independent review of all the code, on 13 Nov 2026: 17 findings
- [x] 15 of the 17 fixed in full, with tests
- [ ] The other 2 fixed in part. What remains is listed as known gaps in `docs/architecture.md`:
      the per-day count is not atomic, and a failed checkpoint can drop one call's cost from a
      task's own total

## 10. Housekeeping

- [ ] Commit the work so far
- [ ] Add a decision record in `docs/decisions` for each decision as it is made

## The measure of success

Not how many agents MerlinCI has, but whether it helped acquire and activate users (deck slide 20).
