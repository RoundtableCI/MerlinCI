# Handjobs

Things that need a person. The code keeps moving with a placeholder for each one; this file
records what the placeholder is, so nothing here is forgotten or mistaken for a decision.

Tick an item when it is done. Items are grouped by what they unblock. Status as of 13 Nov 2026.

## A. Needed before anything runs with a real AI model

- [ ] **Create an OpenRouter account and API key.** Put the key in `.env` as `OPENROUTER_API_KEY`.
      Until then every run stops at once with "OPENROUTER_API_KEY is not set", and the OpenRouter
      connection has only been tested against a local imitation.
- [ ] **Choose an API token.** Put a long random secret (24 characters or more) in `.env` as
      `MERLIN_API_TOKEN`. The API refuses to start without one. Anyone who has it can start work
      and approve actions, so treat it like a password.
- [ ] **Confirm the model allowlist (decision 11).**
      Placeholder in `configs/examples/roundtableci/models.yaml`: `anthropic/claude-haiku-5.5` as
      the default ($0.10 in, $0.50 out per million tokens) and `anthropic/claude-sonnet-5.5` for
      writing and for Brand Guardian ($2 in, $10 out). Prices were read from OpenRouter's public
      list on 13 Nov 2026 and are fixed in that file. They must stay at or above what OpenRouter
      charges, or the budget will under-reserve; update them when OpenRouter's prices change.
- [ ] **Confirm the exchange rate.** Placeholder in `budget.yaml`: ₹90 per US dollar, rounded up on
      purpose so spend is over-counted rather than under-counted.
- [ ] **Do the first real run and read the output.** Start the services, then
      `merlinctl run market-researcher -input '{"topic":"..."}' -wait`. Every test so far used a
      scripted stand-in for the model, so nobody has yet seen what a real model does with these
      roles. Expect the role instructions to need tuning.

## B. Needed before the Market Researcher can search the web

- [ ] **Choose the search provider (decision 07).** Placeholder: Brave Search, through its
      official API. The connector is written but has only been tested against a local imitation.
- [ ] **Create the search account and API key.** Put the key in `.env` as `BRAVE_SEARCH_API_KEY`.
      Until then a search fails with "BRAVE_SEARCH_API_KEY is not set". Reading a page by URL
      needs no key.
- [ ] **Confirm the cost per search.** Placeholder in `budget.yaml`: ₹0.45 per search (about $5 per
      1,000 queries). Each search is charged to the ₹500 monthly research budget before it runs.
      Set this to what the chosen plan really costs.

## C. Needed before anything public is produced

- [ ] **Write the RoundtableCI product profile.** `configs/examples/roundtableci/profile.yaml` has
      "TO BE SUPPLIED" as the product summary, no features and no approved claims. Agents may only
      state what is in this file. With it empty, the content role is told to write pieces that
      are useful on their own and say nothing about the product.
- [ ] **Approve the profile.** Set `approval.status: approved` with `approvedBy` and `approvedOn`.
      While it is a draft, every public-facing action is refused.
- [ ] **Choose the first publishing channel and its rules (decisions 08 and 12).** Placeholder:
      nothing is published. The distribution role prepares a publication package and a person
      publishes it. In the policy file every publishing action needs a person's approval.
- [ ] **Decide who receives budget alerts, and how.** Placeholder: the 70% and 90% alerts are
      written to the log and to the event record. Nobody is emailed or messaged.

## D. Needed before results can be measured

- [ ] **Decide the activation event (decision 05).** Placeholder: `activationEvent: null`.
- [ ] **Create a product analytics account** (PostHog or equivalent) and decide what it tracks.
      Placeholder: the analytics role records the campaign and how to measure it, and is
      instructed to report "No results yet" rather than any number.

## E. Team decisions still open (deck slide 18)

- [ ] 01 Open-source licence and contributor policy. Placeholder: no `LICENSE` file; the code is
      not yet public.
- [ ] 03 Temporal or another durable engine. Placeholder: built and tested on Temporal.
- [ ] 04 Hosting provider and topology. Placeholder: local Docker only.
- [ ] 06 Competitor list and audience segments. Placeholder: the deck's five audience hypotheses;
      empty competitor list. The Competitor Analyst needs a competitor named in its task input.
- [ ] 09 Image and video generation providers. Placeholder: none; no code depends on it yet.
- [ ] 10 Email provider and consent model. Placeholder: none; no email is sent.
- [ ] 13 Dashboard in v1, or CLI and API first. Placeholder: CLI and API built first.

## F. Numbers chosen without you, to check

These are in the role files under `configs/roles/logical/`. The deck gives no figures for them.

- [ ] Per-task spending caps: ₹25 for research roles, ₹10 for the strategist and the analyst,
      ₹40 for the content writer, ₹15 for distribution.
- [ ] Step limits: 12 model calls for research roles, 8 for the content writer, 4 for the rest.
- [ ] Which actions count as public (refused under a draft profile): anything starting with
      `publish`, `newsletter`, `site`, `reply` or `outreach`.

## G. Checks nobody has been able to run yet

- [ ] **Run the tests under the race detector.** `go test -race ./...` needs a C compiler, which
      this machine does not have. Run it once on a machine that does (any Linux box, or CI).
- [ ] **Keep Temporal private in production.** The local setup has no Temporal authentication.
      Anyone who can reach Temporal can signal a workflow without the API token. Locally every
      service now listens on this machine only; a real deployment must do the equivalent.

## H. On your machine

- [ ] **Commit and push the work.** Nothing has been committed since the scaffold.
- [ ] **Check the automated checks on GitHub.** The CI file now starts PostgreSQL for the storage
      tests. That change has not run on GitHub yet.
- [ ] **Know what is running in Docker.** Three local containers were started for testing:
      PostgreSQL (port 55432), Temporal (7233) and object storage (8333), each reachable from this
      machine only. Stop them with
      `docker compose -f deploy/docker-compose.yml down`.
- [ ] **Another PostgreSQL already uses port 5432 on this machine.** It was left untouched. Set
      `MERLIN_POSTGRES_PORT=55432` when starting the local services here, and use that port in
      `MERLIN_DATABASE_URL`.
- [ ] **Docker Hub downloads fail on this network.** Images were pulled from Google's mirror
      (`mirror.gcr.io/...`) and tagged locally. A new machine on this network needs the same.
