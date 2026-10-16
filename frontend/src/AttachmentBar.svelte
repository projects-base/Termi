<script>
    // Chips for files dropped or pasted into the command bar, with a preview so
    // you can confirm you picked the right file before running anything.
    import { attachments } from "./stores.js";
    import { createEventDispatcher } from "svelte";

    const dispatch = createEventDispatcher();

    // Which chip is expanded. Only one at a time: the previews are tall, and
    // the command bar should not push the terminal off the screen.
    let expanded = null;

    function toggle(path) {
        expanded = expanded === path ? null : path;
    }

    function remove(path, e) {
        e.stopPropagation();
        if (expanded === path) expanded = null;
        dispatch("remove", path);
    }

    function icon(a) {
        if (a.isDir) return "\u{1F4C1}";
        if (a.kind === "image") return "\u{1F5BC}";
        if (a.kind === "table") return "\u{1F4CA}";
        if (a.kind === "text") return "\u{1F4C4}";
        return "\u{1F4CE}";
    }

    $: current = $attachments.find((a) => a.path === expanded) || null;
</script>

{#if $attachments.length}
    <div class="attach-bar">
        <div class="chips">
            {#each $attachments as a (a.path)}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div
                    class="chip"
                    class:open={expanded === a.path}
                    class:bad={!!a.error}
                    on:click={() => toggle(a.path)}
                    title={a.path}
                >
                    <span class="chip-icon">{icon(a)}</span>
                    <span class="chip-name">{a.name}</span>
                    {#if a.sizeText && !a.isDir}
                        <span class="chip-size">{a.sizeText}</span>
                    {/if}
                    <button
                        class="chip-x"
                        on:click={(e) => remove(a.path, e)}
                        title="Remove this attachment and its path">✕</button
                    >
                </div>
            {/each}
            <button class="chips-clear" on:click={() => dispatch("clear")}>
                Clear all
            </button>
        </div>

        {#if current}
            <div class="preview">
                <div class="preview-head">
                    <span class="preview-path">{current.path}</span>
                    <span class="preview-meta">
                        {#if current.sheet}sheet “{current.sheet}” ·{/if}
                        {#if current.modified}{current.modified}{/if}
                        {#if current.note} · {current.note}{/if}
                    </span>
                </div>

                {#if current.error}
                    <div class="preview-error">{current.error}</div>
                {:else if current.kind === "image"}
                    {#if current.dataURI}
                        <img class="preview-img" src={current.dataURI} alt={current.name} />
                    {:else}
                        <div class="preview-muted">
                            {current.note || "No thumbnail available"}
                        </div>
                    {/if}
                {:else if current.kind === "table"}
                    {#if current.columns && current.columns.length}
                        <div class="preview-scroll">
                            <table class="preview-table">
                                <thead>
                                    <tr>
                                        {#each current.columns as c}
                                            <th>{c}</th>
                                        {/each}
                                    </tr>
                                </thead>
                                <tbody>
                                    {#each current.rows || [] as row}
                                        <tr>
                                            {#each row as cell}
                                                <td>{cell}</td>
                                            {/each}
                                        </tr>
                                    {/each}
                                </tbody>
                            </table>
                        </div>
                    {:else}
                        <div class="preview-muted">
                            {current.note || "Nothing to show"}
                        </div>
                    {/if}
                {:else if current.kind === "text"}
                    <pre class="preview-text">{current.text}</pre>
                {:else if current.isDir}
                    <div class="preview-muted">
                        Folder{current.note ? " — " + current.note : ""}
                    </div>
                {:else}
                    <div class="preview-muted">
                        {current.note ||
                            "No preview for this file type — the name, size and date are above."}
                    </div>
                {/if}
            </div>
        {/if}
    </div>
{/if}

<style>
    .attach-bar {
        background: var(--topbar-bg, #16213e);
        border-top: 1px solid var(--border-color, #2a2a4a);
        padding: 8px 14px 0;
        flex-shrink: 0;
    }

    .chips {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 6px;
    }

    .chip {
        display: inline-flex;
        align-items: center;
        gap: 6px;
        max-width: 280px;
        padding: 4px 6px 4px 8px;
        background: var(--input-bg, #0f0f1a);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 14px;
        cursor: pointer;
        font-size: 11.5px;
        color: var(--text-primary, #e0e0e0);
        transition: border-color 0.15s;
    }

    .chip:hover,
    .chip.open {
        border-color: var(--accent, #00d4ff);
    }

    .chip.bad {
        border-color: var(--dot-red, #ff5f56);
    }

    .chip-icon {
        font-size: 12px;
        line-height: 1;
    }

    .chip-name {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .chip-size {
        color: var(--text-muted, #888);
        font-size: 10.5px;
        flex-shrink: 0;
    }

    .chip-x {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        cursor: pointer;
        font-size: 10px;
        padding: 2px 4px;
        border-radius: 8px;
        line-height: 1;
    }

    .chip-x:hover {
        color: var(--dot-red, #ff5f56);
        background: rgba(255, 255, 255, 0.07);
    }

    .chips-clear {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        font-size: 10.5px;
        cursor: pointer;
        text-decoration: underline;
        padding: 4px 2px;
    }

    .chips-clear:hover {
        color: var(--accent, #00d4ff);
    }

    .preview {
        margin-top: 8px;
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 8px;
        background: var(--input-bg, #0f0f1a);
        overflow: hidden;
    }

    .preview-head {
        display: flex;
        justify-content: space-between;
        align-items: baseline;
        gap: 12px;
        padding: 7px 10px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
    }

    .preview-path {
        font-family: Consolas, monospace;
        font-size: 11px;
        color: var(--text-primary, #e0e0e0);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .preview-meta {
        font-size: 10.5px;
        color: var(--text-muted, #888);
        white-space: nowrap;
        flex-shrink: 0;
    }

    .preview-scroll {
        max-height: 190px;
        overflow: auto;
    }

    .preview-table {
        border-collapse: collapse;
        font-size: 11px;
        font-family: Consolas, monospace;
        width: max-content;
        min-width: 100%;
    }

    .preview-table th,
    .preview-table td {
        border-right: 1px solid var(--border-color, #2a2a4a);
        border-bottom: 1px solid var(--border-color, #2a2a4a);
        padding: 4px 8px;
        text-align: left;
        max-width: 220px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .preview-table th {
        position: sticky;
        top: 0;
        background: var(--topbar-bg, #16213e);
        color: var(--accent, #00d4ff);
        font-weight: 600;
    }

    .preview-table td {
        color: var(--text-primary, #e0e0e0);
    }

    .preview-img {
        display: block;
        max-width: 100%;
        max-height: 190px;
        margin: 8px auto;
        object-fit: contain;
    }

    .preview-text {
        margin: 0;
        padding: 8px 10px;
        max-height: 190px;
        overflow: auto;
        font-family: Consolas, monospace;
        font-size: 11px;
        line-height: 1.45;
        color: var(--text-primary, #e0e0e0);
        white-space: pre;
    }

    .preview-muted,
    .preview-error {
        padding: 10px;
        font-size: 11.5px;
        color: var(--text-muted, #888);
    }

    .preview-error {
        color: var(--dot-red, #ff5f56);
    }
</style>
