# `todo-cli` Command Reference

## Install

```bash
go install github.com/graydovee/todo-manager/todo-cli@latest
```

## Login And Config

The CLI supports **multiple user profiles** stored under `auth.users` in `~/.todo-manager/config.yaml`. The first `login` (no `-u`) bootstraps the `default` profile; `login -u <name>` adds or overwrites a named profile. Select the active profile on any command with `-u`/`--user` (defaults to `auth.default_user`).

Bootstrap the default profile from a provided key:

```bash
todo-cli login --api-key 'tdk_xxx'
```

From stdin:

```bash
printf 'tdk_xxx\n' | todo-cli login
```

Add or update a named profile (overwrites if the name exists):

```bash
todo-cli login -u work --api-key 'tdk_yyy' --base-url 'https://work.example.com'
```

Inspect config (`config view` defaults to YAML — use `-o json` to parse):

```bash
todo-cli config view
todo-cli config view -o json
todo-cli config validate
```

Manage user profiles:

```bash
todo-cli config user list                       # list profiles (masked keys, default flagged)
todo-cli config user set-default work           # set the default profile
todo-cli config user set-default ''             # clear the default (then -u is required)
todo-cli config user rename work office         # rename; fails if the target name already exists
todo-cli config user remove work                # remove a profile (clears default if it was it)
```

A legacy single-user config (flat `api_key`/`base_url`) is auto-migrated to the `default` profile on first run.

## Read Operations

List todos:

```bash
todo-cli todos list
```

Filtered list:

```bash
todo-cli todos list \
  --q 'auth' \
  --status 'open,in_progress' \
  --category bug \
  --priority p1 \
  --tag backend \
  --tag api \
  --sort-by updated_at \
  --sort-order desc
```

Get one todo:

```bash
todo-cli todos get 123
```

List tags:

```bash
todo-cli todos tags
```

Read graph:

```bash
todo-cli todos graph
```

Read by date range:

```bash
todo-cli todos by-date-range --start-date 2026-06-01 --end-date 2026-06-15
```

## Create And Update

Create:

```bash
todo-cli todos create \
  --title 'Add access key rotation' \
  --description 'Support immediate replacement of access keys' \
  --category feature \
  --priority p1 \
  --tag auth \
  --tag cli \
  --due-at '2026-06-20T12:00:00Z'
```

Create with dependencies:

```bash
todo-cli todos create \
  --title 'Finish CLI integration' \
  --category task \
  --depends-on 101 \
  --depends-on 102
```

Update:

```bash
todo-cli todos update 123 \
  --title 'Refine access key rotation' \
  --priority p0 \
  --tag auth \
  --tag api
```

Set duplicate target:

```bash
todo-cli todos update 123 --duplicate-of 88
```

Delete:

```bash
todo-cli todos delete 123
```

## Lifecycle Commands

```bash
todo-cli todos start 123
todo-cli todos status 123 --status in_progress
todo-cli todos complete 123
todo-cli todos complete 123 --cascade-dependencies
todo-cli todos reopen 123
todo-cli todos reopen 123 --cascade-dependents
todo-cli todos pin 123 --value true
todo-cli todos highlight 123 --value true
```

## Comments

List comments:

```bash
todo-cli todos comments list 123
```

Create comment:

```bash
todo-cli todos comments create 123 --content 'Blocked on API key provisioning'
```

Record a progress update (prefer this over rewriting the description — progress is a timestamped timeline, not a single field):

```bash
todo-cli todos comments create 123 --content 'Progress: auth flow implemented, waiting on backend review (~80%)'
```

Delete comment:

```bash
todo-cli todos comments delete 123 45
```
