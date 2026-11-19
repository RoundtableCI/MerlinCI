# Development

## Requirements

- Go (version in `go.mod`)
- Docker with Compose

## Local services

```sh
cp .env.example .env
docker compose -f deploy/docker-compose.yml up -d --wait postgres
docker compose -f deploy/docker-compose.yml up -d temporal objectstore
```

| Service | Address |
|---|---|
| PostgreSQL + pgvector | `localhost:5432` (merlin / merlin) |
| Temporal | `localhost:7233` |
| Temporal UI | http://localhost:8233 (start `temporal-ui` too) |
| S3-compatible object storage (SeaweedFS) | http://localhost:8333 (merlin / merlin-local-only) |
| Redis | `localhost:6379` (not used yet) |
| Prometheus | http://localhost:9090 (not used yet) |
| Grafana | http://localhost:3000 (admin / admin; not used yet) |

If another PostgreSQL already uses port 5432 on your machine, set `MERLIN_POSTGRES_PORT=55432`
before `docker compose up`, and use that port in `MERLIN_DATABASE_URL`.

The credentials above are for local development only and are in the repository on purpose.
Never use them anywhere else.

## Build and check

```sh
go build ./...
go vet ./...
go test ./...
go run ./cmd/merlinctl validate configs
```

`merlinctl validate` checks every configuration document against its schema in `schemas/`,
then checks the documents against each other (roles, workflows, policy, budget, models). It
prints a digest that identifies the exact configuration.

## Tests that need the local services

Some tests need real services and are skipped without them. Point the tests at the local
services with these variables:

```sh
export MERLIN_TEST_DATABASE_URL='postgres://merlin:merlin@localhost:5432/merlin?sslmode=disable'
export MERLIN_TEST_TEMPORAL_ADDRESS=localhost:7233
export MERLIN_TEST_S3_ENDPOINT=http://localhost:8333
export MERLIN_TEST_S3_ACCESS_KEY_ID=merlin
export MERLIN_TEST_S3_SECRET_ACCESS_KEY=merlin-local-only
go test ./...
```

| Variable | Turns on |
|---|---|
| `MERLIN_TEST_DATABASE_URL` | Storage tests (`internal/store`) and API tests (`internal/api`) |
| `MERLIN_TEST_TEMPORAL_ADDRESS` | The workflow on a real Temporal server (`internal/workflow`) |
| Both of the above | End-to-end tests (`internal/app`): the API, a worker and the configured roles |
| `MERLIN_TEST_S3_ENDPOINT` | Object storage tests (`internal/blob/s3blob`) |

Each database test creates a database of its own and drops it afterwards. The end-to-end tests
take about a minute, because one of them waits for a paused run to notice it has been resumed.

In the end-to-end tests the AI model, the search provider and the public web are stand-ins.
Everything else is real.

## Run MerlinCI locally

Fill in `.env` first: `MERLIN_API_TOKEN` is required, and nothing useful happens without
`OPENROUTER_API_KEY`. See [handjobs.md](../handjobs.md).

```sh
go run ./cmd/merlin-api       # terminal 1: the control plane
go run ./cmd/merlin-worker    # terminal 2: runs the agents
```

Both check the configuration in `MERLIN_CONFIG_DIR` and refuse to start if it has errors.
Both apply any database migrations that have not been applied yet.

Then, in a third terminal:

```sh
go run ./cmd/merlinctl config                     # what is configured and switched on
go run ./cmd/merlinctl run market-researcher -input '{"topic":"comparing model outputs"}' -wait
go run ./cmd/merlinctl events <task>              # everything the task did, in order
go run ./cmd/merlinctl artifact <id> -content     # what it produced
go run ./cmd/merlinctl budget                     # spend against the limits

go run ./cmd/merlinctl campaign first-release -input '{"topic":"comparing model outputs"}' -wait
go run ./cmd/merlinctl campaign-status <campaign>

go run ./cmd/merlinctl approve <task>             # when a task is waiting for a person
go run ./cmd/merlinctl reject <task> -reason "not this week"
go run ./cmd/merlinctl pause                      # stop every run where it is
go run ./cmd/merlinctl resume
```

Run `merlinctl help` for the full list.

Full self-hosting documentation ships with the first release (slide 14).
