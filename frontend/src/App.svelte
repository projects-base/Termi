<script>
import { onMount, tick } from "svelte";
    import { get } from "svelte/store";
    import { Terminal } from "xterm";
    import { FitAddon } from "xterm-addon-fit";
    import { SearchAddon } from "xterm-addon-search";
    import "xterm/css/xterm.css";
    import {
        StartTerminal,
        CloseTerminal,
        RestartTerminal,
        ResizeTerminal,
        ListDirectory,
        GetWorkingDir,
        SaveCommandHistory,
        LoadCommandHistory,
        CreateDirectory,
        SaveRecentPath,
        LoadRecentPaths,
        LoadSettings,
    } from "../wailsjs/go/main/App.js";
    import {
        EventsOn,
        ClipboardGetText,
        ClipboardSetText,
    } from "../wailsjs/runtime/runtime.js";

    import {
        historyOpen,
        searchOpen,
        recentOpen,
        shortcutsOpen,
        settingsOpen,
        settingsStore,
        commandHistory,
        recentPaths,
    } from "./stores.js";
    import { registerShortcuts } from "./shortcuts.js";
    import {
        writeToTab,
        pasteToTab,
        runInTab,
        disposeTabQueue,
        onWriteError,
    } from "./ptyWriter.js";

    import TreeNode from "./TreeNode.svelte";
    import SearchBar from "./SearchBar.svelte";
    import HistoryPanel from "./HistoryPanel.svelte";
    import RecentPaths from "./RecentPaths.svelte";
    import ShortcutsPanel from "./ShortcutsPanel.svelte";
    import SettingsPanel from "./SettingsPanel.svelte";
    import ContextMenu from "./ContextMenu.svelte";
    import Toast from "./Toast.svelte";
    import Autocomplete from "./Autocomplete.svelte";

    // Multi-tab state
    let tabs = [];
    let activeTabId = null;
    let tabCounter = 1;
    let tabKeyCounter = 1;

    // Sessions we are closing or restarting deliberately. Closing a PTY makes the
    // backend emit terminal-closed, and that handler must not treat our own
    // teardown as the shell having exited on its own.
    const intentionalTeardown = new Set();
    let explorerVisible = true;
    let searchAddon; // We might need one per tab, or just use active tab's addon
    let toastRef;

    let commandInput = "";
    let commandTextarea;

    // File explorer state
    let cwd = "";
    let cwdEditPath = "";
    let cwdEditing = false;
    let cwdEntries = [];
    let explorerLoading = false;
    let filterText = "";

    // Navigation history (back/forward)
    let navBack = [];
    let navForward = [];
    let navUserAction = false; // true when navigating via back/forward/breadcrumb (not terminal cd)

    // Pin mode — when pinned, explorer is completely frozen
    let pinned = true;

    // When navigating toward root, this is set to the previous cwd so the tree
    // auto-expands to show where you came from
    let expandToPath = "";

    // Context menu state
    let ctxMenuVisible = false;
    let ctxMenuX = 0;
    let ctxMenuY = 0;
    let ctxMenuItems = [];

    // Drag & drop
    let dragOver = false;

    // Autocomplete
    let autocompleteRef;
    let autocompleteVisible = false;

    // Filtered entries for explorer
    $: filteredEntries = filterText
        ? cwdEntries.filter((e) =>
              e.name.toLowerCase().includes(filterText.toLowerCase()),
          )
        : cwdEntries;

    // Xterm themes, paired with the app palettes in style.css
    const termThemes = {
        dark: {
            background: "#1a1a2e",
            foreground: "#e0e0e0",
            cursor: "#00d4ff",
            selectionBackground: "#264f78",
            black: "#1a1a2e",
            brightBlack: "#444",
            red: "#f07178",
            green: "#c3e88d",
            yellow: "#ffcb6b",
            blue: "#82aaff",
            magenta: "#c792ea",
            cyan: "#89ddff",
            white: "#e0e0e0",
        },
        light: {
            background: "#ffffff",
            foreground: "#1a1a2e",
            cursor: "#0078d4",
            selectionBackground: "#add6ff",
            black: "#1a1a2e",
            brightBlack: "#767676",
            red: "#c72e0f",
            green: "#107c10",
            yellow: "#8e6a00",
            blue: "#0451a5",
            magenta: "#a31515",
            cyan: "#0598bc",
            white: "#f0f0f5",
        },
    };

    function terminalTheme(name) {
        return termThemes[name] || termThemes.dark;
    }

    async function loadExplorer(path) {
        recordRecentPath(path);
        explorerLoading = true;
        try {
            cwdEntries = await ListDirectory(path);
        } catch (e) {
            cwdEntries = [];
        } finally {
            explorerLoading = false;
        }
    }

    // --- RECENT FOLDERS ---
    // The Go side dedupes, caps the list and drops folders that no longer exist,
    // so this is safe to call on every navigation. The guard only suppresses
    // repeats of the folder we just recorded.
    let lastRecordedPath = "";

    function recordRecentPath(path) {
        if (!path || path === lastRecordedPath) return;
        lastRecordedPath = path;
        SaveRecentPath(path);
    }

    // Open a recent folder in the explorer.
    function openRecentPath(e) {
        navigateExplorer(e.detail);
    }

    // cd the active terminal into a recent folder.
    function cdToRecentPath(e) {
        if (!activeTabId) return;
        runInTab(activeTabId, 'cd "' + e.detail + '"', bracketedPaste(activeTabId));
        tabs.find((t) => t.id === activeTabId)?.term.focus();
    }

    // New folder state
    let creatingFolder = false;
    let newFolderName = "";

    function startCreateFolder() {
        creatingFolder = true;
        newFolderName = "";
    }

    async function commitCreateFolder(e) {
        if (e && e.key === "Escape") {
            creatingFolder = false;
            return;
        }
        if (e && e.key !== "Enter") return;
        const name = newFolderName.trim();
        creatingFolder = false;
        if (!name || !cwd) return;
        const folderPath = cwd + "\\" + name;
        await CreateDirectory(folderPath);
        await loadExplorer(cwd);
    }

    function cancelCreateFolder() {
        creatingFolder = false;
    }





    async function createNewTab(focus = true) {
        try {
            const tabId = await StartTerminal("");
            const term = new Terminal({
                cursorBlink: true,
                fontFamily: 'Consolas, "Courier New", monospace',
                fontSize: $settingsStore.fontSize,
                theme: terminalTheme($settingsStore.theme),
            });
            const fitAddon = new FitAddon();
            const sa = new SearchAddon();
            term.loadAddon(fitAddon);
            term.loadAddon(sa);

            const newTab = {
                // `key` identifies the tab in the UI for its whole life. `id` is the
                // backend session, which changes when the shell restarts — keying the
                // markup on it would tear down the div the terminal is attached to.
                key: `tab-${tabKeyCounter++}`,
                id: tabId,
                name: `Terminal ${tabCounter++}`,
                cwd: "",
                term,
                fitAddon,
                searchAddon: sa,
                containerRef: null
            };

            tabs = [...tabs, newTab];

            // These read newTab.id rather than closing over tabId, so a restarted
            // session keeps receiving this terminal's input.
            term.onData((data) => {
                // Send verbatim. xterm has already normalized line endings and, if
                // the foreground program turned on bracketed paste, added the paste
                // markers itself — normalizing or bracketing again here would
                // double-wrap it. The backend still chunks and orders the write, so
                // a large paste arriving as one burst stays intact.
                writeToTab(newTab.id, data);
            });
            term.onResize(({ cols, rows }) => {
                ResizeTerminal(newTab.id, cols, rows);
            });
            
            // Copy-on-select, but only once the selection has settled.
            //
            // onSelectionChange fires on every intermediate selection — every
            // mouse-move of a drag, and programmatic selections too. Copying on
            // each one replaced the clipboard many times per drag, so an
            // accidental drag in the terminal would quietly overwrite whatever
            // the user had copied and was about to paste. Waiting for the
            // selection to stop changing means one copy per deliberate selection.
            let selectionTimer;
            let lastCopiedSelection = "";
            term.onSelectionChange(() => {
                clearTimeout(selectionTimer);
                selectionTimer = setTimeout(() => {
                    const sel = term.getSelection();
                    if (!sel || sel.trim().length === 0) return;
                    if (sel === lastCopiedSelection) return;
                    lastCopiedSelection = sel;
                    ClipboardSetText(sel);
                    if (toastRef) toastRef.showToast("Copied to clipboard!");
                }, 250);
            });
            
            if (focus) {
                await switchTab(tabId);
            }
            
            // The initial cwd-change event from Go may have fired before tabs was updated.
            // Explicitly fetch the working directory here to initialize the explorer correctly.
            const initialCwd = await GetWorkingDir(tabId);
            if (initialCwd) {
                const updatedTab = tabs.find(t => t.id === tabId);
                if (updatedTab) {
                    updatedTab.cwd = initialCwd;
                    if (activeTabId === tabId && explorerVisible) {
                        cwd = initialCwd;
                        await loadExplorer(cwd);
                    }
                }
            }
        } catch(e) {
            console.error("Failed to create terminal tab", e);
        }
    }

    async function switchTab(tabId) {
        activeTabId = tabId;
        const tab = tabs.find(t => t.id === tabId);
        if (tab) {
            searchAddon = tab.searchAddon;
            
            await tick();
            
            if (tab.containerRef && !tab.term.element) {
                tab.term.open(tab.containerRef);
            }
            if (tab.fitAddon) {
                tab.fitAddon.fit();
                ResizeTerminal(tabId, tab.term.cols, tab.term.rows);
            }
            tab.term.focus();
            
            // Update Explorer
            cwd = tab.cwd;
            if (cwd && explorerVisible) loadExplorer(cwd);
        }
    }

    async function closeTab(tabId, e) {
        if(e) e.stopPropagation();
        disposeTabQueue(tabId);
        intentionalTeardown.add(tabId);
        await CloseTerminal(tabId);
        const idx = tabs.findIndex(t => t.id === tabId);
        // The backend also emits terminal-closed, which may have got here first;
        // both paths are guarded and Terminal.dispose() is idempotent.
        tabs[idx]?.term.dispose();
        tabs = tabs.filter(t => t.id !== tabId);
        
        if (activeTabId === tabId) {
            if (tabs.length > 0) {
                switchTab(tabs[Math.max(0, idx - 1)].id);
            } else {
                await createNewTab();
            }
        }
    }

    /**
     * Whether the program in the foreground of a tab has turned on bracketed paste.
     * PSReadLine and most REPL-aware tools set it; cmd.exe and plain console apps
     * do not, and sending them the markers would print them as literal text.
     */
    function bracketedPaste(tabId) {
        const tab = tabs.find((t) => t.id === tabId);
        return Boolean(tab?.term?.modes?.bracketedPasteMode);
    }

    /** Paste the clipboard into the active terminal as one block. */
    async function pasteFromClipboard() {
        if (!activeTabId) return;
        const text = await ClipboardGetText();
        if (text) pasteToTab(activeTabId, text, bracketedPaste(activeTabId));
    }

    async function toggleExplorer() {
        explorerVisible = !explorerVisible;
        if (explorerVisible && cwd) loadExplorer(cwd);
        await tick();
        const tab = tabs.find((t) => t.id === activeTabId);
        if (tab && tab.fitAddon) {
            tab.fitAddon.fit();
            ResizeTerminal(activeTabId, tab.term.cols, tab.term.rows);
        }
    }

    // --- SETTINGS ---
    /** Push the current preferences into the DOM and every open terminal. */
    function applyAppearance(settings) {
        document.documentElement.setAttribute("data-theme", settings.theme);
        const theme = terminalTheme(settings.theme);
        for (const tab of tabs) {
            tab.term.options.fontSize = settings.fontSize;
            tab.term.options.theme = theme;
            if (tab.fitAddon && tab.id === activeTabId) {
                tab.fitAddon.fit();
                ResizeTerminal(tab.id, tab.term.cols, tab.term.rows);
            }
        }
    }

    /**
     * Swap every tab onto a freshly started shell, keeping its position, its
     * xterm instance and its directory. Only the backend session changes.
     */
    async function restartAllShells() {
        for (const tab of tabs) {
            const oldId = tab.id;
            disposeTabQueue(oldId);
            intentionalTeardown.add(oldId);
            try {
                const newId = await RestartTerminal(oldId);
                tab.id = newId;
                tab.term.reset();
                if (activeTabId === oldId) activeTabId = newId;
                ResizeTerminal(newId, tab.term.cols, tab.term.rows);
            } catch (err) {
                console.error("Failed to restart shell for tab", oldId, err);
                if (toastRef) toastRef.showToast("Could not restart the shell");
            }
        }
        tabs = tabs;
        tabs.find((t) => t.id === activeTabId)?.term.focus();
    }

    async function handleSettingsApply(e) {
        const { settings, shellChanged } = e.detail;
        applyAppearance(settings);
        if (shellChanged) {
            if (toastRef) toastRef.showToast(`Restarting with ${settings.shell}…`);
            await restartAllShells();
        }
    }

    // --- TAB NAVIGATION ---
    function switchTabByIndex(index) {
        if (index >= 0 && index < tabs.length) switchTab(tabs[index].id);
    }

    function cycleTab(step) {
        if (tabs.length < 2) return;
        const idx = tabs.findIndex((t) => t.id === activeTabId);
        if (idx === -1) return;
        switchTabByIndex((idx + step + tabs.length) % tabs.length);
    }

    function closeActiveTab() {
        if (activeTabId) closeTab(activeTabId);
    }

    /** Only one overlay panel at a time — opening one dismisses the others. */
    function togglePanel(store) {
        const wasOpen = get(store);
        historyOpen.set(false);
        recentOpen.set(false);
        shortcutsOpen.set(false);
        searchOpen.set(false);
        store.set(!wasOpen);
    }

    onMount(async () => {
        // Preferences first — createNewTab reads font size and theme from the store.
        try {
            const settings = await LoadSettings();
            if (settings) {
                settingsStore.set(settings);
                document.documentElement.setAttribute("data-theme", settings.theme);
            }
        } catch (err) {
            console.error("Failed to load settings", err);
        }

        // Load command history
        const hist = await LoadCommandHistory();
        commandHistory.set(hist || []);

        // Load recent folders
        const recents = await LoadRecentPaths();
        recentPaths.set(recents || []);

        // Surface PTY write failures instead of losing input silently
        onWriteError((msg) => {
            if (toastRef) toastRef.showToast(msg);
        });

        window.addEventListener("resize", () => {
            const tab = tabs.find(t => t.id === activeTabId);
            if (tab && tab.fitAddon) {
                tab.fitAddon.fit();
                ResizeTerminal(activeTabId, tab.term.cols, tab.term.rows);
            }
        });

        // Pipe PTY output from Go → xterm.js
        EventsOn("terminal-output", (payload) => {
            const tab = tabs.find(t => t.id === payload.tabId);
            if (tab) {
                tab.term.write(payload.data);
            }
        });

        // The shell exited on its own (the user typed `exit`) — drop the dead tab
        // rather than leaving it in the bar with a terminal nothing is attached to.
        EventsOn("terminal-closed", (tabId) => {
            if (intentionalTeardown.delete(tabId)) return;
            const tab = tabs.find((t) => t.id === tabId);
            if (!tab) return;
            disposeTabQueue(tabId);
            const idx = tabs.indexOf(tab);
            tab.term.dispose();
            tabs = tabs.filter((t) => t !== tab);
            if (activeTabId === tabId) {
                if (tabs.length > 0) {
                    switchTab(tabs[Math.max(0, idx - 1)].id);
                } else {
                    createNewTab();
                }
            }
        });

        // A command that ran longer than the backend's threshold has finished.
        EventsOn("command-complete", (elapsed) => {
            if (!toastRef) return;
            const seconds = Number(elapsed) || 0;
            const pretty =
                seconds >= 60
                    ? `${Math.floor(seconds / 60)}m ${Math.round(seconds % 60)}s`
                    : `${seconds.toFixed(1)}s`;
            toastRef.showToast(`Command finished in ${pretty}`);
        });

        // Listen for cwd changes detected by Go (terminal cd)
        EventsOn("cwd-change", async (payload) => {
            const tab = tabs.find(t => t.id === payload.tabId);
            if (tab) {
                tab.cwd = payload.data;
                // Track it even when the explorer is hidden or pinned — a folder the
                // terminal cd'd into is exactly what the recent list is for.
                recordRecentPath(payload.data);
                // If it's the active tab, synchronize explorer
                if (tab.id === activeTabId) {
                    if (pinned && cwd) return; // if pinned, UI ignores terminal cd
                    
                    if (payload.data !== cwd) {
                        if (!navUserAction && cwd) {
                            navBack = [...navBack, cwd];
                            navForward = [];
                        }
                        navUserAction = false;
                        cwd = payload.data;
                    }
                    filterText = "";
                    if (explorerVisible) await loadExplorer(cwd);
                }
            }
        });

        // --- KEYBOARD SHORTCUTS ---
        // Every binding here is described in shortcuts.js and shown by F1.
        registerShortcuts({
            clearTerminal: () => {
                if (activeTabId) runInTab(activeTabId, "cls", bracketedPaste(activeTabId));
            },
            copy: () => {
                const sel = tabs.find(t=>t.id===activeTabId)?.term.getSelection();
                if (sel) {
                    ClipboardSetText(sel);
                    if (toastRef) toastRef.showToast("Copied to clipboard!");
                }
            },
            paste: () => pasteFromClipboard(),
            selectAllTerminal: () => {
                tabs.find(t=>t.id===activeTabId)?.term.selectAll();
            },
            newTab: () => createNewTab(),
            closeTab: closeActiveTab,
            nextTab: () => cycleTab(1),
            prevTab: () => cycleTab(-1),
            switchTab: switchTabByIndex,
            toggleExplorer,
            toggleHistory: () => togglePanel(historyOpen),
            toggleRecent: () => togglePanel(recentOpen),
            toggleSearch: () => togglePanel(searchOpen),
            toggleShortcuts: () => togglePanel(shortcutsOpen),
            toggleSettings: () => settingsOpen.update((v) => !v),
        });

        // Start initial Tab
        await createNewTab(true);
    });

    // --- COMMAND BAR ---
    function runCommand() {
        const cmd = commandInput.trim();
        if (!cmd) return;
        // The body goes as a paste and the Enter separately, so a long or
        // multi-line command reaches the shell whole.
        if (activeTabId) runInTab(activeTabId, cmd, bracketedPaste(activeTabId));

        // Save to history
        SaveCommandHistory(cmd);
        commandHistory.update((h) => {
            const filtered = h.filter((c) => c !== cmd);
            return [cmd, ...filtered];
        });

        // Clearing the box is undoable — Ctrl+Z brings the command back.
        pushUndo(currentCommandState(), false);
        commandInput = "";
        autocompleteVisible = false;
        tabs.find((t) => t.id === activeTabId)?.term.focus();
    }

    // --- COMMAND BAR UNDO / REDO ---
    // Svelte rewrites the textarea's value whenever commandInput changes (picking
    // from history, accepting a suggestion, clearing after a run), and that wipes
    // the browser's built-in undo stack. Keep our own so Ctrl+Z always works.
    const UNDO_LIMIT = 100;
    const UNDO_COALESCE_MS = 400;

    let undoStack = [];
    let redoStack = [];
    let pendingUndoState = null;
    let lastUndoPush = 0;

    function currentCommandState() {
        const caret = commandTextarea ? commandTextarea.selectionStart : commandInput.length;
        const end = commandTextarea ? commandTextarea.selectionEnd : commandInput.length;
        return { value: commandInput, start: caret, end };
    }

    /**
     * Record the state from before an edit. `coalesce` folds a run of typing into
     * one undo step; programmatic changes pass false so they get a step of their own.
     */
    function pushUndo(state, coalesce = true) {
        const top = undoStack[undoStack.length - 1];
        if (top && top.value === state.value) return;

        const now = Date.now();
        if (coalesce && top && now - lastUndoPush < UNDO_COALESCE_MS) {
            lastUndoPush = now;
            return;
        }
        lastUndoPush = now;
        undoStack = [...undoStack.slice(-(UNDO_LIMIT - 1)), state];
        redoStack = [];
    }

    async function applyCommandState(state) {
        commandInput = state.value;
        autocompleteVisible = false;
        await tick();
        if (commandTextarea) {
            commandTextarea.focus();
            commandTextarea.setSelectionRange(state.start, state.end);
        }
    }

    async function undoCommandEdit() {
        if (undoStack.length === 0) return;
        const previous = undoStack[undoStack.length - 1];
        undoStack = undoStack.slice(0, -1);
        redoStack = [...redoStack, currentCommandState()];
        lastUndoPush = 0;
        await applyCommandState(previous);
    }

    async function redoCommandEdit() {
        if (redoStack.length === 0) return;
        const next = redoStack[redoStack.length - 1];
        redoStack = redoStack.slice(0, -1);
        undoStack = [...undoStack, currentCommandState()];
        lastUndoPush = 0;
        await applyCommandState(next);
    }

    // beforeinput fires while commandInput still holds the pre-edit value.
    function captureCommandEdit() {
        pendingUndoState = currentCommandState();
    }

    function commitCommandEdit() {
        if (!pendingUndoState) return;
        pushUndo(pendingUndoState, true);
        pendingUndoState = null;
    }

    function handleKeydown(e) {
        const ctrl = e.ctrlKey || e.metaKey;
        const key = (e.key || "").toLowerCase();

        if (ctrl && key === "z" && !e.shiftKey) {
            e.preventDefault();
            undoCommandEdit();
            return;
        }
        if (ctrl && ((key === "z" && e.shiftKey) || (key === "y" && !e.shiftKey))) {
            e.preventDefault();
            redoCommandEdit();
            return;
        }

        // Let autocomplete handle navigation keys next
        if (autocompleteRef && autocompleteRef.handleKeydown(e)) {
            return;
        }
        if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            runCommand();
        }
    }

    function handleAutocompleteSelect(e) {
        pushUndo(currentCommandState(), false);
        commandInput = e.detail;
        autocompleteVisible = false;
        if (commandTextarea) commandTextarea.focus();
    }

    // --- EXPLORER NAVIGATION ---
    // Checks if target is closer to root than current cwd
    function isTowardRoot(target, current) {
        const t = target.replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
        const c = current.replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
        return c.startsWith(t + "/") || c === t || t.length < c.length;
    }

    // Navigate explorer to a new root path. Re-roots the tree.
    // If going toward root, sets expandToPath so tree auto-expands to previous location.
    // When pinned, explorer navigates freely but does NOT send cd to terminal.
    async function navigateExplorer(newPath, { pushHistory = true, focusPath = "" } = {}) {
        if (newPath === cwd) return;

        if (pushHistory && cwd) {
            navBack = [...navBack, cwd];
            navForward = [];
        }

        // If going toward root, auto-expand to show where we came from
        if (focusPath) {
            expandToPath = focusPath;
        } else if (isTowardRoot(newPath, cwd)) {
            expandToPath = cwd;
        } else {
            expandToPath = "";
        }

        cwd = newPath;
        filterText = "";

        // Only cd in terminal when NOT pinned
        if (!pinned) {
            navUserAction = true;
            if (activeTabId) runInTab(activeTabId, 'cd "' + newPath + '"', bracketedPaste(activeTabId));
        }

        await loadExplorer(newPath);

        if (expandToPath) {
            setTimeout(() => { expandToPath = ""; }, 3000);
        }
    }

    function goBack() {
        if (navBack.length === 0) return;
        const prev = navBack[navBack.length - 1];
        navBack = navBack.slice(0, -1);
        navForward = [...navForward, cwd];

        expandToPath = isTowardRoot(prev, cwd) ? cwd : "";
        cwd = prev;
        filterText = "";

        if (!pinned) {
            navUserAction = true;
            if (activeTabId) runInTab(activeTabId, 'cd "' + prev + '"', bracketedPaste(activeTabId));
        }

        loadExplorer(prev);
        if (expandToPath) {
            setTimeout(() => { expandToPath = ""; }, 3000);
        }
    }

    function goForward() {
        if (navForward.length === 0) return;
        const next = navForward[navForward.length - 1];
        navForward = navForward.slice(0, -1);
        navBack = [...navBack, cwd];

        expandToPath = isTowardRoot(next, cwd) ? cwd : "";
        cwd = next;
        filterText = "";

        if (!pinned) {
            navUserAction = true;
            if (activeTabId) runInTab(activeTabId, 'cd "' + next + '"', bracketedPaste(activeTabId));
        }

        loadExplorer(next);
        if (expandToPath) {
            setTimeout(() => { expandToPath = ""; }, 3000);
        }
    }

    function goUp() {
        if (!cwd) return;
        const parts = cwd.replace(/\\/g, "/").split("/").filter(Boolean);
        if (parts.length <= 1) return;
        const parent = parts.slice(0, -1).join("\\");
        const parentPath = parent.length === 2 && parent[1] === ":" ? parent + "\\" : parent;
        navigateExplorer(parentPath);
    }

    // --- BREADCRUMB PATH ---
    function getBreadcrumbs(p) {
        if (!p) return [];
        const parts = p.replace(/\\/g, "/").split("/").filter(Boolean);
        return parts.map((part, i) => {
            const fullPath = parts.slice(0, i + 1).join("\\");
            const path = fullPath.length === 2 && fullPath[1] === ":" ? fullPath + "\\" : fullPath;
            return { label: part, path };
        });
    }

    $: breadcrumbs = getBreadcrumbs(cwd);

    function navigateToBreadcrumb(path) {
        navigateExplorer(path);
    }

    // Double-click breadcrumb bar -> copy full path
    function copyBreadcrumbPath() {
        if (!cwd) return;
        ClipboardSetText(cwd).then(() => {
            copied = true;
            setTimeout(() => (copied = false), 2000);
        });
    }

    // --- CWD PATH EDITING ---
    function startEditPath() {
        cwdEditPath = cwd;
        cwdEditing = true;
    }

    async function commitPathEdit(e) {
        if (e && e.key !== "Enter") return;
        const newPath = cwdEditPath.trim();
        cwdEditing = false;
        if (newPath && newPath !== cwd) {
            navigateExplorer(newPath);
        }
    }

    function cancelPathEdit() {
        cwdEditing = false;
    }

    // --- RESIZER & EXPLORER STATE ---
    let explorerWidth = 240;
    let isResizing = false;
    let copied = false;

    function copyPath(event) {
        event.stopPropagation();
        if (!cwd) return;
        ClipboardSetText(cwd).then(() => {
            copied = true;
            setTimeout(() => (copied = false), 2000);
        });
    }

    function startResize(e) {
        isResizing = true;
        window.addEventListener("mousemove", handleMouseMove);
        window.addEventListener("mouseup", stopResize);
        document.body.style.userSelect = "none";
        document.body.style.cursor = "col-resize";
    }

    function handleMouseMove(e) {
        if (!isResizing) return;
        const minWidth = window.innerWidth * 0.15;
        const maxWidth = window.innerWidth * 0.6;
        let newWidth = window.innerWidth - e.clientX;
        if (newWidth < minWidth) newWidth = minWidth;
        if (newWidth > maxWidth) newWidth = maxWidth;
        explorerWidth = newWidth;
        const tab = tabs.find(t => t.id === activeTabId);
        if (tab && tab.fitAddon) tab.fitAddon.fit();
    }

    function stopResize() {
        isResizing = false;
        window.removeEventListener("mousemove", handleMouseMove);
        window.removeEventListener("mouseup", stopResize);
        document.body.style.userSelect = "auto";
        document.body.style.cursor = "default";
        const tab = tabs.find(t => t.id === activeTabId);
        if (tab && tab.fitAddon) {
            tab.fitAddon.fit();
            ResizeTerminal(activeTabId, tab.term.cols, tab.term.rows);
        }
    }

    // --- TERMINAL CONTEXT MENU ---
    function handleTerminalContext(e) {
        e.preventDefault();
        ctxMenuX = e.clientX;
        ctxMenuY = e.clientY;
        ctxMenuItems = [
            {
                label: "Copy",
                shortcut: "Ctrl+Shift+C",
                action: () => {
                    const sel = tabs.find(t=>t.id===activeTabId)?.term.getSelection();
                    if (sel) ClipboardSetText(sel);
                },
            },
            {
                label: "Paste",
                shortcut: "Ctrl+Shift+V",
                action: pasteFromClipboard,
            },
            { divider: true },
            {
                label: "Select All",
                action: () => tabs.find(t=>t.id===activeTabId)?.term.selectAll(),
            },
            {
                label: "Clear Terminal",
                shortcut: "Ctrl+L",
                action: () => {
                    if (activeTabId) runInTab(activeTabId, "cls", bracketedPaste(activeTabId));
                },
            },
        ];
        ctxMenuVisible = true;
    }

    // --- FILE CONTEXT MENU ---
    function handleFileContext(e) {
        const { x, y, entry } = e.detail;
        ctxMenuX = x;
        ctxMenuY = y;

        if (entry.isDir) {
            ctxMenuItems = [
                {
                    label: "Open in Terminal",
                    action: () => {
                        if (activeTabId) runInTab(activeTabId, 'cd "' + entry.path + '"', bracketedPaste(activeTabId));
                    }
                },
                {
                    label: "Copy Path",
                    action: () => ClipboardSetText(entry.path),
                },
                { divider: true },
                {
                    label: "Rename",
                    action: () => startRename(entry),
                },
                {
                    label: "Delete",
                    action: () => confirmDelete(entry),
                },
            ];
        } else {
            ctxMenuItems = [
                {
                    label: "Open",
                    action: async () => {
                        const { OpenFileInEditor } = await import(
                            "../wailsjs/go/main/App.js"
                        );
                        OpenFileInEditor(entry.path);
                    },
                },
                {
                    label: "Open in Terminal",
                    action: () => {
                        const dir = entry.path
                            .split("\\")
                            .slice(0, -1)
                            .join("\\");
                        if (activeTabId) runInTab(activeTabId, 'cd "' + dir + '"', bracketedPaste(activeTabId));
                    },
                },
                {
                    label: "Copy Path",
                    action: () => ClipboardSetText(entry.path),
                },
                { divider: true },
                {
                    label: "Rename",
                    action: () => startRename(entry),
                },
                {
                    label: "Delete",
                    action: () => confirmDelete(entry),
                },
            ];
        }
        ctxMenuVisible = true;
    }

    // Rename state
    let renamingPath = "";
    let renameNewName = "";

    function startRename(entry) {
        renamingPath = entry.path;
        renameNewName = entry.name;
    }

    async function confirmRename(entry) {
        if (!renameNewName.trim() || renameNewName === entry.name) {
            renamingPath = "";
            return;
        }
        const { RenameFile } = await import("../wailsjs/go/main/App.js");
        const dir = entry.path.split("\\").slice(0, -1).join("\\");
        const newPath = dir + "\\" + renameNewName.trim();
        await RenameFile(entry.path, newPath);
        renamingPath = "";
        await loadExplorer(cwd);
    }

    function cancelRename() {
        renamingPath = "";
    }

    async function confirmDelete(entry) {
        // Simple confirmation via the context — just delete
        if (entry.isDir) {
            const { DeleteDirectory } = await import(
                "../wailsjs/go/main/App.js"
            );
            await DeleteDirectory(entry.path);
        } else {
            const { DeleteFile } = await import(
                "../wailsjs/go/main/App.js"
            );
            await DeleteFile(entry.path);
        }
        await loadExplorer(cwd);
    }

    // --- DRAG & DROP ---
    function handleDragOver(e) {
        e.preventDefault();
        e.dataTransfer.dropEffect = "copy";
        dragOver = true;
    }

    function handleDragLeave() {
        dragOver = false;
    }

    function handleDrop(e) {
        e.preventDefault();
        dragOver = false;
        const path = e.dataTransfer.getData("text/plain");
        if (path) {
            if (activeTabId)
                pasteToTab(activeTabId, '"' + path + '"', bracketedPaste(activeTabId));
            tabs.find(t=>t.id===activeTabId)?.term.focus();
        }
    }

    // --- HISTORY SELECT ---
    function handleHistorySelect(e) {
        pushUndo(currentCommandState(), false);
        commandInput = e.detail;
        historyOpen.set(false);
        if (commandTextarea) commandTextarea.focus();
    }
