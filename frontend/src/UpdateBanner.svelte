<script>
    // Shown once per launch when GitHub has a newer release than this build,
    // and the place the update is downloaded and installed from.
    //
    // This deliberately is not a toast: a notice that disappears after five
    // seconds is missed by anyone who launched Termi and looked away, and it
    // cannot be clicked. The banner stays until it is acted on.
    //
    // Installing is a separate click from downloading on purpose. Installing
    // closes Termi, and Termi is holding the user's shells - so the moment that
    // happens has to be theirs to choose, not something that follows a download
    // finishing while they are mid-command.
    import { onMount } from "svelte";
    import { updateInfo, appInfo } from "./stores.js";
    import { createEventDispatcher } from "svelte";
    import { EventsOn } from "../wailsjs/runtime/runtime.js";
    import {
        DownloadUpdate,
        InstallUpdate,
        CanInstallUpdate,
    } from "../wailsjs/go/main/App.js";

    const dispatch = createEventDispatcher();

    // idle | downloading | verifying | ready | failed
    let stage = "idle";
    let percent = 0;
    let received = 0;
    let total = 0;
    let message = "";
    let canInstall = false;
    let installing = false;

    onMount(async () => {
        try {
            canInstall = await CanInstallUpdate();
        } catch (err) {
            canInstall = false;
        }

        EventsOn("update-progress", (p) => {
            if (!p) return;
            stage = p.stage || "idle";
            percent = p.percent || 0;
            received = p.received || 0;
            total = p.total || 0;
            message = p.message || "";
        });
    });

    function startDownload() {
        stage = "downloading";
        percent = 0;
        received = 0;
        total = 0;
        message = "";
        DownloadUpdate();
    }

    async function install() {
        installing = true;
        try {
            await InstallUpdate();
            // Termi closes itself from here; the helper brings it back.
        } catch (err) {
            installing = false;
            stage = "failed";
            message = (err && err.message) || String(err);
        }
    }

    function megabytes(n) {
        if (!n) return "";
        return (n / 1048576).toFixed(1) + " MB";
    }

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

        {#if stage === "idle"}
            <span class="update-text">
                <strong>Termi {$updateInfo.latestVersion}</strong> is available
                <span class="update-dim">
                    — you have {$updateInfo.currentVersion || $appInfo.version}{#if published},
                        released {published}{/if}
                </span>
            </span>
            {#if canInstall}
                <button class="update-action" on:click={startDownload}>
                    Download and install
                </button>
                <button
                    class="update-later"
                    on:click={() => dispatch("notes")}
                    title="See what changed in this version"
                >
                    What's new
                </button>
            {:else}
                <button class="update-action" on:click={() => dispatch("view")}>
                    View release
                </button>
            {/if}
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
        {:else if stage === "downloading"}
            <span class="update-text">
                Downloading Termi {$updateInfo.latestVersion}
                <span class="update-dim">
                    {#if total}— {megabytes(received)} of {megabytes(total)}{/if}
                </span>
            </span>
            <div class="update-bar" title="{percent}%">
                <div class="update-bar-fill" style="width: {percent}%"></div>
            </div>
            <span class="update-pct">{percent}%</span>
        {:else if stage === "verifying"}
            <span class="update-text">
                Checking the download against its published checksum…
            </span>
        {:else if stage === "ready"}
            <span class="update-text">
                <strong>Termi {$updateInfo.latestVersion}</strong> is ready to install
                <span class="update-dim">
                    — Termi will close, ask Windows for permission, and reopen
                </span>
            </span>
            <button class="update-action" on:click={install} disabled={installing}>
                {installing ? "Closing…" : "Install and restart"}
            </button>
            <button
                class="update-x"
                on:click={() => dispatch("dismiss")}
                title="Not now — the download is kept for later">&#10005;</button
            >
        {:else if stage === "failed"}
            <span class="update-text update-bad">
                Update failed <span class="update-dim">— {message}</span>
            </span>
            <button class="update-later" on:click={startDownload}>Try again</button>
            <button class="update-action" on:click={() => dispatch("view")}>
                Download it yourself
            </button>
            <button
                class="update-x"
                on:click={() => dispatch("dismiss")}
                title="Dismiss until the next launch">&#10005;</button
            >
        {/if}
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

    .update-bad {
        color: var(--dot-red, #ff5f56);
    }

    .update-bar {
        flex: 0 0 180px;
        height: 5px;
        border-radius: 3px;
        background: var(--input-bg, #0f0f1a);
        border: 1px solid var(--border-color, #2a2a4a);
        overflow: hidden;
    }

    .update-bar-fill {
        height: 100%;
        background: var(--accent, #00d4ff);
        transition: width 0.2s linear;
    }

    .update-pct {
        flex-shrink: 0;
        font-size: 11px;
        color: var(--text-muted, #888);
        font-variant-numeric: tabular-nums;
        min-width: 34px;
        text-align: right;
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

    .update-action:hover:not(:disabled) {
        background: var(--accent, #00d4ff);
        color: var(--bg-deep, #0f0f1a);
    }

    .update-action:disabled {
        opacity: 0.6;
        cursor: default;
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
