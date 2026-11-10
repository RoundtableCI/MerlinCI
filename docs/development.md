# Development

## Requirements

- Go (version in `go.mod`)
- Docker with Compose

## Local dependencies

```sh
cp .env.example .env
docker compose -f deploy/docker-compose.yml up -d
```

| Service | Address |
|---|---|
| PostgreSQL + pgvector | `localhost:5432` (merlin / merlin) |
| Redis | `localhost:6379` |
| Temporal | `localhost:7233` |
| Temporal UI | http://localhost:8233 |
| S3-compatible object storage (SeaweedFS) | http://localhost:8333 |
| Prometheus | http://localhost:9090 |
| Grafana | http://localhost:3000 (admin / admin) |

## Build and check

```sh
go build ./...
go vet ./...
go test ./...
```

Full self-hosting documentation ships with the first release (slide 14).
