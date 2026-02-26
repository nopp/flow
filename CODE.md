# Code documentation — PiaFlow

This document is a complete reference of the codebase: every package, file, type, and function with a short description. Use it to navigate the code and understand responsibilities.

---

## Repository layout

```
piaflow/
├── cmd/cicd/          # Entry point
├── config/            # apps.yaml (data, not code)
├── data/              # SQLite DB (created at runtime)
├── internal/          # Go packages
│   ├── auth/          # Password hashing (seed)
│   ├── config/        # YAML load/save for apps
│   ├── pipeline/      # Clone, test, build, deploy
│   ├── seed/          # Default group & admin user
│   ├── server/        # HTTP API & static serving
│   └── store/         # SQLite persistence
├── web/               # Frontend (HTML, CSS, JS)
├── work/              # Clone directory (created at runtime)
├── CODE.md            # This file
├── README.md
├── PROMPTS.txt
└── go.mod
```

---

## cmd/cicd

### main.go

| Symbol   | Kind   | Description |
|----------|--------|-------------|
| `main`   | func   | Entry point. Parses flags (`-config`, `-db`, `-work`, `-static`, `-addr`), creates data and work dirs, loads apps via `config.LoadApps`, opens `store.New`, runs `seed.Run`, creates `pipeline.NewRunner` and `server.New`, then `http.ListenAndServe`. |

---

## internal/config

Loads and saves application definitions from/to YAML (e.g. `config/apps.yaml`). Used at startup and when the API creates/updates/deletes apps.

### config.go

| Symbol      | Kind   | Description |
|-------------|--------|-------------|
| `App`       | struct | One application: `ID`, `Name`, `Repo`, `Branch`, `BuildCmd`, `TestCmd`, `DeployCmd`. YAML and JSON tags for file and API. |
| `AppsConfig`| struct | Root of apps.yaml: `Apps []App`. |
| `LoadApps`  | func   | Reads YAML at path, unmarshals into `AppsConfig`, returns `cfg.Apps`. |
| `SaveApps`  | func   | Marshals `apps` to YAML and writes to path. Used by server on create/update/delete app. |

---

## internal/store

SQLite persistence. Creates tables on first open (runs, users, groups, user_groups, app_groups). Runs are used by the API; users/groups tables are used only by the seed.

### store.go

| Symbol              | Kind   | Description |
|---------------------|--------|-------------|
| `Run`               | struct | One pipeline run: `ID`, `AppID`, `Status`, `CommitSHA`, `Log`, `StartedAt`, `EndedAt`. Status: pending, running, success, failed. |
| `Store`             | struct | Holds `*sql.DB`. Single entry point for all DB access. |
| `New`               | func   | Opens DB at path, runs `migrate`, returns Store. |
| `migrate`           | func   | CREATE TABLE IF NOT EXISTS for runs, users, groups, user_groups, app_groups; indexes on runs(app_id), runs(started_at). |
| `CreateRun`         | func   | Inserts run with status 'pending', returns ID. |
| `UpdateRunLog`      | func   | Updates only the log column (for streaming). |
| `UpdateRunStatus`   | func   | Sets status and log; if success/failed, also sets ended_at. |
| `GetRun`            | func   | Returns run by ID or nil. |
| `ListRuns`          | func   | Returns runs, optional filter by appID, limit (default 50), ordered by started_at DESC. |
| `User`              | struct | id, username, group_ids (for seed/API). |
| `Group`             | struct | id, name. |
| `CreateUser`        | func   | Inserts user (passwordHash must be bcrypt). |
| `UserByID`          | func   | Returns user by ID with GroupIDs filled. |
| `UserByUsername`    | func   | Returns user by username. |
| `UserPasswordHash`  | func   | Returns password_hash for user ID. |
| `UserGroupIDs`      | func   | Returns group IDs for a user. |
| `SetUserGroups`     | func   | Replaces user_groups for a user. |
| `ListUsers`         | func   | All users with GroupIDs. |
| `CreateGroup`       | func   | Inserts group, returns ID. |
| `ListGroups`        | func   | All groups. |
| `AppGroupIDs`       | func   | Group IDs for an app. |
| `SetAppGroups`      | func   | Replaces app_groups for an app. |
| `AppIDsByUserGroupIDs` | func | App IDs that belong to any of the given group IDs. |
| `Close`             | func   | Closes the DB connection. |

