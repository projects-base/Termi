<script>
    import { recentOpen, recentPaths } from "./stores.js";
    import {
        LoadRecentPaths,
        ClearRecentPaths,
        RemoveRecentPath,
    } from "../wailsjs/go/main/App.js";
    import { ClipboardSetText } from "../wailsjs/runtime/runtime.js";
    import { createEventDispatcher } from "svelte";

    const dispatch = createEventDispatcher();

    let filter = "";

    $: filtered = filter
        ? $recentPaths.filter((r) =>
              r.path.toLowerCase().includes(filter.toLowerCase()),
          )
        : $recentPaths;

    // Reload from disk each time the panel opens — entries are pruned there, so a
    // folder deleted outside Termi disappears from the list.
    $: if ($recentOpen) {
        reload();
        filter = "";
    }

    async function reload() {
        const list = await LoadRecentPaths();
        recentPaths.set(list || []);
    }

    /** Parent folder, shown under the folder name for context. */
    function parentOf(path) {
        const idx = path.replace(/\\$/, "").lastIndexOf("\\");
        return idx > 0 ? path.slice(0, idx) : path;
    }

    function relativeTime(unixSeconds) {
        if (!unixSeconds) return "";
        const mins = Math.floor((Date.now() / 1000 - unixSeconds) / 60);
        if (mins < 1) return "just now";
        if (mins < 60) return `${mins}m ago`;
        const hours = Math.floor(mins / 60);
        if (hours < 24) return `${hours}h ago`;
        const days = Math.floor(hours / 24);
        return days < 30 ? `${days}d ago` : `${Math.floor(days / 30)}mo ago`;
    }

    function open(entry) {
        dispatch("open", entry.path);
        recentOpen.set(false);
    }

    function cdTo(entry, e) {
        e.stopPropagation();
        dispatch("cd", entry.path);
        recentOpen.set(false);
    }

    function copy(entry, e) {
        e.stopPropagation();
        ClipboardSetText(entry.path);
    }

    async function remove(entry, e) {
        e.stopPropagation();
        const list = await RemoveRecentPath(entry.path);
        recentPaths.set(list || []);
    }

    async function clearAll() {
        await ClearRecentPaths();
        recentPaths.set([]);
    }

    function close() {
        recentOpen.set(false);
    }

    function handleKeydown(e) {
        if (e.key === "Escape" && $recentOpen) {
            e.stopPropagation();
            close();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $recentOpen}
    <div class="recent-panel">
        <div class="recent-header">
            <span class="recent-title">Recent Folders</span>
            <div class="recent-actions">
                <button class="recent-clear" on:click={clearAll} title="Clear the list">Clear</button>
                <button class="recent-close" on:click={close}>✕</button>
            </div>
        </div>

        <div class="recent-filter">
            <!-- svelte-ignore a11y-autofocus -->
            <input
                bind:value={filter}
                placeholder="Filter paths..."
                spellcheck="false"
                class="recent-filter-input"
                autofocus
            />
        </div>

        <div class="recent-list">
            {#if filtered.length === 0}
                <div class="recent-empty">
                    {filter ? "No matching folders" : "No folders visited yet"}
                </div>
            {:else}
                {#each filtered as entry (entry.path)}
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div class="recent-item" on:click={() => open(entry)} title={entry.path}>
                        <div class="recent-info">
                            <div class="recent-name">
                                <span class="recent-icon">&#128193;</span>
                                <span class="recent-name-text">{entry.name}</span>
                                <span class="recent-time">{relativeTime(entry.lastUsed)}</span>
                            </div>
                            <div class="recent-parent">{parentOf(entry.path)}</div>
                        </div>
                        <div class="recent-item-actions">
                            <button
                                class="recent-action"
                                on:click={(e) => cdTo(entry, e)}
                                title="cd here in the active terminal"
                            >cd</button>
                            <button
                                class="recent-action"
                                on:click={(e) => copy(entry, e)}
                                title="Copy path"
                            >copy</button>
                            <button
                                class="recent-action recent-action-danger"
                                on:click={(e) => remove(entry, e)}
                                title="Remove from list"
                            >✕</button>
                        </div>
                    </div>
                {/each}
            {/if}
        </div>

        <div class="recent-footer">Click a folder to open it in the explorer</div>
    </div>
{/if}

<style>
    .recent-panel {
        position: absolute;
        top: 48px;
        left: 12px;
        width: 420px;
        max-height: 60vh;
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 10px;
        box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
        z-index: 100;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .recent-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 12px 14px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
        flex-shrink: 0;
    }

    .recent-title {
        font-size: 12px;
        font-weight: 600;
        color: var(--text-muted, #888);
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .recent-actions {
        display: flex;
        gap: 6px;
        align-items: center;
    }

    .recent-clear {
        background: transparent;
        border: 1px solid var(--border-color, #2a2a4a);
        color: var(--text-muted, #888);
        font-size: 11px;
        padding: 3px 8px;
        border-radius: 4px;
        cursor: pointer;
        transition: background 0.15s, color 0.15s;
    }

    .recent-clear:hover {
        background: rgba(240, 113, 120, 0.15);
        color: #f07178;
        border-color: #f07178;
    }

    .recent-close {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        font-size: 14px;
        cursor: pointer;
        padding: 2px 6px;
        border-radius: 4px;
        transition: background 0.15s;
    }

    .recent-close:hover {
        background: rgba(255, 255, 255, 0.08);
    }

    .recent-filter {
        padding: 8px 12px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
        flex-shrink: 0;
    }

    .recent-filter-input {
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

    .recent-filter-input:focus {
        border-color: var(--accent, #00d4ff);
    }

    .recent-list {
        flex: 1;
        overflow-y: auto;
        padding: 4px 0;
        scrollbar-width: thin;
        scrollbar-color: var(--border-color, #2a2a4a) transparent;
    }

    .recent-item {
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 7px 14px;
        cursor: pointer;
        transition: background 0.12s;
    }

    .recent-item:hover {
        background: rgba(0, 212, 255, 0.08);
    }

    .recent-info {
        flex: 1;
        min-width: 0;
    }

    .recent-name {
        display: flex;
        align-items: center;
        gap: 7px;
    }

    .recent-icon {
        font-size: 12px;
    }

    .recent-name-text {
        font-family: Consolas, monospace;
        font-size: 12.5px;
        color: var(--text-primary, #e0e0e0);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .recent-time {
        font-size: 10px;
        color: var(--text-muted, #888);
        flex-shrink: 0;
    }

    .recent-parent {
        font-family: Consolas, monospace;
        font-size: 10.5px;
        color: var(--text-muted, #888);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        margin-top: 1px;
        padding-left: 19px;
    }

    .recent-item-actions {
        display: flex;
        gap: 4px;
        opacity: 0;
        transition: opacity 0.12s;
        flex-shrink: 0;
    }

    .recent-item:hover .recent-item-actions {
        opacity: 1;
    }

    .recent-action {
        background: transparent;
        border: 1px solid var(--border-color, #2a2a4a);
        color: var(--text-muted, #888);
        font-family: Consolas, monospace;
        font-size: 10px;
        padding: 2px 6px;
        border-radius: 4px;
        cursor: pointer;
        transition: color 0.15s, border-color 0.15s;
    }

    .recent-action:hover {
        color: var(--accent, #00d4ff);
        border-color: var(--accent, #00d4ff);
    }

    .recent-action-danger:hover {
        color: #f07178;
        border-color: #f07178;
    }

    .recent-empty {
        font-size: 12px;
        color: var(--text-muted, #888);
        padding: 20px;
        text-align: center;
        font-style: italic;
    }

    .recent-footer {
        padding: 8px 14px;
        border-top: 1px solid var(--border-color, #2a2a4a);
        font-size: 10.5px;
        color: var(--text-muted, #888);
        flex-shrink: 0;
    }
</style>
