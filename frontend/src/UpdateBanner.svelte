<script>
    // Shown once per launch when GitHub has a newer release than this build.
    //
    // This deliberately is not a toast: a notice that disappears after five
    // seconds is missed by anyone who launched Termi and looked away, and it
    // cannot be clicked. The banner stays until it is acted on, and offers the
    // three things someone actually wants - read about it, stop being told, or
    // leave it for next time.
    import { updateInfo, appInfo } from "./stores.js";
    import { createEventDispatcher } from "svelte";

    const dispatch = createEventDispatcher();

    // The published date is far more useful as "3 days ago" than as a timestamp.
    function relativeDate(iso) {
        if (!iso) return "";
        const then = new Date(iso);
        if (isNaN(then)) return "";
        const days = Math.floor((Date.now() - then) / 86400000);
        if (days <= 0) return "today";
        if (days === 1) return "yesterday";
        if (days < 30) return days + " days ago";
        const months = Math.round(days / 30);
        return months === 1 ? "a month ago" : months + " months ago";
    }

    $: published = relativeDate($updateInfo && $updateInfo.publishedAt);
</script>

{#if $updateInfo}
    <div class="update-banner">
        <span class="update-icon">&#8679;</span>
        <span class="update-text">
            <strong>Termi {$updateInfo.latestVersion}</strong> is available
            <span class="update-dim">
                — you have {$updateInfo.currentVersion || $appInfo.version}{#if published},
                    released {published}{/if}
            </span>
        </span>
        <button class="update-action" on:click={() => dispatch("view")}>
            View release
        </button>
        <button
            class="update-later"
            on:click={() => dispatch("skip")}
            title="Do not tell me about this version again"
        >
            Skip this version
        </button>
        <button
            class="update-x"
            on:click={() => dispatch("dismiss")}
            title="Dismiss until the next launch">&#10005;</button
        >
    </div>
{/if}

<style>
    .update-banner {
        display: flex;
        align-items: center;
        gap: 10px;
        flex-shrink: 0;
        padding: 7px 12px;
        font-size: 12px;
        color: var(--text-primary, #e0e0e0);
        background: var(--topbar-bg, #16213e);
        border-bottom: 1px solid var(--accent, #00d4ff);
    }

    .update-icon {
        color: var(--accent, #00d4ff);
        font-size: 14px;
        line-height: 1;
    }

    .update-text {
        flex: 1;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .update-dim {
        color: var(--text-muted, #888);
    }

    .update-action,
    .update-later {
        flex-shrink: 0;
        cursor: pointer;
        font-size: 11.5px;
        padding: 4px 10px;
        border-radius: 4px;
        border: 1px solid var(--accent, #00d4ff);
        background: transparent;
        color: var(--accent, #00d4ff);
    }

    .update-action:hover {
        background: var(--accent, #00d4ff);
        color: var(--bg-deep, #0f0f1a);
    }

    .update-later {
        border-color: var(--border-color, #2a2a4a);
        color: var(--text-muted, #888);
    }

    .update-later:hover {
        color: var(--text-primary, #e0e0e0);
        border-color: var(--text-muted, #888);
    }

    .update-x {
        flex-shrink: 0;
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        cursor: pointer;
        font-size: 11px;
        padding: 3px 5px;
        border-radius: 4px;
        line-height: 1;
    }

    .update-x:hover {
        color: var(--text-primary, #e0e0e0);
        background: rgba(255, 255, 255, 0.07);
    }
</style>
