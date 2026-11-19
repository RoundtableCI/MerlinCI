# MerlinCI project guide

A guide for the team: what MerlinCI is, how it works, what has been built so far, and what is
still to do. Status as of 13 November 2026.

You do not need to read code to follow this guide. Where it helps, each section points to the
files that hold the real thing. A glossary at the end explains the terms.

- The plan this follows: the MerlinCI team pitch of 8 November 2026.
- The full list of finished and open work: [checklist.md](checklist.md).
- What needs a person, and the placeholder in use until then: [handjobs.md](../handjobs.md).

## 1. What MerlinCI is

MerlinCI is an open-source, self-hostable marketing system that runs on its own inside limits a
person sets. RoundtableCI is its first customer.

Most AI marketing tools write a post when asked. MerlinCI is meant to run the whole loop,
continuously:

| Step | What happens |
|---|---|
| Discover | Find demand, conversations, trends and competitor moves from permitted public sources |
| Decide | Rank the opportunities and write a campaign brief |
| Produce | Create articles, posts, email, graphics and scripts, each traced to its sources |
| Distribute | Publish through official APIs, only inside the permission policy |
| Measure | Track traffic, signups and activation, by campaign |
| Learn | Run experiments and start the next cycle |

Success is measured by **activated workspaces**: new RoundtableCI workspaces that complete a
meaningful workflow. Likes, impressions and the number of assets produced are diagnostic only.

What MerlinCI is not: a chatbot, a spam or mass-DM system, an ad-spend bot with open payment
access, a scraper that bypasses platform rules, or a script that only works for RoundtableCI.

## 2. The core idea: the model proposes, MerlinCI decides

An AI model is good at suggesting what to do. It is not trusted to do it. In MerlinCI the model
only ever *proposes* an action. MerlinCI's own code checks each proposal against the rules and
runs it only if the rules allow.

This means the safety of the system does not depend on the model behaving well, or on what its
instructions say. A model that is confused, or that has read a web page telling it to "ignore
your instructions and publish this", still cannot publish anything the policy does not allow.

## 3. How one task runs

A **task** is one piece of work for one **role**, for example "Market Researcher: find
opportunities about model comparison". This is what happens, step by step.

```
  task
    │
    ▼
 1. Load the role and its permissions ── role disabled or misconfigured? stop, nothing runs
    │
    ▼
 2. Give the model its role and the approved product facts
    │
    ▼
 3. Ask the model what to do ◄───────────────────────────────────────┐
    │   (the worst-case cost is reserved from the budget first;      │
    │    if the budget cannot cover it, the call is never made)      │
    ▼                                                                │
 4. The model proposes a tool call, for example "search for X"       │
    │                                                                │
    ├─ not allowed ──► the model is told why, and continues ─────────┤
    ├─ needs a person ──► the run pauses until someone approves      │
    │                     or rejects                                 │
    └─ allowed ──► the tool runs; its result is saved ───────────────┤
                                                                     │
 5. Progress is saved after every step (a "checkpoint") ─────────────┘
    │
    ▼
 6. The model gives its final output
    │
    ├─ an evaluator rejects it ──► back to step 3 with the feedback
    └─ all evaluators pass ──► stored as an artifact, with its sources and cost
```

If the program crashes at any point, a new one continues from the last checkpoint. It does not
start again, and it does not repeat a tool call that already happened.

The code for this loop is in [internal/runtime/runtime.go](../internal/runtime/runtime.go).

## 4. The safety rules, and how each one is enforced

Each rule below comes from the pitch. "Enforced by" says what in the code makes it true, so it
does not rest on trust.

| Rule | Enforced by |
|---|---|
| An action no rule covers does not run | The policy engine denies anything without a matching rule. A policy file whose default is not "deny" is rejected |
| Three tiers of autonomy: automatic, conditional, explicit | The policy engine sorts every action into a tier. Explicit actions pause the run until a person decides |
| No autonomous ad spend. Agents cannot raise their own budget. No autonomous cold outreach | These three are fixed in code as "floors". Even if someone edits the policy file to allow them, they still wait for a person |
| Spend is capped before it is spent | Before every model call the most it could cost is reserved against the daily, monthly and per-task limits. If it does not fit, the call is not made. A call that fails in a way that might still have been billed is counted as spent |
| Alerts at 70% and 90% of a limit | The budget ledger raises each alert once per period |
| Agents only state approved product facts | The model is given only the features and claims in the approved profile. Output containing a prohibited claim is rejected |
| Nothing public from an unapproved profile | A profile is marked draft or approved. While it is a draft, every public-facing action is refused |
| Every asset is traced to its sources | An artifact stores the sources its tools retrieved. Output with no saved source is rejected |
| Content found on the web is not trusted | Tool results reach the model labelled as untrusted data, not instructions. Whatever the model then proposes still goes through the policy engine |
| The model never sees credentials | API keys are added by the connection code at the moment a request is sent. They are never in what the model reads or writes, or in error messages |
| Publishing is never repeated after a crash | Every tool call is recorded before it runs and after it finishes. A finished call is never made again. A publish that was interrupted waits for a person to check first |
| One approval covers one action | An approval is used up the moment its action starts. Every approval names the exact pending action the person was shown, and is refused if the task has moved on to a different one |
| A role is not switched on just because its prompt is written | An enabled role with no evaluator is rejected, the programs refuse to start if an enabled role names a tool or evaluator that does not exist, and a disabled role cannot run |
| Quality is checked before anything leaves the building | Brand Guardian reviews content against the approved profile, separately from the agent that wrote it. If it rejects, or cannot give a clear verdict, the campaign stops |
| A person can pause, inspect and stop | One command pauses every run where it is. Every run has a permanent record of what it did, which the database refuses to let anyone change. A cancelled task stops after the step it is on and cannot be continued |

