/**
 * Every keyboard shortcut Termi knows about, in one place.
 *
 * This list is both the binding table used by registerShortcuts() and the source
 * for the Shortcuts panel (F1), so a shortcut can never exist without being
 * documented. Entries marked `manual` are handled by the component that owns the
 * focused element (the command bar, mostly) and are listed here for discovery only.
 */
export const SHORTCUTS = [
    // --- General ---
    { id: "toggleShortcuts", group: "General", keys: "F1", label: "Show this shortcut list" },
    { id: "toggleHistory", group: "General", keys: "Ctrl+K", label: "Command history" },
    { id: "toggleRecent", group: "General", keys: "Ctrl+Shift+R", label: "Recent folders" },
    { id: "toggleSearch", group: "General", keys: "Ctrl+F", label: "Search terminal output" },
    { id: "toggleExplorer", group: "General", keys: "Ctrl+Shift+E", label: "Show / hide file explorer" },
    { id: "toggleSettings", group: "General", keys: "Ctrl+,", label: "Settings (font, theme, shell)" },
    { id: "closePanel", group: "General", keys: "Esc", label: "Close the open panel", manual: true },

    // --- Terminal ---
    { id: "clearTerminal", group: "Terminal", keys: "Ctrl+L", label: "Clear terminal" },
    { id: "copy", group: "Terminal", keys: "Ctrl+Shift+C", label: "Copy selection" },
    { id: "paste", group: "Terminal", keys: "Ctrl+Shift+V", label: "Paste" },
    { id: "selectAllTerminal", group: "Terminal", keys: "Ctrl+Shift+A", label: "Select all output" },

    // --- Tabs ---
    { id: "newTab", group: "Tabs", keys: "Ctrl+Shift+T", label: "New tab" },
    { id: "closeTab", group: "Tabs", keys: "Ctrl+Shift+W", label: "Close current tab" },
    { id: "nextTab", group: "Tabs", keys: "Ctrl+Tab", label: "Next tab" },
    { id: "prevTab", group: "Tabs", keys: "Ctrl+Shift+Tab", label: "Previous tab" },
    { id: "switchTab", group: "Tabs", keys: "Alt+1 … Alt+9", label: "Jump to tab 1–9", manual: true },

    // --- Command bar ---
    { id: "runCommand", group: "Command bar", keys: "Enter", label: "Run the command", manual: true },
    { id: "newline", group: "Command bar", keys: "Shift+Enter", label: "New line without running", manual: true },
    { id: "undo", group: "Command bar", keys: "Ctrl+Z", label: "Undo edit", manual: true },
    { id: "redo", group: "Command bar", keys: "Ctrl+Y", label: "Redo edit", manual: true },
    { id: "redoAlt", group: "Command bar", keys: "Ctrl+Shift+Z", label: "Redo edit (alternate)", manual: true },
    { id: "accept", group: "Command bar", keys: "Tab", label: "Accept the highlighted suggestion", manual: true },
    { id: "suggestNav", group: "Command bar", keys: "↑ / ↓", label: "Move through suggestions", manual: true },
];

/** The order groups appear in the Shortcuts panel. */
export const SHORTCUT_GROUPS = ["General", "Terminal", "Tabs", "Command bar"];

/** Split a "Ctrl+Shift+C" style string into modifier flags and a key name. */
function parseCombo(keys) {
    const parts = keys.split("+").map((p) => p.trim());
    return {
        ctrl: parts.some((p) => /^ctrl$/i.test(p)),
        shift: parts.some((p) => /^shift$/i.test(p)),
        alt: parts.some((p) => /^alt$/i.test(p)),
        key: parts[parts.length - 1].toLowerCase(),
    };
}

function matches(e, combo) {
    if ((e.ctrlKey || e.metaKey) !== combo.ctrl) return false;
    if (e.shiftKey !== combo.shift) return false;
    if (e.altKey !== combo.alt) return false;
    return (e.key || "").toLowerCase() === combo.key;
}

/**
 * Bind every non-manual shortcut that has a handler.
 * Uses the capture phase so shortcuts fire before xterm.js consumes the key.
 *
 * @param {Object} handlers keyed by shortcut id; `switchTab(index)` also gets Alt+1–9.
 * @returns {Function} call to unbind.
 */
export function registerShortcuts(handlers) {
    const bindings = SHORTCUTS.filter((s) => !s.manual && handlers[s.id]).map((s) => ({
        run: handlers[s.id],
        combo: parseCombo(s.keys),
    }));

    const onKeyDown = (e) => {
        // Alt+1 … Alt+9 — jump straight to a tab
        if (handlers.switchTab && e.altKey && !e.ctrlKey && !e.shiftKey && /^[1-9]$/.test(e.key)) {
            e.preventDefault();
            handlers.switchTab(Number(e.key) - 1);
            return;
        }

        for (const b of bindings) {
            if (matches(e, b.combo)) {
                e.preventDefault();
                e.stopPropagation();
                b.run();
                return;
            }
        }
    };

    window.addEventListener("keydown", onKeyDown, true);
    return () => window.removeEventListener("keydown", onKeyDown, true);
}
