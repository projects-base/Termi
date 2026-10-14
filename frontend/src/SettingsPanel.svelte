<script>
    import { settingsOpen, settingsStore, appInfo } from "./stores.js";
    import {
        SaveSettings,
        ChooseStartupDir,
        CheckForUpdate,
        OpenURL,
    } from "../wailsjs/go/main/App.js";
    import { createEventDispatcher } from "svelte";

    const dispatch = createEventDispatcher();

    let fontSize = 14;
    let theme = "dark";
    let shell = "PowerShell";
    let startupDir = "";

    // Sync from store on open
    $: if ($settingsOpen) {
        fontSize = $settingsStore.fontSize;
        theme = $settingsStore.theme;
        shell = $settingsStore.shell;
        startupDir = $settingsStore.startupDir || "";
    }

    async function applySettings() {
        // Every field goes in: SaveSettings writes the whole file, so omitting
        // one would blank it on disk.
        const next = {
            fontSize,
            theme,
            shell,
            startupDir: startupDir.trim(),
        };
        const shellChanged = shell !== $settingsStore.shell;
        settingsStore.set(next);
        await SaveSettings(next);

        // App owns the terminals, so it applies the change and restarts the
        // shell if it needs to — this panel only reports what was chosen.
        dispatch("apply", { settings: next, shellChanged });
    }

    async function browseStartupDir() {
        const chosen = await ChooseStartupDir();
        if (chosen) {
            startupDir = chosen;
            await applySettings();
        }
    }

    async function clearStartupDir() {
        startupDir = "";
        await applySettings();
    }

    // --- Update check ---
    let checking = false;
    let update = null;

    async function checkForUpdate() {
        checking = true;
        update = null;
        try {
            update = await CheckForUpdate();
        } catch (err) {
            update = { error: "Update check failed" };
        } finally {
            checking = false;
        }
    }

    function openRelease() {
        if (update && update.releaseURL) OpenURL(update.releaseURL);
    }

    function close() {
        settingsOpen.set(false);
    }

    function handleBackdropClick(e) {
        if (e.target === e.currentTarget) close();
    }

    function handleKeydown(e) {
        if (e.key === "Escape" && $settingsOpen) {
            e.stopPropagation();
            close();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $settingsOpen}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="settings-overlay" on:click={handleBackdropClick}>
        <div class="settings-modal">
            <div class="settings-header">
                <span class="settings-title">Settings</span>
                <button class="settings-close" on:click={close}>✕</button>
            </div>

            <div class="settings-body">
                <div class="setting-row">
                    <label class="setting-label" for="font-size">Font Size</label>
                    <div class="setting-control">
                        <input
                            id="font-size"
                            type="range"
                            min="10"
                            max="24"
                            bind:value={fontSize}
                            on:change={applySettings}
                            class="setting-slider"
                        />
                        <span class="setting-value">{fontSize}px</span>
                    </div>
                </div>

                <div class="setting-row">
                    <label class="setting-label" for="theme-select">Theme</label>
                    <div class="setting-control">
                        <select
                            id="theme-select"
                            bind:value={theme}
                            on:change={applySettings}
                            class="setting-select"
                        >
                            <option value="dark">Dark</option>
                            <option value="light">Light</option>
                        </select>
                    </div>
                </div>

                <div class="setting-row">
                    <label class="setting-label" for="shell-select">Shell</label>
                    <div class="setting-control">
                        <select
                            id="shell-select"
                            bind:value={shell}
                            on:change={applySettings}
                            class="setting-select"
                        >
                            <option value="PowerShell">PowerShell</option>
                            <option value="CMD">CMD</option>
                            <option value="WSL">WSL</option>
                        </select>
                        {#if shell !== $settingsStore.shell}
                            <span class="shell-warn">Terminal will restart</span>
                        {/if}
                    </div>
                </div>

                <div class="setting-row">
                    <label class="setting-label" for="startup-dir">Startup Folder</label>
                    <div class="setting-control">
                        <input
                            id="startup-dir"
                            class="setting-input"
                            bind:value={startupDir}
                            on:change={applySettings}
                            placeholder="Your home folder"
                            spellcheck="false"
                        />
                        <button class="setting-btn" on:click={browseStartupDir}>Browse</button>
                        {#if startupDir}
                            <button class="setting-btn" on:click={clearStartupDir} title="Use your home folder">Reset</button>
                        {/if}
                    </div>
                    <span class="setting-hint">
                        Where new terminals open. Leave it empty for your home folder —
                        without this, a shortcut launch would start in the install folder.
                    </span>
                </div>
            </div>

            <div class="settings-about">
                <div class="about-line">
                    <span class="about-name">{$appInfo.productName} v{$appInfo.version}</span>
                    <button class="setting-btn" on:click={checkForUpdate} disabled={checking}>
                        {checking ? "Checking…" : "Check for updates"}
                    </button>
                </div>

                {#if update}
                    {#if update.error}
                        <div class="about-status about-muted">{update.error}</div>
                    {:else if update.available}
                        <!-- svelte-ignore a11y-click-events-have-key-events -->
                        <!-- svelte-ignore a11y-no-static-element-interactions -->
                        <div class="about-status about-update">
                            Version {update.latestVersion} is available —
                            <span class="about-link" on:click={openRelease}>open the download page</span>
                        </div>
                    {:else}
                        <div class="about-status about-muted">You are on the latest version.</div>
                    {/if}
                {/if}

                {#if $appInfo.copyright}
                    <div class="about-status about-muted">{$appInfo.copyright}</div>
                {/if}
            </div>
        </div>
    </div>
{/if}

<style>
    .settings-overlay {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.6);
        backdrop-filter: blur(4px);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
    }

    .settings-modal {
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 12px;
        width: 400px;
        max-width: 90vw;
        box-shadow: 0 24px 80px rgba(0, 0, 0, 0.5);
        overflow: hidden;
    }

    .settings-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 16px 20px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
    }

    .settings-title {
        font-size: 15px;
        font-weight: 600;
        color: var(--text-primary, #e0e0e0);
        letter-spacing: 0.3px;
    }

    .settings-close {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        font-size: 16px;
        cursor: pointer;
        padding: 4px 8px;
        border-radius: 4px;
        transition: background 0.15s, color 0.15s;
    }

    .settings-close:hover {
        background: rgba(255, 255, 255, 0.08);
        color: var(--text-primary, #e0e0e0);
    }

    .settings-body {
        padding: 20px;
        display: flex;
        flex-direction: column;
        gap: 20px;
    }

    .setting-row {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .setting-label {
        font-size: 12px;
        font-weight: 500;
        color: var(--text-muted, #888);
        text-transform: uppercase;
        letter-spacing: 0.6px;
    }

    .setting-control {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .setting-slider {
        flex: 1;
        accent-color: var(--accent, #00d4ff);
        height: 4px;
    }

    .setting-value {
        font-family: Consolas, monospace;
        font-size: 13px;
        color: var(--accent, #00d4ff);
        min-width: 40px;
        text-align: right;
    }

    .setting-select {
        flex: 1;
        background: var(--input-bg, #0f0f1a);
        color: var(--text-primary, #e0e0e0);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 6px;
        padding: 8px 12px;
        font-size: 13px;
        font-family: 'Inter', sans-serif;
        outline: none;
        cursor: pointer;
        transition: border-color 0.2s;
    }

    .setting-select:focus {
        border-color: var(--accent, #00d4ff);
    }

    .setting-input {
        flex: 1;
        min-width: 0;
        background: var(--input-bg, #0f0f1a);
        color: var(--text-primary, #e0e0e0);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 6px;
        padding: 8px 12px;
        font-family: Consolas, monospace;
        font-size: 12px;
        outline: none;
        transition: border-color 0.2s;
    }

    .setting-input:focus {
        border-color: var(--accent, #00d4ff);
    }

    .setting-btn {
        background: transparent;
        border: 1px solid var(--border-color, #2a2a4a);
        color: var(--text-muted, #888);
        font-size: 11px;
        padding: 6px 10px;
        border-radius: 5px;
        cursor: pointer;
        white-space: nowrap;
        transition: color 0.15s, border-color 0.15s;
    }

    .setting-btn:hover:not(:disabled) {
        color: var(--accent, #00d4ff);
        border-color: var(--accent, #00d4ff);
    }

    .setting-btn:disabled {
        opacity: 0.5;
        cursor: default;
    }

    .setting-hint {
        font-size: 11px;
        color: var(--text-muted, #888);
        line-height: 1.45;
    }

    .settings-about {
        padding: 14px 20px 18px;
        border-top: 1px solid var(--border-color, #2a2a4a);
        display: flex;
        flex-direction: column;
        gap: 7px;
    }

    .about-line {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
    }

    .about-name {
        font-size: 12.5px;
        font-weight: 600;
        color: var(--text-primary, #e0e0e0);
    }

    .about-status {
        font-size: 11.5px;
    }

    .about-muted {
        color: var(--text-muted, #888);
    }

    .about-update {
        color: var(--dot-green, #27c93f);
    }

    .about-link {
        color: var(--accent, #00d4ff);
        cursor: pointer;
        text-decoration: underline;
    }

    .shell-warn {
        font-size: 11px;
        color: var(--dot-yellow, #ffbd2e);
        white-space: nowrap;
    }
</style>
