# PiaFlow Code Reference

This file documents the current code structure and main symbols.

## Repository Layout

```text
piaflow/
├── cmd/cicd/              # Application entrypoint
├── internal/
│   ├── auth/              # Password hash/check helpers
│   ├── config/            # apps.yaml load/save
│   ├── pipeline/          # Clone/pull + test/build/deploy runner
│   ├── server/            # HTTP API + static serving + auth/session
│   └── store/             # Persistence (runs/users/groups mappings)
├── web/                   # Static frontend (HTML/CSS/JS)
├── config/apps.yaml       # App definitions
└── README.md
```

## cmd/cicd

### `main.go`

- Parses flags (`-config`, `-db`, `-work`, `-static`, `-addr`)
- Resolves DB driver (`sqlite3` default, `mysql` via env)
- Loads apps from YAML
- Opens store and runs migrations
- Ensures bootstrap admin user (`ADMIN_USERNAME` / `ADMIN_PASSWORD`, default `admin/admin`)
- Starts HTTP server with `server.New(...).Handler()`

## internal/auth

### `auth.go`

- `HashPassword(password) (string, error)`  
  Returns prefixed SHA-256 hash string.
- `CheckPassword(password, hash) bool`  
  Validates plaintext against stored hash.

## internal/config

### `config.go`

- `type App`  
  App config model used by YAML and API JSON.
- `LoadApps(path)`  
  Reads `apps.yaml`.
- `SaveApps(path, apps)`  
  Persists app list to YAML.

## internal/store

### `store.go`

Core entities:
- `Run`
- `User` (`IsAdmin` included)
- `Group`

Core methods:
- Runs: `CreateRun`, `UpdateRunLog`, `UpdateRunStatus`, `GetRun`, `ListRuns`, `CountRuns`, `ListRunsByAppIDs`, `CountRunsByAppIDs`, `DeleteRunsByAppID`
- Users: `CreateUser`, `GetUser`, `GetUserByUsername`, `ListUsers`, `UpdateUserPassword`, `DeleteUser`, `EnsureAdminUser`
- Groups and mappings:
  - `CreateGroup`, `ListGroups`, `GetGroup`
  - `UserGroupIDs`, `SetUserGroups`, `GroupUserIDs`, `SetGroupUsers`
  - `AppGroupIDs`, `SetAppGroups`, `GroupAppIDs`, `SetGroupApps`
  - `AppIDsByUserGroupIDs`

Migrations create:
- `runs`
- `users` (`is_admin`)
- `groups`
- `user_groups`
- `app_groups`

## internal/pipeline

### `pipeline.go`

- `Runner.Run(app, onLogUpdate)` executes:
  1. clone/pull
  2. test
  3. build
  4. deploy (optional)
- Streams log updates through callback.

## internal/server

### `server.go`

Responsibilities:
- Session auth via cookie
- Role-based authorization
- Group-based app visibility/access
- App CRUD + run trigger
- Users/groups/admin operations
- Static file serving

Public HTTP routes:
- Health: `GET /health`
- Auth:
  - `POST /api/auth/login`
  - `POST /api/auth/logout`
  - `GET /api/auth/me`
  - `PUT /api/auth/password`
  - `GET /api/auth/profile`
- Apps:
  - `GET /api/apps`
  - `POST /api/apps`
  - `GET /api/apps/{appID}`
  - `PUT /api/apps/{appID}`
  - `DELETE /api/apps/{appID}`
  - `GET /api/apps/{appID}/groups`
  - `PUT /api/apps/{appID}/groups`
  - `POST /api/apps/{appID}/run`
- Runs:
  - `GET /api/runs`
  - `GET /api/runs/{id}`
- Users (admin):
  - `GET /api/users`
  - `POST /api/users`
  - `PUT /api/users/{userID}/groups`
  - `PUT /api/users/{userID}/password`
  - `DELETE /api/users/{userID}`
- Groups (admin):
  - `GET /api/groups`
  - `POST /api/groups`
  - `GET /api/groups/{groupID}`
  - `PUT /api/groups/{groupID}/users`
  - `PUT /api/groups/{groupID}/apps`

Authorization model:
- Admin: full access
- Non-admin:
  - sees only allowed apps (by group bindings)
  - can run allowed apps
  - can edit allowed apps
  - sees only runs for allowed apps

Deletion behavior:
- Deleting an app also deletes all runs for that app.

## web

### Main pages

- `/login.html`
- `/` (runs)
- `/apps.html`
- `/profile.html`
- `/access.html` (admin)
- `/group.html?group_id=` (admin)
- `/docs.html`

### `web/js/app.js`

Contains:
- API client helpers
- auth/bootstrap checks
- runs/apps rendering and actions
- admin access management flows
- group detail page editor
- profile rendering and self password change

### `web/css/style.css`

Shared theme and component styles used by all pages.
