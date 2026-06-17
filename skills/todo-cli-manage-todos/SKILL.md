---
name: todo-cli-manage-todos
description: "Manage todos end-to-end with the `todo-cli` command-line client — the primary skill for any todo workflow in this repo. Use it whenever Codex is asked to create, list, filter, inspect, update, or delete todos; change a todo's state (start, complete, reopen, pin, highlight); add or read comments; browse the tag vocabulary; or view the dependency graph. Trigger it for concrete todo requests like 'add a bug for auth rotation', 'show me what's still open', 'mark TASK-12 done', 'leave a blocking note', or 'what depends on this'. It also takes care of one-time setup — installing `todo-cli` and running `todo-cli login` — only when those are missing, and returns JSON by default so output stays script-friendly."
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
todo-cli login --api-key 'tdk_xxx'
# or from stdin:
printf 'tdk_xxx\n' | todo-cli login
```

Important defaults:

- Config file path: `~/.todo-manager/config.yaml`
- Default base URL: `https://todo.qaer.io`
- Environment overrides:
  - `TODO_MANAGER_API_KEY`
  - `TODO_MANAGER_BASE_URL`
- Successful `login` writes config and prints YAML to stdout.

## Command Selection

- Use `todo-cli todos list` for discovery and filtering.
- Use `todo-cli todos get <id>` for full detail on one todo.
- Use `todo-cli todos create` for new items.
- Use `todo-cli todos update <id>` for title, description, category, priority, due date, tags, dependencies, or duplicate target updates.
- Use `todo-cli todos status|start|complete|reopen|pin|highlight` for lifecycle changes.
- Use `todo-cli todos comments ...` for comment operations.
- Use `todo-cli todos tags` to inspect the tag vocabulary.
- Use `todo-cli todos graph` when the dependency graph matters.
- Use `todo-cli todos by-date-range --start-date YYYY-MM-DD --end-date YYYY-MM-DD` for updated-date filtering.

Prefer specific mutation commands over generic `update` when the user intent is status-only or pin/highlight-only.

## Output Rules

- The CLI defaults to JSON output. Keep that default unless the user explicitly asks for something else.
- Use `--output pretty` only when the user wants prettier JSON for reading.
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
- `todo-cli login` accepts `--api-key` and optional `--base-url`.

For detailed command examples and flag mappings, read [references/commands.md](references/commands.md).

## Failure Handling

- If `command -v todo-cli` reports the binary is missing, install it once with `go install` (see Install And Init). Do not reinstall when the binary is already on PATH.
- If config is missing or invalid, run `todo-cli login` instead of trying to patch config manually unless the user explicitly wants manual YAML editing.
- If a command returns auth or permission errors, surface the exact JSON error output and suggest re-running `todo-cli login` only when the credentials are likely stale or missing.
- If a user asks for operations outside todo scope, note that this skill only covers `todo-cli` todo workflows, not summary APIs.

## References

- Read [references/commands.md](references/commands.md) for concrete install, login, query, mutation, and comment command examples.