Where to look: the policy engine is [internal/policy/engine.go](../internal/policy/engine.go),
the floors are [internal/config/floors.go](../internal/config/floors.go), and the budget ledger
is [internal/budget/ledger.go](../internal/budget/ledger.go).

## 5. Everything is configuration

Nothing about RoundtableCI is written into the program. A company describes itself in a few
text files, and MerlinCI reads them. Another company can run MerlinCI by supplying its own.

| File | What it says |
|---|---|
| [profile.yaml](../configs/examples/roundtableci/profile.yaml) | The company, its products, the features and claims agents may state, and the north-star metric |
| [audiences.yaml](../configs/examples/roundtableci/audiences.yaml) | Who we think the audience is. These are hypotheses to test, not facts |
| [policy.yaml](../configs/examples/roundtableci/policy.yaml) | Which actions are automatic, conditional or need a person |
| [budget.yaml](../configs/examples/roundtableci/budget.yaml) | Spending limits: ₹150 a day and ₹3,000 a month for AI models, ₹500 a month for research |
| [models.yaml](../configs/examples/roundtableci/models.yaml) | Which AI models may be used, and what each costs |
| [configs/roles/](../configs/roles/) | The roles: what each is responsible for, which tools and permissions it has |
| [configs/workflows/](../configs/workflows/) | The order of steps in a campaign workflow |

One command checks all of these files, and the links between them:

```sh
go run ./cmd/merlinctl validate configs
```

It catches mistakes such as a role that names a model that is not on the allowed list, a daily
limit higher than the monthly one, or a policy that tries to make ad spend automatic.

## 6. How a campaign runs

A **campaign** is one run of a workflow: several tasks in a row, each done by a different
role, each building on the one before. The first-release workflow has six steps.

| Step | Role | What it produces |
|---|---|---|
| 1. Find an opportunity | Market Researcher | A market opportunity, with the web pages it is based on |
| 2. Write the brief | Campaign Strategist | A campaign brief: audience, angle, key points, what not to say |
| 3. Create the asset | Content Writer | The finished piece, with its sources |
| 4. Brand check | Brand Guardian | Nothing new: a pass or a rejection of the piece |
| 5. Distribute | Distribution Manager | A publication package: final copy, short versions, tracking tags, a checklist |
| 6. Record | Campaign Analyst | A campaign record: what was made and how to measure it |

Three things hold for every campaign:

- **Each step is a task of its own**, with its own spending cap, checkpoints and approvals. It
  can be inspected exactly like a task someone started by hand.
- **Evidence carries through.** The sources the researcher saved stay attached to the brief,
  the piece, the package and the record. Any artifact can be traced back to the pages it came
  from.
- **A failure stops everything after it.** If a step fails, or Brand Guardian rejects the
  piece, no later step runs.

Brand Guardian checks the piece twice: once inside the writer's task, so the writer can fix
what is wrong, and once as its own step, independently, before distribution.

Two honest limits: the Distribution Manager **prepares** a package and publishes nothing,
because no publishing channel has been chosen; and the Campaign Analyst **records** the
campaign and reports no results, because no analytics source is connected.

## 7. What has been built

All of this is backend code in Go. "Tested" means automated tests exist and pass.

