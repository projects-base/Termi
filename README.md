<p align="center">
  <img src="assets/banner.svg" alt="Termi" width="700" />
</p>

<p align="center">
  <strong>A modern, feature-rich terminal for Windows</strong> — built with Go, Svelte, and xterm.js
</p>

<p align="center">
  <img src="https://img.shields.io/badge/platform-Windows%2010%2F11-blue?style=flat-square" alt="Platform" />
  <img src="https://img.shields.io/badge/go-1.23-00ADD8?style=flat-square&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/svelte-3.49-FF3E00?style=flat-square&logo=svelte" alt="Svelte" />
  <img src="https://img.shields.io/badge/wails-2.12-412991?style=flat-square" alt="Wails" />
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="License" />
</p>

---

Termi replaces the classic Windows Command Prompt with a sleek, modern interface. It features **multi-tab support**, an integrated file explorer, breadcrumb navigation, command history, terminal search, auto-complete, and more.

## Download & Run

### Option 1: Download the .exe (no install needed)

1. Go to the [Releases](https://github.com/yourusername/termi/releases) page
2. Download `termi.exe` from the latest release
3. Double-click to run — that's it

No installer, no setup, no dependencies. Just a single `.exe` file. Works on Windows 10 and 11.

> **Note:** On first launch, Windows SmartScreen may show a warning since the app isn't code-signed. Click **"More info"** then **"Run anyway"**.

### Option 2: Build from source

Requires Go 1.23+, Node.js 18+, and Wails CLI 2+.

```powershell
# Install Wails CLI (one-time)
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Clone and build
git clone https://github.com/yourusername/termi.git
cd termi
wails build
```

The compiled `termi.exe` is in `build/bin/`. Run it directly.

### Creating a Release (Installers)

If you want to distribute Termi to others with an official `.exe` Windows Installer:

1. **Install NSIS Compiler**: Wails generates installers using NSIS. You must install it first: `scoop install nsis` (or download it manually).
2. **Execute Automated Build Pipeline**: Run the included PowerShell pipeline script.
```powershell
.\build_release.ps1
```
This script will:
- Read your exact version configuration from `wails.json`.
- Spin up the Wails NSIS installer-compiler.
- Generate and auto-rename your binaries dynamically into `build/bin/` (e.g. `Termi-v1.0.0.exe` and `Termi-v1.0.0-setup.exe`).

### Windows SmartScreen ("Safe to Run")
Like all Windows applications downloaded from the web, end-users will be presented with a blue **"Windows protected your PC"** popup natively. To bypass this, Windows **strictly requires** you to physically cryptographically sign the `Termi-v1.0.0-setup.exe` executable using a purchased **Authenticode EV (Extended Validation) Code Signing Certificate** (from authorities like DigiCert, Sectigo, etc.).

Once you purchase a certificate, you can securely sign the installer using the Windows SDK:
```powershell
signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /f mycert.pfx /p password build/bin/Termi-v1.0.0-setup.exe
```

To easily distribute via GitHub:
1. Tag your version: `git tag v1.0.0 && git push --tags`
2. Go to your repo > Releases > Draft a new release
3. Upload `build/bin/Termi-v1.0.0-setup.exe` and publish.

### Development

```powershell
wails dev
```

Hot-reloads both Go backend and Svelte frontend on file changes.

## Usage

### Terminal

Termi runs PowerShell via Windows ConPTY. Everything you can do in a normal terminal works here.

| Action | How |
|---|---|
| Open new tab | Click **+** in the tab bar, or **Ctrl+Shift+T** |
| Close tab | Click **✕** on the tab, or **Ctrl+Shift+W** |
| Switch tabs | Click the tab title, **Ctrl+Tab** / **Ctrl+Shift+Tab**, or **Alt+1**–**Alt+9** |
| Run a command | Type in the bottom command bar, press **Enter** |
| Multi-line input | **Shift+Enter** in the command bar |
| Copy selection | **Ctrl+C** (when text is selected), or Right-click > Copy |
| Auto-Copy | Highlight text in the terminal and it lands on the clipboard once the selection settles (**Copy-on-Select**) |
| Paste | **Ctrl+Shift+V** or right-click > Paste |
| Clear screen | **Ctrl+L** |
| Search output | **Ctrl+F**, then Enter/Shift+Enter to navigate matches |
| Select all | **Ctrl+Shift+A**, or right-click > Select All |
| Change shell (PowerShell / CMD / WSL) | **Ctrl+,** > Shell |
| Undo / redo in the command bar | **Ctrl+Z** / **Ctrl+Y** (or **Ctrl+Shift+Z**) |
| See every shortcut | **F1**, or the keyboard icon in the tab bar |

### File Explorer

The right panel shows the file tree for the current directory.

| Action | How |
|---|---|
| Expand/collapse folder | Single-click the folder |
| Open file in default app | Double-click the file |
| Create new folder | Click the folder+ icon in the nav bar |
| Rename file/folder | Right-click > Rename |
| Delete file/folder | Right-click > Delete |
| Copy path | Right-click > Copy Path |
| Open folder in terminal | Right-click > Open in Terminal |
| Drag file to terminal | Drag from explorer, drop on terminal to paste path |
| Filter files | Type in the filter box above the tree |

### Navigation

The nav bar at the top of the explorer provides directory navigation.

| Button | Action |
|---|---|
| **<** | Go back to previous directory |
| **>** | Go forward |
| **^** | Go up to parent directory |
| **Breadcrumb segments** | Click any segment to jump to that directory |
| **Double-click breadcrumbs** | Copy the full path to clipboard |
| **Pencil icon** | Edit the path manually |
| **Pin icon** | Toggle pin mode (see below) |
| **Clock icon** | Open the recent folders list (**Ctrl+Shift+R**) |

### Pin Mode

Pin mode is **on by default**. When pinned:

- The explorer navigates **independently** -- terminal `cd` commands don't change the explorer
- All explorer navigation (back, forward, up, breadcrumbs, new folder) works normally
- Only the terminal is unaffected by explorer actions

Click the pin icon to **unpin** -- the explorer and terminal will sync: navigating in the explorer also runs `cd` in the terminal, and `cd` in the terminal updates the explorer.

### Command History

Press **Ctrl+K** to open the history panel. It shows all previously run commands (most recent first). Click any command to paste it into the command bar. Use the filter input to search, or click "Clear" to wipe history.

### Recent Folders

Press **Ctrl+Shift+R** (or click the clock icon in the tab bar or the explorer nav)
to see every folder you have worked in, newest first, with how long ago you were
there. Click one to open it in the explorer, or use the **cd** button on the row to
change the active terminal into it. **copy** puts the path on the clipboard and
**✕** drops a single entry; **Clear** empties the list.

Folders are recorded whenever the explorer or the terminal moves into them, and
entries that no longer exist on disk are pruned automatically. The list lives in
`%APPDATA%/termi/recent.json` and holds the 50 most recent folders.

### Keyboard Shortcut Help

Press **F1** (or click the keyboard icon in the tab bar) for a searchable list of
every shortcut Termi understands, grouped by what it affects. The list is generated
from the same table that registers the shortcuts, so it is always accurate.

### Pasting Long Commands

Long or multi-line input is sent to the shell as a single ordered paste rather than
a burst of keystrokes: the backend feeds ConPTY in 512-byte chunks, serializes the
writes per tab, and — when the program in the foreground supports it — wraps the
block in a bracketed-paste sequence.

A single-line paste that ends in a newline still runs straight away. A multi-line
block drops its trailing newline, so the last line waits at the prompt for you to
look it over before pressing Enter.

### Settings

Press **Ctrl+,** or click the gear icon in the tab bar.

| Setting | Effect |
|---|---|
| **Font size** | Applies to every open terminal straight away (10–24px) |
| **Theme** | Dark or light, for both the app chrome and the terminal colours |
| **Shell** | PowerShell, CMD or WSL |

Changing the shell restarts every tab on the new shell. Tabs keep their position
and their working directory — only the underlying session is replaced. Preferences
are stored in `%APPDATA%/termi/settings.json` and reload on next launch.

### Auto-Complete

As you type in the command bar, suggestions appear from:
- Your command history
- Common commands (git, npm, go, docker, etc.)
- Files and folders in the current directory

Use **Arrow keys** to navigate, **Tab** or **Enter** to accept, **Escape** to dismiss.

### Notifications

When a command runs for longer than 10 seconds, a toast appears in the bottom-right
corner once it finishes, showing the elapsed time. Toasts also confirm a
copy-on-select and report input that could not be delivered to the shell.

If the shell exits on its own (you type `exit`), its tab closes with it, and Termi
opens a fresh one if that was the last tab.

## Keyboard Shortcuts

Press **F1** in the app for this list, searchable and always up to date.

| Shortcut | Action |
|---|---|
| **F1** | Show the keyboard shortcut list |
| **Ctrl+F** | Search in terminal output |
| **Ctrl+K** | Toggle command history panel |
| **Ctrl+Shift+R** | Toggle recent folders panel |
| **Ctrl+Shift+E** | Show / hide the file explorer |
| **Ctrl+,** | Open settings (font, theme, shell) |
| **Ctrl+L** | Clear terminal |
| **Ctrl+C** | Interrupt running process (SIGINT) *OR* Copy if text is currently highlighted |
| **Ctrl+Shift+C** | Force copy selected text |
| **Ctrl+Shift+V** | Paste from clipboard |
| **Ctrl+Shift+A** | Select all terminal output |
| **Ctrl+Shift+T** | New tab |
| **Ctrl+Shift+W** | Close the current tab |
| **Ctrl+Tab** / **Ctrl+Shift+Tab** | Next / previous tab |
| **Alt+1** … **Alt+9** | Jump to a tab by position |
| **Ctrl+Z** | Undo an edit in the command bar |
| **Ctrl+Y** / **Ctrl+Shift+Z** | Redo an edit in the command bar |
| **Enter** | Run command / Next search match |
| **Shift+Enter** | New line in command bar / Previous search match |
| **Escape** | Close the open panel |
| **Tab** | Accept auto-complete suggestion |

## Project Structure

```
termi/
├── main.go              # App entry point, Wails window config
├── app.go               # Backend: PTY, file ops, history, completions
├── frontend/src/
│   ├── App.svelte        # Main layout: terminal + explorer
│   ├── TreeNode.svelte   # Recursive file tree component
│   ├── SearchBar.svelte  # Terminal search (Ctrl+F)
│   ├── HistoryPanel.svelte   # Command history (Ctrl+K)
│   ├── RecentPaths.svelte    # Recent folders (Ctrl+Shift+R)
│   ├── ShortcutsPanel.svelte # Keyboard shortcut list (F1)
│   ├── SettingsPanel.svelte  # Font, theme and shell settings (Ctrl+,)
│   ├── ContextMenu.svelte    # Right-click menus
│   ├── Toast.svelte      # Notification toasts
│   ├── Autocomplete.svelte   # Command auto-complete
│   ├── stores.js         # Shared Svelte state
│   ├── shortcuts.js      # Shortcut table + global key handler
│   ├── ptyWriter.js      # Ordered, chunked input queue per tab
│   └── style.css         # Global styles and theme
└── build/bin/            # Compiled output
```

## Tech Stack

- **Backend**: Go 1.23 + [Wails v2](https://wails.io) + [go-pty](https://github.com/aymanbagabas/go-pty) (Windows ConPTY)
- **Frontend**: [Svelte 3](https://svelte.dev) + [xterm.js 5](https://xtermjs.org) + [Vite 3](https://vitejs.dev)
- **Runtime**: WebView2 (Chromium-based, ships with Windows 10/11)

See [DESIGN.md](DESIGN.md) for detailed architecture, data flow, and version matrix.

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Make your changes and test with `wails dev`
4. Commit: `git commit -m "Add my feature"`
5. Push: `git push origin feature/my-feature`
6. Open a Pull Request

## License

MIT License. See [LICENSE](LICENSE) for details.
