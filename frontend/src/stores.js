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

// Files dropped or pasted into the command bar. Each entry is a FilePreview
// from the backend, so the chip can show a thumbnail or the first rows.
export const attachments = writable([]);

// User preferences, mirrored from %APPDATA%/termi/settings.json
export const settingsStore = writable({
    fontSize: 14,
    theme: "dark",
    shell: "PowerShell",
    startupDir: "",
    notifyUpdates: true,
    skippedVersion: "",
});

// A newer release found at launch, or null. Drives the update banner.
export const updateInfo = writable(null);

// This build's name, version and copyright, read from the embedded wails.json
export const appInfo = writable({
    version: "",
    productName: "Termi",
    companyName: "",
    copyright: "",
});
