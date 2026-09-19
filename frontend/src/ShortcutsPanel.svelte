<script>
    import { shortcutsOpen } from "./stores.js";
    import { SHORTCUTS, SHORTCUT_GROUPS } from "./shortcuts.js";

    let filter = "";

    $: matching = filter
        ? SHORTCUTS.filter(
              (s) =>
                  s.label.toLowerCase().includes(filter.toLowerCase()) ||
                  s.keys.toLowerCase().includes(filter.toLowerCase()),
          )
        : SHORTCUTS;

    $: groups = SHORTCUT_GROUPS.map((name) => ({
        name,
        items: matching.filter((s) => s.group === name),
    })).filter((g) => g.items.length > 0);

    // "Ctrl+Shift+C" -> ["Ctrl", "Shift", "C"] so each key gets its own <kbd>
    function keyParts(keys) {
        return keys.split("+").map((p) => p.trim());
    }

    function close() {
        shortcutsOpen.set(false);
    }

    function handleKeydown(e) {
        if (e.key === "Escape" && $shortcutsOpen) {
            e.stopPropagation();
            close();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $shortcutsOpen}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="shortcuts-backdrop" on:click={close}></div>

    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="shortcuts-panel" on:click|stopPropagation>
        <div class="shortcuts-header">
            <span class="shortcuts-title">Keyboard Shortcuts</span>
            <button class="shortcuts-close" on:click={close}>✕</button>
        </div>

        <div class="shortcuts-filter">
            <!-- svelte-ignore a11y-autofocus -->
            <input
                bind:value={filter}
                placeholder="Filter shortcuts..."
                spellcheck="false"
                class="shortcuts-filter-input"
                autofocus
            />
        </div>

        <div class="shortcuts-body">
            {#if groups.length === 0}
                <div class="shortcuts-empty">No shortcuts match "{filter}"</div>
            {:else}
                {#each groups as group}
                    <div class="shortcuts-group">{group.name}</div>
                    {#each group.items as s}
                        <div class="shortcuts-row">
                            <span class="shortcuts-label">{s.label}</span>
                            <span class="shortcuts-keys">
                                {#each keyParts(s.keys) as part, i}
                                    {#if i > 0}<span class="shortcuts-plus">+</span>{/if}
                                    <kbd>{part}</kbd>
                                {/each}
                            </span>
                        </div>
                    {/each}
                {/each}
            {/if}
        </div>

        <div class="shortcuts-footer">
            Press <kbd>F1</kbd> any time to reopen this list
        </div>
    </div>
{/if}

<style>
    .shortcuts-backdrop {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.45);
        z-index: 300;
    }

    .shortcuts-panel {
        position: fixed;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        width: 520px;
        max-width: 90vw;
        max-height: 78vh;
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 10px;
        box-shadow: 0 20px 60px rgba(0, 0, 0, 0.6);
        z-index: 301;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .shortcuts-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 12px 14px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
        flex-shrink: 0;
    }

    .shortcuts-title {
        font-size: 12px;
        font-weight: 600;
        color: var(--text-muted, #888);
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .shortcuts-close {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        font-size: 14px;
        cursor: pointer;
        padding: 2px 6px;
        border-radius: 4px;
        transition: background 0.15s;
    }

    .shortcuts-close:hover {
        background: rgba(255, 255, 255, 0.08);
    }

    .shortcuts-filter {
        padding: 8px 12px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
        flex-shrink: 0;
    }

    .shortcuts-filter-input {
        width: 100%;
        background: var(--input-bg, #0f0f1a);
        color: var(--text-primary, #e0e0e0);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 5px;
        padding: 6px 10px;
        font-family: Consolas, monospace;
        font-size: 12px;
        outline: none;
        transition: border-color 0.2s;
    }

    .shortcuts-filter-input:focus {
        border-color: var(--accent, #00d4ff);
    }

    .shortcuts-body {
        flex: 1;
        overflow-y: auto;
        padding: 4px 0 10px;
        scrollbar-width: thin;
        scrollbar-color: var(--border-color, #2a2a4a) transparent;
    }

    .shortcuts-group {
        font-size: 10px;
        font-weight: 600;
        letter-spacing: 0.6px;
        text-transform: uppercase;
        color: var(--accent, #00d4ff);
        padding: 12px 16px 5px;
    }

    .shortcuts-row {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 16px;
        padding: 5px 16px;
    }

    .shortcuts-label {
        font-size: 12.5px;
        color: var(--text-primary, #e0e0e0);
    }

    .shortcuts-keys {
        display: flex;
        align-items: center;
        gap: 3px;
        flex-shrink: 0;
    }

    .shortcuts-plus {
        font-size: 10px;
        color: var(--text-muted, #888);
    }

    kbd {
        font-family: Consolas, monospace;
        font-size: 11px;
        color: var(--text-primary, #e0e0e0);
        background: var(--input-bg, #0f0f1a);
        border: 1px solid var(--border-color, #2a2a4a);
        border-bottom-width: 2px;
        border-radius: 4px;
        padding: 2px 6px;
        white-space: nowrap;
    }

    .shortcuts-empty {
        font-size: 12px;
        color: var(--text-muted, #888);
        padding: 24px;
        text-align: center;
        font-style: italic;
    }

    .shortcuts-footer {
        padding: 9px 16px;
        border-top: 1px solid var(--border-color, #2a2a4a);
        font-size: 11px;
        color: var(--text-muted, #888);
        flex-shrink: 0;
    }
</style>
