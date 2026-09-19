<script>
    import { settingsOpen, settingsStore } from "./stores.js";
    import { SaveSettings } from "../wailsjs/go/main/App.js";
    import { createEventDispatcher } from "svelte";

    const dispatch = createEventDispatcher();

    let fontSize = 14;
    let theme = "dark";
    let shell = "PowerShell";

    // Sync from store on open
    $: if ($settingsOpen) {
        fontSize = $settingsStore.fontSize;
        theme = $settingsStore.theme;
        shell = $settingsStore.shell;
    }

    async function applySettings() {
        const next = { fontSize, theme, shell };
        const shellChanged = shell !== $settingsStore.shell;
        settingsStore.set(next);
        await SaveSettings(next);

        // App owns the terminals, so it applies the change and restarts the
        // shell if it needs to — this panel only reports what was chosen.
        dispatch("apply", { settings: next, shellChanged });
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
                    <label class="setting-label">Font Size</label>
                    <div class="setting-control">
                        <input
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
                    <label class="setting-label">Theme</label>
                    <div class="setting-control">
                        <select
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
                    <label class="setting-label">Shell</label>
                    <div class="setting-control">
                        <select
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

    .shell-warn {
        font-size: 11px;
        color: var(--dot-yellow, #ffbd2e);
        white-space: nowrap;
    }
</style>