</script>

<div class="app-container">
    <!-- ===== TERMINAL PANEL ===== -->
    <div
        class="terminal-wrapper"
        class:drag-over={dragOver}
        on:dragover={handleDragOver}
        on:dragleave={handleDragLeave}
        on:drop={handleDrop}
    >
        <!-- Search bar overlay -->
        <SearchBar {searchAddon} />

        <!-- History panel overlay -->
        <HistoryPanel on:select={handleHistorySelect} />

        <!-- Recent folders overlay -->
        <RecentPaths on:open={openRecentPath} on:cd={cdToRecentPath} />

        <!-- TABS BAR -->
        <div class="tabs-bar">
            {#each tabs as t (t.key)}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div class="tab" class:active={t.id === activeTabId} on:click={() => switchTab(t.id)}>
                    <span class="tab-title">{t.name}</span>
                    <button class="tab-close" on:click={(e) => closeTab(t.id, e)}>✕</button>
                </div>
            {/each}
            <button class="tab-new" on:click={() => createNewTab()} title="New tab (Ctrl+Shift+T)">+</button>
            <div class="tab-spacer"></div>

            <!-- Recent folders -->
            <button
                class="sidebar-toggle"
                class:active={$recentOpen}
                on:click={() => togglePanel(recentOpen)}
                title="Recent folders (Ctrl+Shift+R)"
            >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="9"></circle>
                    <polyline points="12 7 12 12 15.5 14"></polyline>
                </svg>
            </button>

            <!-- Keyboard shortcuts -->
            <button
                class="sidebar-toggle"
                class:active={$shortcutsOpen}
                on:click={() => togglePanel(shortcutsOpen)}
                title="Keyboard shortcuts (F1)"
            >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <rect x="2" y="6" width="20" height="13" rx="2" ry="2"></rect>
                    <line x1="6" y1="10" x2="6" y2="10"></line>
                    <line x1="10" y1="10" x2="10" y2="10"></line>
                    <line x1="14" y1="10" x2="14" y2="10"></line>
                    <line x1="18" y1="10" x2="18" y2="10"></line>
                    <line x1="7.5" y1="15" x2="16.5" y2="15"></line>
                </svg>
            </button>

            <!-- Settings -->
            <button
                class="sidebar-toggle"
                class:active={$settingsOpen}
                on:click={() => settingsOpen.update((v) => !v)}
                title="Settings (Ctrl+,)"
            >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="3"></circle>
                    <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.6a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
                </svg>
            </button>

            <!-- Explorer Toggle Button -->
            <button class="sidebar-toggle" class:active={explorerVisible} on:click={toggleExplorer} title="Toggle Explorer (Ctrl+Shift+E)">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
                    <line x1="15" y1="3" x2="15" y2="21"></line>
                </svg>
            </button>
        </div>

        <!-- TERMINAL CONTAINERS -->
        <div class="terminals-container">
            {#each tabs as t (t.key)}
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div 
                    class="terminal-instance" 
                    class:active={t.id === activeTabId} 
                    bind:this={t.containerRef}
                    on:contextmenu={(e) => handleTerminalContext(e)}
                    on:click={() => t.term.focus()}
                ></div>
            {/each}
        </div>

        <div class="command-bar" style="position: relative;">
            <Autocomplete
                bind:this={autocompleteRef}
                partial={commandInput}
                {cwd}
                bind:visible={autocompleteVisible}
                on:select={handleAutocompleteSelect}
            />
            <span class="prompt-label">&#10095;</span>
            <textarea
                id="command-input"
                bind:this={commandTextarea}
                bind:value={commandInput}
                on:keydown={handleKeydown}
                on:beforeinput={captureCommandEdit}
                on:input={commitCommandEdit}
                placeholder="Type a command and press Enter…  (F1 for shortcuts)"
                autocomplete="off"
                spellcheck="false"
            ></textarea>
            <button id="run-btn" on:click={runCommand}>Run</button>
        </div>
    </div>

    <!-- ===== RESIZER ===== -->
    {#if explorerVisible}
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div
        class="resizer"
        class:active={isResizing}
        on:mousedown={startResize}
    ></div>
    {/if}

    <!-- ===== FILE EXPLORER PANEL ===== -->
    {#if explorerVisible}
    <div class="explorer-panel" style="width: {explorerWidth}px;">
        <!-- Navigation bar: new folder / back / forward / up / pin -->
        <div class="explorer-nav">
            <button
                class="nav-btn nav-btn-icon"
                on:click={startCreateFolder}
                title="New Folder"
            >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path d="M1 4V13C1 13.55 1.45 14 2 14H14C14.55 14 15 13.55 15 13V6C15 5.45 14.55 5 14 5H8L6.5 3H2C1.45 3 1 3.45 1 4Z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
                    <path d="M8 8V12M6 10H10" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
                </svg>
            </button>

            <button
                class="nav-btn nav-btn-icon"
                on:click={() => togglePanel(recentOpen)}
                title="Recent folders (Ctrl+Shift+R)"
            >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="1.3"/>
                    <path d="M8 4.5V8L10.2 9.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>

            <button
                class="nav-btn"
                on:click={goBack}
                disabled={navBack.length === 0}
                title="Back"
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M10 3L5 8L10 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>
            <button
                class="nav-btn"
                on:click={goForward}
                disabled={navForward.length === 0}
                title="Forward"
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M6 3L11 8L6 13" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>
            <button
                class="nav-btn"
                on:click={goUp}
                title="Go up one level"
            >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                    <path d="M3 10L8 5L13 10" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>

            <div class="nav-spacer"></div>

            <button
                class="nav-btn nav-btn-icon"
                on:click={startEditPath}
                title="Edit path"
            >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path d="M11.5 1.5L14.5 4.5L5 14H2V11L11.5 1.5Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
            </button>
            <button
                class="nav-btn nav-btn-icon"
                on:click={copyPath}
                title="Copy full path"
            >
                {#if copied}
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                        <path d="M3 8.5L6.5 12L13 4" stroke="var(--dot-green, #27c93f)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                    </svg>
                {:else}
                    <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                        <rect x="5" y="5" width="9" height="9" rx="1.5" stroke="currentColor" stroke-width="1.5"/>
                        <path d="M3 11V3C3 2.45 3.45 2 4 2H12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                    </svg>
                {/if}
            </button>
            <button
                class="nav-btn nav-pin-btn"
                class:pinned
                on:click={() => (pinned = !pinned)}
                title={pinned ? "Unpin — explorer follows terminal" : "Pin — explorer stays independent"}
            >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                    <path d="M9.5 1.5L14.5 6.5L10 11L8 13L3 8L5 6L9.5 1.5Z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" fill={pinned ? "currentColor" : "none"}/>
                    <path d="M1 15L4.5 11.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                </svg>
            </button>
        </div>

        <!-- Breadcrumb path bar -->
        <div class="explorer-cwd" title={cwd}>
            {#if cwdEditing}
                <input
                    class="cwd-edit-input"
                    bind:value={cwdEditPath}
                    on:keydown={commitPathEdit}
                    on:blur={cancelPathEdit}
                    autofocus
                />
            {:else}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div class="breadcrumb-bar" on:dblclick={copyBreadcrumbPath}>
                    {#each breadcrumbs as crumb, i}
                        {#if i > 0}
                            <span class="breadcrumb-sep">&#10095;</span>
                        {/if}
                        <!-- svelte-ignore a11y-click-events-have-key-events -->
                        <!-- svelte-ignore a11y-no-static-element-interactions -->
                        <span
                            class="breadcrumb-item"
                            class:breadcrumb-active={i === breadcrumbs.length - 1}
                            on:click={() => navigateToBreadcrumb(crumb.path)}
                            title={crumb.path}
                        >{crumb.label}</span>
                    {/each}
                    {#if breadcrumbs.length === 0}
                        <span class="breadcrumb-item breadcrumb-active">Loading...</span>
                    {/if}
                </div>
            {/if}
        </div>

        <!-- File filter -->
        <div class="explorer-filter">
            <input
                class="explorer-filter-input"
                bind:value={filterText}
                placeholder="Filter files..."
                spellcheck="false"
            />
        </div>

        <div class="explorer-tree">
            {#if creatingFolder}
                <div class="new-folder-row">
                    <span class="new-folder-icon">&#128193;</span>
                    <!-- svelte-ignore a11y-autofocus -->
                    <input
                        class="new-folder-input"
                        bind:value={newFolderName}
                        on:keydown={commitCreateFolder}
                        on:blur={cancelCreateFolder}
                        placeholder="Folder name..."
                        autofocus
                    />
                </div>
            {/if}
            {#if explorerLoading}
                <div class="explorer-loading">Loading...</div>
            {:else if filteredEntries.length === 0}
                <div class="explorer-empty">
                    {filterText ? "No matches" : "No files found"}
                </div>
            {:else}
                {#each filteredEntries as entry (entry.path)}
                    <TreeNode
                        name={entry.name}
                        path={entry.path}
                        isDir={entry.isDir}
                        depth={0}
                        {renamingPath}
                        {renameNewName}
                        {expandToPath}
                        on:filecontext={handleFileContext}
                        on:rename={(e) => confirmRename(e.detail)}
                        on:cancelrename={cancelRename}
                        on:renameinput={(e) => (renameNewName = e.detail)}
                    />
                {/each}
            {/if}
        </div>
    </div>
    {/if}
</div>

<!-- Global overlays -->
<ContextMenu
    bind:visible={ctxMenuVisible}
    x={ctxMenuX}
    y={ctxMenuY}
    items={ctxMenuItems}
/>
<ShortcutsPanel />
<SettingsPanel on:apply={handleSettingsApply} />
<Toast bind:this={toastRef} />