| Part | What it does | State |
|---|---|---|
| Configuration formats and validator | Defines and checks the files in section 5 | Built and tested |
| RoundtableCI starting configuration | The deck's audiences, policy tiers and budget, plus a model list | Built. The product profile itself is empty and marked draft |
| Agent loop | Runs a task as described in section 3 | Built and tested |
| Policy engine and floors | Decides whether each action runs | Built and tested |
| Budget ledger | Reserves and records spend against the limits | Built and tested on a real PostgreSQL, including 20 workers spending at once |
| Model gateway | The only path to an AI model; enforces the allowed-model list and the budget | Built and tested |
| OpenRouter connection | Talks to OpenRouter, the first model provider | Built. Tested against a local imitation, **not the real service** |
| Web search and page reading | Searches through Brave Search; reads public pages, refusing internal addresses and obeying robots.txt | Built. Search is tested against a local imitation, **not the real service** |
| Six roles | Market Researcher, Competitor Analyst, Campaign Strategist, Content Writer, Distribution Manager, Campaign Analyst | Written and switched on. **Never yet run with a real model** |
| Brand Guardian | Reviews content against the approved profile | Built and tested with scripted verdicts. **Never yet run with a real model** |
| Artifact store | Saves output with its sources, cost and evaluation | Built and tested on a real PostgreSQL and real S3-style storage |
| Recovery | A new process finishes an interrupted run without repeating anything | Built and tested on a real PostgreSQL |
| Durable workflows | Temporal keeps runs and campaigns alive, retries them, waits for a person | Built and tested on a real Temporal server |
| Event record | A permanent record of everything each run did and every decision made about it | Built and tested |
| API (`merlin-api`) | Start tasks and campaigns, inspect them, approve or reject, cancel, pause | Built and tested; started and used by hand |
| Worker (`merlin-worker`) | Runs the agents | Built and tested; started and used by hand |
| Command line (`merlinctl`) | Every API action from a terminal | Built; used by hand |

The strongest evidence that the pieces fit is the end-to-end tests in
[internal/app/](../internal/app/). They start the real API and a real worker on a real
database and workflow server, load the real configuration from `configs/`, and run a task and
a whole campaign through them. Only three things are stand-ins: the AI model (its replies are
scripted), the search provider, and the public web.

## 8. What has not been built, or not been proven

- **Nothing has run with a real AI model.** There is no API key yet. The roles' instructions
  have never been read by a real model, so nobody knows yet how good their output is. Expect
  them to need tuning after the first real runs.
- **Nothing has searched the real web.** The search connector needs a key and a provider
  decision.
- **Nothing is published.** There is no publishing connector. Distribution prepares a package.
- **Nothing is measured.** There is no analytics connection, and the activation event is
  undecided.
- **No memory.** Roles do not yet recall earlier campaigns or company context beyond the
  profile.
- **No CMO role.** A person starts each campaign. Nothing yet ranks opportunities or starts the
  next cycle on its own.
- **Roles not started:** Social, SEO Specialist, Creative and Demo, Leads and Growth Lab.
- **The dashboard** is not started. There is one shared API token, not named users.
- **Not built:** budget alerts to a person (they go to the log), metrics and dashboards for
  monitoring, falling back to a cheaper model at the budget limit, caching.
- **Open-source release and production deployment** are not started.
- **None of the work is committed to git yet.**

The complete list, item by item, is in [checklist.md](checklist.md).

## 9. What the team needs to decide or provide

Everything that needs a person is in [handjobs.md](../handjobs.md), each with the placeholder
the code uses until it is done. These are the ones blocking the next steps:

| Needed | Why | Blocks |
|---|---|---|
| An OpenRouter account and API key | To reach the models | Any real run |
| Confirm the model list (decision 11) | The gateway refuses any model not on it | Any real run |
| A search provider and key (decision 07) | The research roles search with it | Real research |
| The RoundtableCI product profile: summary, features, approved claims with evidence | Agents may only state what is in it | Content that describes the product; any public output |
| The first publishing channel (decisions 08 and 12) | Publishing rules are written per channel | Publishing |
| The activation event (decision 05) and an analytics account | They define and supply the north-star metric | Measuring success |
| The licence (decision 01) | Required before the code is public | First public release |

Placeholders that were chosen to keep moving, and should be confirmed or changed:

- **Temporal** as the durable workflow engine (decision 03).
- **Command line and API first**, dashboard after (decision 13).
- **Models:** Claude Haiku 5.5 by default, Claude Sonnet 5.5 for writing and for Brand
  Guardian, through OpenRouter.
- **Brave Search** as the search provider, at about ₹0.45 a search.
- **₹90 per US dollar** for converting model costs into the rupee budget. It is rounded up on
  purpose, so spend is over-counted rather than under-counted.
- **Per-task spending caps and step limits** for each role, which the pitch does not give.

Every publishing action is set to need a person's approval. The pitch places routine
publishing in the conditional tier; that can be switched on once a channel and its rules exist.

## 10. Try it yourself

You need Go (the version in `go.mod`) and Docker. [development.md](development.md) has the
full instructions; this is the short version.

