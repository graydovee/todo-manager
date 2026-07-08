# Todo Manager Desktop

A small, always-on-top todo widget built with [Gio](https://gioui.org). It is a
companion desktop client for the todo-manager backend, designed to live
unobtrusively on your screen.

**Primary platform: Windows.** macOS support is planned (the native
top-most/click-through layer is stubbed for now).

## Features

- **Minimalist greyscale UI** — light background, black text, hairline borders.
  Priorities and statuses are distinguished by weight and shading, not colour.
- **Pin (always-on-top)** — keep the window above everything; UI stays
  interactive.
- **Lock** — pin + click-through + glassy translucent look. While locked, pointer
  events pass through to whatever is behind the window. Unlock from the
  **system tray** menu (the window itself can't receive clicks while locked).
- **Todo list** with sortable header, inline start/complete buttons (completed
  todos show no button), and click-to-open detail.
- **Detail view** — status transitions (start / complete / reopen with dependency
  conflict handling), edit title/description/priority/tags/due date, relations,
  duplicates, and comments.
- **Manage view** — filter by status/category/priority, title/code search, create
  new todos, and log out.
- **System tray** — unlock, toggle top-most, quit.

## Authentication

The desktop client authenticates with a **Bearer API key** (`tdk_...`), the same
mechanism used by `todo-cli`. To obtain a key:

1. Open the web app and log in.
2. In the web app's settings, create an access key.
3. Paste the `tdk_...` value into the desktop client's login screen along with
  the backend URL.

OIDC and username/password login are intentionally not implemented; only the API
key flow is supported.

## Configuration

Stored separately from `todo-cli` at `~/.todo-manager/gui-config.yaml` (0600):

```yaml
base_url: https://todo.qaer.io
api_key: tdk_...
window:
  width: 360
  height: 560
  locked: false
  topmost: false
filters:
  status: [open, in_progress]
  category: []
  priority: []
  sort_by: created_at
  sort_order: desc
```

## Build

From the repository root:

```bash
# Native build (current OS)
make desktop-build        # -> bin/todo-desktop

# Windows binary (no console window)
make desktop-windows      # -> bin/todo-desktop.exe

# Run on the current OS
make desktop-run
```

## Architecture

```
desktop/
├── main.go                     # Gio event loop (goroutine) + app.Main() + tray wiring
└── internal/
    ├── client/                 # Bearer-auth HTTP client (ported from todo-cli)
    ├── config/                 # ~/.todo-manager/gui-config.yaml (independent file)
    ├── platform/               # Native window control + system tray
    │   ├── windows/            #   SetWindowPos / WS_EX_TRANSPARENT / DWM blur
    │   │                       #   + Shell_NotifyIcon tray (own message loop)
    │   └── (stub on !windows)  #   no-op until macOS support lands
    ├── store/                  # App state + todo/comment caches (mutex-guarded)
    └── ui/                     # Minimalist light theme + page layouts
```

The window mode (top-most / lock) is driven through `app.Win32ViewEvent`, which
hands the native `HWND` to a platform controller that issues Win32 calls
(`SetWindowPos`, `SetWindowLongPtrW` with `WS_EX_TRANSPARENT`, and best-effort
`DwmEnableBlurBehindWindow`).

### System tray (native, no third-party dependency)

The tray is implemented directly with Win32 `Shell_NotifyIconW` rather than a
systray library, because the common Go systray libraries call
`runtime.LockOSThread()` at `init()` time, which collides with Gio's own thread
requirements and hangs the window. The tray runs on a dedicated goroutine that
owns a hidden message-only window and its own `GetMessage` loop, completely
isolated from Gio's event loop. Menu actions are forwarded over a channel and
the GUI loop is woken via `w.Invalidate()` so it can drain them promptly.

### Thread model

- The window event loop runs in a goroutine; `app.Main()` owns the main thread
  (per Gio's documented Windows pattern).
- The tray runs on its own goroutine, pinned to a dedicated OS thread via
  `runtime.LockOSThread()` (Win32 windows are thread-affine).
- Network calls run on short-lived goroutines and report back by mutating the
  mutex-guarded stores and calling `w.Invalidate()`.
