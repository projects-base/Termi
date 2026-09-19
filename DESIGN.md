# Termi - Design Document

## Overview

Termi is a desktop terminal application for Windows that combines native shell access with a modern web-based UI. It uses a Go backend for system-level operations and a Svelte frontend for the interface, bridged together by the Wails framework.

## Architecture

```
+-------------------+       Wails IPC        +--------------------------------+
|   Go Backend      | <------------------->  |  Svelte Frontend               |
|                   |                        |                                |
|  - Map of PTYs    |   EventsEmit/EventsOn  |  - Array of Xterm.js instances |
|  - File I/O       |   Function Bindings    |  - Tab Bar & Explorer          |
|  - Global History |                        |  - Svelte Stores               |
+-------------------+                        +--------------------------------+
        |                                            |
        v                                            v
   Windows OS                                   WebView2
   (PowerShell xN)                          (Chromium-based)
```

## Tech Stack

### Desktop Framework
| Component | Version | Purpose |
|---|---|---|
| Wails | v2.12.0 | Go-to-Web bridge, native window, IPC, asset embedding |
| WebView2 | v1.0.22 | Chromium-based rendering engine (via go-webview2) |

### Backend (Go)
| Component | Version | Purpose |
|---|---|---|
| Go | 1.23.0 | Backend language |
| go-pty | v0.2.2 | Windows ConPTY pseudo-terminal management |
| creack/pty | v1.1.21 | Cross-platform PTY abstraction (used by go-pty) |
| gorilla/websocket | v1.5.3 | WebSocket communication (Wails internal) |
| labstack/echo | v4.13.3 | HTTP server for dev mode (Wails internal) |
| google/uuid | v1.6.0 | Unique identifiers |

### Frontend (Svelte + Xterm.js)
| Component | Version | Purpose |
|---|---|---|
| Svelte | v3.49.0 | Reactive UI framework |
| Vite | v3.0.7 | Build tool and dev server |
| @sveltejs/vite-plugin-svelte | v1.0.1 | Svelte integration for Vite |
| xterm | v5.3.0 | Terminal emulator UI component |
| xterm-addon-fit | v0.8.0 | Auto-fit terminal to container size |
| xterm-addon-search | v0.13.0 | Search within terminal output |

### System Requirements
| Requirement | Version |
|---|---|
| Windows | 10/11 (ConPTY support required) |
| Node.js | v18+ (build only) |
| Go | v1.23+ (build only) |
| Wails CLI | v2+ (build only) |

## Project Structure

```
Termi/
├── main.go                    # Wails app initialization, window config
├── app.go                     # Core backend: PTY, file ops, history, completions
├── go.mod / go.sum            # Go dependencies
├── wails.json                 # Wails build configuration & app versioning
├── build_release.ps1          # Automated NSIS compiler & code signing pipeline
│
├── frontend/
│   ├── src/
│   │   ├── App.svelte         # Main layout: terminal + explorer + navigation
│   │   ├── TreeNode.svelte    # Recursive file tree with auto-expand
│   │   ├── SearchBar.svelte   # Terminal search overlay (Ctrl+F)
│   │   ├── HistoryPanel.svelte    # Command history panel (Ctrl+K)
│   │   ├── RecentPaths.svelte     # Recent folders panel (Ctrl+Shift+R)
│   │   ├── ShortcutsPanel.svelte  # Keyboard shortcut list (F1)
│   │   ├── SettingsPanel.svelte   # Font, theme and shell settings (Ctrl+,)
│   │   ├── ContextMenu.svelte # Reusable right-click context menu
│   │   ├── Toast.svelte       # Notification toast for long commands
│   │   ├── Autocomplete.svelte    # Command bar auto-complete dropdown
│   │   ├── stores.js          # Svelte writable stores (shared state)
│   │   ├── shortcuts.js       # Shortcut table + global key handler
│   │   ├── ptyWriter.js       # Ordered, chunked PTY input queue per tab
│   │   ├── style.css          # Global styles, dark theme, CSS variables
│   │   └── main.js            # Svelte app bootstrap
│   │
│   ├── wailsjs/               # Auto-generated Wails bindings
│   │   ├── go/main/App.js     # JS wrappers for Go functions
│   │   ├── go/main/App.d.ts   # TypeScript declarations
│   │   ├── go/models.ts       # Go struct type definitions
│   │   └── runtime/           # Wails runtime (events, clipboard, window)
│   │
│   ├── package.json           # NPM dependencies
│   ├── vite.config.js         # Vite build config
│   └── index.html             # Entry HTML
│
└── build/
    ├── bin/                   # Compiled executables
    └── windows/               # Windows build assets (icon, manifest)
```

## Data Flow

### Terminal I/O (Multi-Tab Routed)
```
Keystroke → Active Xterm.js onData → WriteToTerminal(tabId, data) → Target Go PTY Write → PowerShell
PowerShell Output → Target Go PTY Read → EventsEmit("terminal-output", {tabId, data}) → Target Xterm.js write
```

### Directory Detection
```
Shell prompt appears in PTY output
  → Go detectCwdChange() parses "PS C:\path>" or "C:\path>" pattern
  → EventsEmit("cwd-change", path)
  → Frontend updates breadcrumb + tree (if not pinned)
```

### Persistent Data
```
%APPDATA%/termi/settings.json  — application settings
%APPDATA%/termi/history.txt    — command history (one per line)
```

## Communication

### Go → Frontend Events
| Event | Payload | Purpose |
|---|---|---|
| `terminal-output` | string (raw PTY data) | Pipe shell output to xterm.js |
| `cwd-change` | string (path) | Notify frontend of directory change |
| `command-complete` | float64 (seconds) | Notify when a long command finishes |

### Frontend → Go Function Calls
| Function | Purpose |
|---|---|
| `StartTerminal(initialDir)` | Initialize new PTY with PowerShell, returns generated `tabId` |
| `RestartTerminal(tabId)` | Tear down and restart specific tab's PTY |
| `WriteToTerminal(tabId, input)` | Send keystrokes to target shell |
| `ResizeTerminal(tabId, cols, rows)` | Resize specific PTY dimensions |
| `GetWorkingDir(tabId)` | Synchronously fetch current directory of a tab |
| `ListDirectory(path)` | List files in a directory |
| `CreateDirectory(path)` | Create a new folder |
| `OpenFileInEditor(path)` | Open file with system default app |
| `RenameFile(old, new)` | Rename a file or folder |
| `DeleteFile(path)` | Delete a file |
| `DeleteDirectory(path)` | Delete a directory recursively |
| `LoadCommandHistory()` / `SaveCommandHistory(cmd)` | Read/append history |
| `ClearCommandHistory()` | Truncate history file |
| `GetCompletions(partial, cwd)` | Get autocomplete suggestions |

## State Management

Frontend state is managed through Svelte writable stores (`stores.js`):

| Store | Type | Purpose |
|---|---|---|
| `historyOpen` | boolean | History panel visibility |
| `searchOpen` | boolean | Search bar visibility |
| `commandHistory` | string[] | In-memory command history |

## Theming

Dark theme with CSS variables:
- Deep navy background: `#0f0f1a`, `#1a1a2e`
- Topbar/panels: `#16213e`
- Cyan accent: `#00d4ff`
- Text: `#e0e0e0` (primary), `#888` (muted)
- Borders: `#2a2a4a`

The xterm.js terminal has its own matching theme object with the same color palette.
