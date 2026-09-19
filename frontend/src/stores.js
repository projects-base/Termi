import { writable } from "svelte/store";

// Panel visibility toggles
export const historyOpen = writable(false);
export const searchOpen = writable(false);
export const recentOpen = writable(false);
export const shortcutsOpen = writable(false);
export const settingsOpen = writable(false);

// Command history
export const commandHistory = writable([]);

// Recently visited directories, newest first
export const recentPaths = writable([]);

// User preferences, mirrored from %APPDATA%/termi/settings.json
export const settingsStore = writable({
    fontSize: 14,
    theme: "dark",
    shell: "PowerShell",
});
