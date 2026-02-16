# Jira Integration + Task Dispatch — Status

**Date:** 2026-02-14
**Status:** ✅ Complete (compiles, ready for testing)

## What was built

### 1. `internal/jira/models.go` — Jira data types
- Project, Board, Sprint, Issue, Status, User, Priority, IssueType, Transition, Comment
- ADF (Atlassian Document Format) with `PlainText()` extraction
- `Config` struct with `IsConfigured()` helper

### 2. `internal/jira/client.go` — Jira REST API client
- Basic auth via email + API token (base64 encoded)
- `ListProjects()`, `ListBoards()`, `ListSprints()`
- `SearchIssues()` with JQL support
- `GetIssue()`, `AssignIssue()`, `GetTransitions()`, `TransitionIssue()`
- `AddComment()` with ADF body format
- 15s HTTP timeout, proper error handling

### 3. `internal/dispatch/dispatch.go` — Task dispatch system
- Thread-safe JSON file store at `~/.pulse/dispatch.json`
- `Assign()`, `UpdateStatus()`, `ForTarget()`, `All()`, `Remove()`
- Statuses: pending, in-progress, done, failed
- Deduplicates by issue+target

### 4. `jira_view.go` — TUI Jira view (bubbletea)
- Three-level navigation: Projects → Issues → Detail
- Issue list with status coloring, assignee, summary
- Detail view with all fields + description
- Dispatch overlay: select a host to assign an issue to
- Graceful "not configured" screen when credentials missing
- Breadcrumb navigation

### 5. Updated `tui.go` — Tab system
- Tab bar with `1:Hosts` / `2:Jira` tabs
- Switch via `tab`, `1`, `2` keys
- Jira messages forwarded correctly

### 6. Updated `config.go` + `main.go` — Config & wiring
- New config fields: `jira_url`, `jira_email`, `jira_token`, `dispatch_file`
- Env vars: `JIRA_URL`, `JIRA_EMAIL`, `JIRA_API_TOKEN` (override config)
- Dispatch store initialized on startup

## Configuration

### Environment variables (recommended):
```bash
export JIRA_URL=https://zonitrnd.atlassian.net
export JIRA_EMAIL=your@email.com
export JIRA_API_TOKEN=your-api-token
```

### Or in `hosts.yaml`:
```yaml
jira_url: https://zonitrnd.atlassian.net
jira_email: your@email.com
jira_token: your-api-token
dispatch_file: ~/.pulse/dispatch.json
```

## Key Bindings (Jira view)
- `j/k` — navigate
- `enter/l` — drill into project/issue
- `esc/h/backspace` — go back
- `d` — dispatch issue to a host
- `r` — refresh
- `tab/1/2` — switch between Hosts and Jira tabs
