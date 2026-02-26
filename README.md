# PiaFlow (Go)

Minimal CI/CD system in Go: easy to add apps, easy to maintain, SQLite for data.

## Quick start

```bash
make tidy
make run
```

Server runs at `http://localhost:8080`. Open it in a browser for the **web UI** (apps list, trigger run, recent runs, view run logs). No login required.

## Documentation

- **[CODE.md](CODE.md)** — Code reference: every package, file, type, and function with a short description.
- **Web docs** — In the UI, click **Docs** (or open `/docs.html`) for architecture, API, pipeline, and frontend documentation.
- In-code docs — All Go packages and public symbols have doc comments; use `go doc` or your IDE.

## Pipeline steps

Each run runs **up to three steps** in order:

1. **Test** – runs `test_cmd` (e.g. `go test ./...`)
2. **Build** – runs `build_cmd` (e.g. `go build ...`)
3. **Deploy** – runs `deploy_cmd` if set (optional; leave empty to skip)

Before that, the pipeline clones or pulls the repo. If any step fails, the run stops and is marked as failed. The run log (see `GET /runs/{id}`) shows each step with `=== Step: test ===`, `=== Step: build ===`, `=== Step: deploy ===`.

## Adding apps

Edit **`config/apps.yaml`** and add a new entry under `apps`:

```yaml
apps:
  - id: my-service          # unique id, used in API and DB
    name: My Service        # display name
    repo: https://github.com/org/my-service.git
    branch: main
    build_cmd: go build -o bin/app .
    test_cmd: go test ./...
    deploy_cmd: ""          # optional; leave empty to skip deploy
```

No code changes needed. Restart the server to pick up new apps (or implement a reload endpoint later).

## Web UI

The UI is served at the root URL (`/`). It lists configured apps (with **Add app**, **Edit**, **Delete**, and **Run**), shows recent runs with status, and offers **live log** by clicking **▶** on a run to expand the log inline (updates in real time while the pipeline is running). Apps can be created, edited, and deleted from the UI; changes are persisted to `config/apps.yaml`.

## API

All API routes are under `/api`:

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/apps` | List all apps |
| POST | `/api/apps` | Create app (body: JSON with id, name, repo, branch, test_cmd, build_cmd, deploy_cmd) |
| GET | `/api/apps/{appID}` | Get one app config |
| PUT | `/api/apps/{appID}` | Update app (body: JSON) |
| DELETE | `/api/apps/{appID}` | Delete app |
| POST | `/api/apps/{appID}/run` | Trigger a pipeline run (async) |
| GET | `/api/runs?app_id=&limit=` | List runs (optional filter by app_id) |
| GET | `/api/runs/{id}` | Get run details and log |

## Data

- **SQLite**: `data/cicd.db` (default). Tables: `runs` (id, app_id, status, commit, log, started_at, ended_at).
- **Work dir**: `work/` – clones repos under `work/<app_id>/`.

## Prompts and changes

- **`PROMPTS.txt`**: Contains the original request and a section for change requests. Append each new change request there with date and description.
- **`PROMPT-CRIAR-APP-DO-ZERO.txt`**: Single prompt that describes how to recreate this entire application from scratch (backend, API, UI, config, tests).

## Commands

- `make run` – run the server
- `make build` – build binary to `bin/cicd`
- `make test` – run tests
- `make tidy` – go mod tidy

## Flags (when running `bin/cicd` or `go run`)

- `-config` – path to apps.yaml (default: config/apps.yaml)
- `-db` – SQLite path (default: data/cicd.db)
- `-work` – clone directory (default: work)
- `-addr` – listen address (default: :8080)
- `-static` – directory for web UI files (default: web)