---

## internal/pipeline

Executes the CI/CD steps for an app: clone (or pull), test, build, deploy. Working directory is `workDir/<app.ID>/`. Commands (TestCmd, BuildCmd, DeployCmd) are parsed with `splitCommand` (supports quotes).

### pipeline.go

| Symbol         | Kind   | Description |
|----------------|--------|-------------|
| `Runner`       | struct | Holds `workDir` (e.g. work/). Each app uses workDir/app.ID. |
| `NewRunner`    | func   | Returns Runner with given workDir. |
| `Result`       | struct | `Success bool`, `Log string`. |
| `Run`          | func   | Runs clone/pull → test → build → deploy. Calls `onLogUpdate(log)` after each log write for streaming. Returns Result. |
| `runCmd`       | func   | Runs command in dir; stdout/stderr to process (for git). |
| `runCmdWithLog`| func   | Runs shell command in dir, stdout/stderr to buffer (for test/build/deploy). |
| `output`       | func   | Runs command in dir, returns stdout. |
| `splitCommand` | func   | Splits command string into parts, respecting single/double quotes. |

---

## internal/server

HTTP API and static file serving. Chi router; app list protected by RWMutex; config changes persisted via config.SaveApps; runs executed by pipeline Runner and stored via Store.

### server.go

| Symbol       | Kind   | Description |
|--------------|--------|-------------|
| `Server`     | struct | `appsMu`, `apps`, `store`, `runner`, `appsPath`, `staticDir`. |
| `New`        | func   | Builds Server with given deps and absolute paths. |
| `Handler`    | func   | Returns Chi router: Logger, Recoverer; GET /health; /api/apps (GET, POST), /api/apps/:appID (GET, PUT, DELETE), POST /api/apps/:appID/run; GET /api/runs, GET /api/runs/:id; GET /* → serveStatic. |
| `serveStatic`| func   | / or "" → index.html; else file under staticDir; missing path → index.html (SPA). |
| `health`     | func   | 200 "ok". |
| `listApps`   | func   | Returns [{id, name}] for all apps (read lock). |
| `getApp`     | func   | Returns full app by appID or 404. |
| `createApp`  | func   | Decodes JSON, validates id/name/repo and test_cmd/build_cmd, default branch main, duplicate check, append, SaveApps, 201. |
| `updateApp`  | func   | Decodes JSON, find by appID, replace, SaveApps, 200. |
| `deleteApp`  | func   | Remove app from slice, SaveApps, 204. |
| `triggerRun` | func   | Find app, CreateRun, goroutine: UpdateRunStatus(running), Runner.Run with onLogUpdate → UpdateRunLog, then UpdateRunStatus(success/failed). Responds 202 {run_id, status}. |
| `listRuns`   | func   | Query app_id, limit; ListRuns; JSON. |
| `getRun`     | func   | Parse id, GetRun, 200 or 404. |
| `writeJSON`  | func   | Content-Type, WriteHeader(status), Encode(v). |

---

## internal/seed

Initializes DB at startup: default group, admin user (password admin) if no users, and assigns apps without groups to the default group. No HTTP API for this; idempotent.

### seed.go

| Symbol | Kind | Description |
|--------|-----|-------------|
| `Run`  | func | ListGroups; if none, CreateGroup("default"). ListUsers; if none, HashPassword("admin"), CreateUser("admin", hash), SetUserGroups(adminID, defaultGroupID). For each app, if AppGroupIDs empty, SetAppGroups(app.ID, defaultGroupID). |

---

## internal/auth

Password hashing for the seed (default admin user). Bcrypt cost 10.

### auth.go

| Symbol        | Kind | Description |
|---------------|-----|-------------|
| `bcryptCost`  | const | 10. |
| `HashPassword`| func | Returns bcrypt hash of password. |

---

## web (frontend)

Static files served at /. No build step; vanilla HTML, CSS, JS.

### web/index.html

- One screen: header (logo, tagline, Docs link), main (server-error, Apps section with add-app btn and apps-grid, Recent runs with refresh btn and runs-container).
- Overlays: log-overlay (log title, close, pre for log content); app-form-overlay (form title, close, form: id, name, repo, branch, test_cmd, build_cmd, deploy_cmd, Cancel, Save).
- Script: /js/app.js. Styles: DM Sans, /css/style.css.

### web/js/app.js

| Symbol / block      | Description |
|---------------------|-------------|
| `API`               | '/api'. |
| `fetchApi(path, options)` | fetch(API + path, method, headers, body). |
| `getApps`           | GET /api/apps, return JSON. |
| `triggerRun(appId)`  | POST /api/apps/:id/run, return JSON. |
| `getRuns`, `getRun`  | GET /api/runs, GET /api/runs/:id. |
| `createApp`, `updateApp`, `deleteApp` | POST/PUT/DELETE apps. |
| `showToast(msg, type)` | Toast notification (success/error). |
| `statusClass(status)`| CSS class for run status badge. |
| `formatDate(iso)`    | Relative time or formatted date. |
| `escapeHtml(s)`      | Escape for safe HTML. |
| `renderApps(container, apps)` | Cards with Run / Edit / Delete; wire click handlers. |
| `renderRuns(container, runs)` | Table with expand btn, status, started; expand row = inline log; polling for log when expanded and run pending/running; runs list polling when any pending/running. |
| `loadApps`, `loadRuns` | Fetch and render. |
| `openAppForm(appId)`, `closeAppForm` | Show/hide form overlay; if appId load getApp and fill. |
| Form submit          | Build app object, createApp or updateApp, toast, close, loadApps. |
| `confirmDeleteApp`   | Confirm, deleteApp, toast, loadApps. |
| `initApp`            | Promise.all(loadApps, loadRuns). |
| `init`               | checkServerReachable (/health), then initApp or show server-error. |
| Polling              | runsListPollInterval 2s while any run pending/running; inline log poll 1.5s for expanded run until success/failed. |

### web/css/style.css

- CSS variables: --bg, --bg-card, --border, --text, --text-muted, --accent, --success, --error, --radius, --font (DM Sans).
- Layout: header, main sections, apps grid (cards), runs table, overlays/modals (log, app form).
- Components: buttons (btn, btn-primary, btn-ghost, btn-icon), form inputs, badges (run status), toast, empty state.
- Responsive and focus/aria where relevant.

---

## Other files

| File                    | Description |
|-------------------------|-------------|
| `web/docs.html`         | Full user-facing documentation (architecture, API, pipeline, frontend). Linked from UI header as "Docs". |
| `PROMPTS.txt`           | Log of change requests and implementations. |
| `PROMPT-CRIAR-APP-DO-ZERO.txt` | Single prompt to recreate the app from scratch. |
| `config/apps.yaml`      | Default app list (YAML). |
| `go.mod`                | Go module and dependencies (chi, yaml, sqlite3, bcrypt). |

---

## Data flow summary

1. **Startup**: main → LoadApps(config) → Store.New(db) → seed.Run(store, apps) → NewRunner(work) → server.New(apps, store, runner, configPath, staticPath) → ListenAndServe(Handler()).
2. **List apps**: GET /api/apps → listApps → read lock apps → JSON [{id, name}].
3. **Create app**: POST /api/apps → createApp → decode, validate, lock, append, SaveApps(path, newApps), unlock → 201.
4. **Trigger run**: POST /api/apps/:id/run → triggerRun → CreateRun(appID) → goroutine: UpdateRunStatus(running), Runner.Run(app, onLogUpdate), UpdateRunStatus(success/failed) → 202 {run_id}.
5. **Streaming log**: onLogUpdate in Run() calls store.UpdateRunLog(runID, log); UI polls GET /api/runs/:id and updates expanded row.
6. **Static**: GET /* → serveStatic → file from web/ or index.html.

For more detail on behaviour and API contracts, see `web/docs.html`.
