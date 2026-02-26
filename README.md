# PiaFlow (Go)

Minimal CI/CD system in Go with app-level access control by groups.
Supports SQLite (dev) and MySQL (prod).

## Quick Start

```bash
make tidy
make run-dev
```

Server runs at `http://localhost:8080`.

Default login:
- Username: `admin`
- Password: `admin`

You can override bootstrap admin credentials with:
- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`

## Run Modes

- `make run-dev` (or `make run`): uses SQLite at `data/cicd.db`
- `make run-prod`: uses MySQL (set `DB_DSN`)

MySQL example:

```bash
export DB_DSN='user:password@tcp(host:3306)/dbname?parseTime=true'
make run-prod
```

## Authentication and Access Model

- Login is required for API and UI features.
- Sessions are cookie-based (`HttpOnly`).
- `admin` users can manage users, groups, app-group bindings, and app lifecycle.
- Non-admin users can:
  - view only apps allowed by their groups
  - run allowed apps
  - edit allowed apps
  - view only runs of allowed apps
  - change their own password

## Web UI

Main pages:
- `/login.html` — login
- `/` — recent runs
- `/apps.html` — apps list and app actions
- `/profile.html` — current user profile (name, groups, accessible repos/apps, change password)
- `/access.html` — admin-only access management (users, groups, app permissions)
- `/group.html?group_id=<id>` — admin-only group detail editor (members and apps)
- `/docs.html` — project docs page

Notes:
- The `Access` link is hidden for non-admin users.
- If a non-admin directly opens admin-only pages, content is not shown.

## Pipeline Behavior

Each run executes (in order):
1. `test_cmd`
2. `build_cmd`
3. `deploy_cmd` (optional)

Before steps, PiaFlow clones or pulls the app repository into `work/<app_id>/`.
If any step fails, run status becomes `failed`.

## App Configuration

Apps are stored in `config/apps.yaml`.

Example:

```yaml
apps:
  - id: my-service
    name: My Service
    repo: https://github.com/org/my-service.git
    branch: main
    build_cmd: go build -o bin/app .
    test_cmd: go test ./...
    deploy_cmd: ""
```

## API

All API routes are under `/api`.

### Health

- `GET /health`

### Auth

- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `PUT /api/auth/password` (change current user password)
- `GET /api/auth/profile` (current user profile, groups, accessible apps/repos)

### Apps

- `GET /api/apps`
- `POST /api/apps` (admin)
- `GET /api/apps/{appID}`
- `PUT /api/apps/{appID}` (admin or allowed non-admin)
- `DELETE /api/apps/{appID}` (admin)
- `GET /api/apps/{appID}/groups` (admin)
- `PUT /api/apps/{appID}/groups` (admin)
- `POST /api/apps/{appID}/run`

### Runs

- `GET /api/runs?app_id=&limit=&offset=&page=`
- `GET /api/runs/{id}`

### Users (admin)

- `GET /api/users`
- `POST /api/users`
- `PUT /api/users/{userID}/groups`
- `PUT /api/users/{userID}/password`
- `DELETE /api/users/{userID}` (admin users cannot be deleted)

### Groups (admin)

- `GET /api/groups`
- `POST /api/groups`
- `GET /api/groups/{groupID}`
- `PUT /api/groups/{groupID}/users`
- `PUT /api/groups/{groupID}/apps`

## Data

SQLite default file: `data/cicd.db`

Main tables:
- `runs`
- `users` (`is_admin` included)
- `groups`
- `user_groups`
- `app_groups`

Important behavior:
- Deleting an app also deletes all runs for that app.

## Commands

- `make run` — run server
- `make build` — build binary to `bin/cicd`
- `make test` — run tests
- `make tidy` — `go mod tidy`

## Flags

When running `bin/cicd` or `go run ./cmd/cicd`:

- `-config` (default: `config/apps.yaml`)
- `-db` (default: `data/cicd.db`)
- `-work` (default: `work`)
- `-addr` (default: `:8080`)
- `-static` (default: `web`)

## Documentation

- [CODE.md](CODE.md) — package/file/function reference
- `/docs.html` — architecture and API docs in the web UI