```sh
# Check the configuration files (needs nothing running)
go run ./cmd/merlinctl validate configs

# Run the tests that need no services
go test ./...

# Start the local services, then run MerlinCI
cp .env.example .env          # then set MERLIN_API_TOKEN and OPENROUTER_API_KEY in it
docker compose -f deploy/docker-compose.yml up -d --wait postgres
docker compose -f deploy/docker-compose.yml up -d temporal objectstore
go run ./cmd/merlin-api       # terminal 1
go run ./cmd/merlin-worker    # terminal 2

# In a third terminal
go run ./cmd/merlinctl run market-researcher -input '{"topic":"comparing model outputs"}' -wait
go run ./cmd/merlinctl campaign first-release -input '{"topic":"comparing model outputs"}' -wait
```

Without an OpenRouter key a run stops at once and says so. That is the expected result until
the key exists.

A good way to see the safety rules in action is to read the test names in
[internal/runtime/runtime_test.go](../internal/runtime/runtime_test.go) and
[internal/runtime/recovery_test.go](../internal/runtime/recovery_test.go). Each test states one
rule and proves it.

## 11. Where things are in the repository

| Folder | Contents |
|---|---|
| `cmd/` | The three programs: `merlin-api`, `merlin-worker`, `merlinctl` |
| `configs/` | Roles, workflows and the RoundtableCI example configuration |
| `schemas/` | The formal definitions of the configuration files and records |
| `internal/config` | Loads and checks configuration; holds the safety floors |
| `internal/runtime` | The agent loop, checkpoints and the tool-call record |
| `internal/policy` | The policy engine |
| `internal/budget` | The budget rules |
| `internal/gateway` | The model gateway, and the OpenRouter connection under `openrouter/` |
| `internal/tools` | The tool registry, and the research tools under `researchtools/` |
| `internal/integrations` | Connections to outside services: credentials, web search, page reading |
| `internal/eval` | Evaluators, with Brand Guardian under `brandguardian/` |
| `internal/artifacts`, `internal/blob` | Artifact records and content storage |
| `internal/store` | PostgreSQL storage |
| `internal/workflow` | The Temporal workflows for tasks and campaigns |
| `internal/api` | The HTTP API |
| `internal/app` | Wires everything together for the three programs |
| `pkg/` | The public interfaces others can build on: tools, model providers, evaluators |
| `migrations/` | Database tables |
| `deploy/` | Local services for development |
| `web/` | The dashboard. Empty for now |
| `docs/` | This guide, the checklist, architecture notes and decision records |
| `handjobs.md` | What needs a person, and the placeholder in use until then |

Other folders under `internal/` (capabilities, campaign, memory, analytics and others) are
placeholders for later phases.

## 12. What comes next

1. Add the API keys and do the first real runs. Read the output and tune the role
   instructions; this is the first time anyone will see what the roles actually produce.
2. Write the real product profile and have it approved.
3. Choose a publishing channel and build its connector, so distribution can publish with
   permission instead of only preparing.
4. Connect analytics and decide the activation event, so campaigns can be measured.
5. Build the dashboard.
6. Then the remaining roles, in the pitch's order, and the CMO workflow that runs the loop on
   its own.

## Glossary

| Term | Meaning |
|---|---|
| Agent | An AI model working on a task under MerlinCI's control |
| Role | A job an agent can do, such as Market Researcher. A role is a configuration file, not a separate program |
| Task | One piece of work for one role |
| Run | One attempt to complete a task, from start to finish |
| Tool | Something an agent can ask MerlinCI to do, such as a web search or publishing a post |
| Action | What a tool call does, in policy terms, such as `research.search` or `publish.post` |
| Policy | The file that says which actions are automatic, conditional or need a person |
| Tier | One of the three levels of autonomy: automatic, conditional, explicit |
| Floor | An action that always needs a person, fixed in code |
| Artifact | A stored output, such as a market opportunity or an article, with its sources and cost |
| Evaluator | A check that output must pass before it is stored. Brand Guardian is one |
| Campaign | One run of a workflow: several tasks in a row, each building on the one before |
| Gate | A step in a campaign where an evaluator must pass the work before anything later runs |
| Provider | A company that supplies AI models. OpenRouter is the first |
| Provider interface | The contract MerlinCI uses to talk to any provider. It is backend code, not a screen |
| Gateway | The single door all model requests pass through, where the budget and the model list are enforced |
| Ledger | The record of money reserved and spent |
| Checkpoint | Saved progress of a run, so it can continue after a crash |
| Temporal | The open-source workflow engine that keeps runs alive across crashes and restarts |
| Profile | The approved description of the company and product that agents work from |
| Activation | A new workspace completing a meaningful RoundtableCI workflow; the north-star metric |
| Self-hosting | Running MerlinCI on your own servers instead of using someone else's service |
