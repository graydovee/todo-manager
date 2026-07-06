---
name: todo-cli-manage-todos
description: "Manage todos end-to-end with the `todo-cli` command-line client — the primary skill for any todo workflow in this repo. Use it whenever Codex is asked to create, list, filter, inspect, update, or delete todos; change a todo's state (start, complete, reopen, pin, highlight); add or read comments; record progress updates; browse the tag vocabulary; or view the dependency graph. Trigger it for concrete todo requests like 'add a bug for auth rotation', 'show me what's still open', 'mark TASK-12 done', 'leave a blocking note', 'record the latest progress on TASK-7' (记一下进度 / 更新最新进展), or 'what depends on this'. It also takes care of one-time setup — installing `todo-cli` and running `todo-cli login` — only when those are missing, and returns JSON by default so output stays script-friendly."
---

# Todo Cli Manage Todos

## Overview

This skill manages todos — creating, listing, updating, changing lifecycle state, commenting, and exploring tags and dependencies — using the `todo-cli` command-line client as the interface. The CLI keeps operations scriptable and returns JSON by default, which makes results easy for both you and downstream automation to consume.

## Workflow

1. Detect, then act. Probe for `todo-cli` on PATH and for the config file; run install or login ONLY when the matching probe fails. These are one-time setup steps — never repeat them just to be safe. See [Install And Init](#install-and-init) for the exact checks.
2. Choose the narrowest `todo-cli` subcommand that matches the task.
3. Default to JSON output unless the user explicitly wants human-formatted shell output.
4. Quote shell arguments conservatively when generating commands for titles, descriptions, comments, or API keys.

## Install And Init

Install and login are one-time setup. Both are comparatively heavy — `go install` compiles from source and `login` needs an API key and rewrites config — so gate them on a cheap presence check rather than running them on every session. The pattern is detect-then-act: probe first, and run the heavy step only when the probe fails.

### Detect `todo-cli`, install only if missing

```bash
# Fast presence check; the install runs ONLY when the binary is absent.
command -v todo-cli >/dev/null 2>&1 || go install github.com/graydovee/todo-manager/todo-cli@latest
```

Do not reinstall when the binary is already on PATH. The only reasons to run `go install ...@latest` again are a missing binary or an explicit user request to upgrade.

### Detect config, login only if missing or invalid

`login` rewrites `~/.todo-manager/config.yaml`, so run it ONLY when the file is missing, `todo-cli config validate` fails, or the user wants to replace credentials. Check existing config before deciding to log in:

```bash
todo-cli config view
todo-cli config validate
```

Write config from a provided key:

```bash
todo-cli login --api-key 'tdk_xxx'             # first run: creates the "default" profile and sets it as default
# or from stdin:
printf 'tdk_xxx\n' | todo-cli login
# add or update an additional named profile:
todo-cli login -u work --api-key 'tdk_yyy' --base-url 'https://work.example.com'
```

The CLI supports **multiple user profiles**. `login` without `-u` bootstraps the `default` profile (only allowed when no default exists yet); `login -u <name>` adds or overwrites a named profile. Select the profile per command with `-u`/`--user` (defaults to `auth.default_user`). Manage profiles with `config user list|set-default|remove|rename`.

Important defaults:

- Config file path: `~/.todo-manager/config.yaml` (multi-user `auth:` block; a legacy flat config auto-migrates to the `default` profile on first run)
- Default base URL: `https://todo.qaer.io`
- Environment overrides (applied to the selected user for the current run only):
  - `TODO_MANAGER_API_KEY`
  - `TODO_MANAGER_BASE_URL`
- Successful `login` writes config and prints the target user's YAML to stdout.

## Command Selection

- Use `todo-cli todos list` for discovery and filtering.
- Use `todo-cli todos get <id>` for full detail on one todo.
- Use `todo-cli todos create` for new items.
- Use `todo-cli todos update <id>` for title, description, category, priority, due date, tags, dependencies, or duplicate target updates.
- Use `todo-cli todos status|start|complete|reopen|pin|highlight` for lifecycle changes.
- Use `todo-cli todos comments create <id> --content '…'` when the user wants to **record progress** — phrases like 'update the progress', '记一下进度', '最新进展是 …', or 'where is TASK-7 at now' should land as a comment, not as edits to the title or description. See [Progress Updates](#progress-updates).
- Use `todo-cli todos comments ...` for other comment operations (list, delete).
- Use `todo-cli todos tags` to inspect the tag vocabulary.
- Use `todo-cli todos graph` when the dependency graph matters.
- Use `todo-cli todos by-date-range --start-date YYYY-MM-DD --end-date YYYY-MM-DD` for updated-date filtering.

Prefer specific mutation commands over generic `update` when the user intent is status-only or pin/highlight-only.

## Progress Updates

When the user wants to capture where a todo currently stands — 'update the progress', 'record the latest', '记一下进度', '更新最新进展', '进度大概 80%' — add a comment rather than rewriting fields.

The reasoning matters here. A todo's `title` and `description` describe **what the work is**, so they should stay stable enough that the item stays recognizable over its lifetime. `status` tracks lifecycle (open / in_progress / completed). Progress, by contrast, is **chronological** — 'auth flow implemented, waiting on backend review', '约 80%，剩联调'. Each update is a moment in time, and overwriting the description with every new status erases that history. Comments are timestamped and append-only, so they naturally form a progress timeline that anyone can scroll back through with `todos comments list`.

Concretely:

- 'Record progress / 记一下进度 / 最新进展是 …' → `todo-cli todos comments create <id> --content '…'`.
- Edit the `--description` only when the user is redefining the **scope or nature** of the work, not narrating how far along it is.
- If the progress update also crosses a lifecycle boundary (e.g. 'done — testing finished'), pair the comment with the matching lifecycle command (`complete`, `reopen`, …) instead of encoding the state change in prose alone.

## Output Rules

- The CLI defaults to JSON output (pretty/indented, still machine-parseable). Keep that default unless the user explicitly asks for something else.
- `--output`/`-o` accepts `yaml` or `json` (both pretty); `pretty` is kept as a backward-compatible `json` alias. Use `-o yaml` for human-readable data.
- `config view` defaults to **YAML**; pass `config view -o json` when you need to parse its output programmatically.
- Do not invent table output; this CLI does not provide it.
- When showing commands to the user, emit ready-to-run shell commands, not pseudocode.

## Enum Values

The backend rejects values outside these sets, so always use them verbatim. Do not invent synonyms (no `pending`, no `todo`, no `done`, no `high`).

- **Status** (`--status`, `todos status --status …`): `open`, `in_progress`, `completed`, `duplicate`.
  - `open` is the unstarted state — when a user asks for "未开始" / "not started" / "pending" / "todo", filter on `open`.
  - `in_progress` is active work — "进行中" / "in progress" / "doing".
  - `completed` is finished — "已完成" / "done".
  - `duplicate` is set automatically by `--duplicate-of`; rarely filtered on directly.
  - "Unfinished / outstanding / 未完成" means everything except `completed` and `duplicate`. Pass `--status 'open,in_progress'` to `todos list` (the flag accepts a comma-separated set).
- **Category** (`--category`): `bug`, `feature`, `task`. Immutable after creation.
- **Priority** (`--priority`): `p0`, `p1`, `p2`, `p3`. Lower number is higher priority; `p2` is the default when omitted.

## Flags And Data Conventions

- `--due-at` and `--updated-after` use RFC3339.
- `--start-date` and `--end-date` use `YYYY-MM-DD`.
- Repeat `--tag` for multiple tags.
- Repeat `--depends-on` for multiple dependency IDs.
- Use `--duplicate-of <id>` to mark duplicate relationships.
- `todo-cli login` accepts `--api-key`, optional `--base-url`, and optional `-u`/`--user` (profile name). Select the active profile on any command with `-u`/`--user`.

For detailed command examples and flag mappings, read [references/commands.md](references/commands.md).

## Failure Handling

- If `command -v todo-cli` reports the binary is missing, install it once with `go install` (see Install And Init). Do not reinstall when the binary is already on PATH.
- If config is missing or invalid, run `todo-cli login` instead of trying to patch config manually unless the user explicitly wants manual YAML editing.
- If a command returns auth or permission errors, surface the exact JSON error output and suggest re-running `todo-cli login` only when the credentials are likely stale or missing.
- If a user asks for operations outside todo scope, note that this skill only covers `todo-cli` todo workflows, not summary APIs.

## References

- Read [references/commands.md](references/commands.md) for concrete install, login, query, mutation, and comment command examples.
