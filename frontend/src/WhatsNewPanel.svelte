<script>
    // What changed in a version: this build's own entry on the first launch
    // after an update, or a pending version's fetched from GitHub before you
    // install it.
    import { whatsNewOpen, whatsNew, appInfo } from "./stores.js";
    import { OpenURL } from "../wailsjs/go/main/App.js";

    const RELEASES = "https://github.com/projects-base/Termi/releases";

    function close() {
        whatsNewOpen.set(false);
    }

    function handleBackdropClick(e) {
        if (e.target === e.currentTarget) close();
    }

    function handleKeydown(e) {
        if (e.key === "Escape" && $whatsNewOpen) {
            e.stopPropagation();
            close();
        }
    }

    // The changelog is a small, known subset of Markdown, so it is rendered
    // here rather than by pulling in a parser.
    //
    // Everything is escaped first and the output is assembled from fixed tags.
    // That matters because this text can arrive from the network - a pending
    // version's entry is fetched - and must never be able to inject markup.
    function escapeHTML(s) {
        return s
            .replace(/&/g, "&amp;")
            .replace(/</g, "&lt;")
            .replace(/>/g, "&gt;")
            .replace(/"/g, "&quot;");
    }

    function inline(s) {
        let out = escapeHTML(s);
        // `code` first, so emphasis inside it is left alone.
        out = out.replace(/`([^`]+)`/g, '<code class="wn-code">$1</code>');
        out = out.replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>");
        out = out.replace(/(^|[^*])\*([^*]+)\*/g, "$1<em>$2</em>");
        return out;
    }

    function render(body) {
        if (!body) return "";
        const html = [];
        let list = false;
        let paragraph = [];

        const flushParagraph = () => {
            if (paragraph.length) {
                html.push("<p>" + inline(paragraph.join(" ")) + "</p>");
                paragraph = [];
            }
        };
        const closeList = () => {
            if (list) {
                html.push("</ul>");
                list = false;
            }
        };

        for (const raw of body.split("\n")) {
            const line = raw.trimEnd();

            if (!line.trim()) {
                flushParagraph();
                continue;
            }

            const heading = line.match(/^(#{3,4})\s+(.*)$/);
            if (heading) {
                flushParagraph();
                closeList();
                html.push('<h4 class="wn-h">' + inline(heading[2]) + "</h4>");
                continue;
            }

            const bullet = line.match(/^[-*]\s+(.*)$/);
            if (bullet) {
                flushParagraph();
                if (!list) {
                    html.push('<ul class="wn-list">');
                    list = true;
                }
                html.push("<li>" + inline(bullet[1]) + "</li>");
                continue;
            }

            // A wrapped continuation of the bullet above it.
            if (list && /^\s{2,}\S/.test(raw)) {
                const i = html.length - 1;
                if (html[i] && html[i].startsWith("<li>")) {
                    html[i] = html[i].slice(0, -5) + " " + inline(line.trim()) + "</li>";
                    continue;
                }
            }

            if (line.trim() === "---") {
                flushParagraph();
                closeList();
                html.push('<hr class="wn-rule" />');
                continue;
            }

            closeList();
            paragraph.push(line.trim());
        }

        flushParagraph();
        closeList();
        return html.join("");
    }

    $: entry = $whatsNew;
    $: rendered = entry ? render(entry.body) : "";
</script>

<svelte:window on:keydown={handleKeydown} />

{#if $whatsNewOpen}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="wn-backdrop" on:click={handleBackdropClick}>
        <div class="wn-panel">
            <div class="wn-head">
                <div>
                    <div class="wn-title">What's new</div>
                    <div class="wn-sub">
                        {#if entry && entry.heading}
                            Termi {entry.heading}
                        {:else}
                            Termi {(entry && entry.version) || $appInfo.version}
                        {/if}
                    </div>
                </div>
                <button class="wn-x" on:click={close} title="Close (Esc)">✕</button>
            </div>

            <div class="wn-body">
                {#if !entry}
                    <p class="wn-muted">Looking for the notes…</p>
                {:else if entry.error}
                    <p class="wn-muted">{entry.error}</p>
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <p class="wn-muted">
                        <span class="wn-link" on:click={() => OpenURL(RELEASES)}>
                            Open the releases page
                        </span>
                    </p>
                {:else}
                    {@html rendered}
                {/if}
            </div>

            <div class="wn-foot">
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <span class="wn-link" on:click={() => OpenURL(RELEASES)}>
                    All releases
                </span>
                <button class="wn-done" on:click={close}>Done</button>
            </div>
        </div>
    </div>
{/if}

<style>
    .wn-backdrop {
        position: fixed;
        inset: 0;
        background: rgba(0, 0, 0, 0.55);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
    }

    .wn-panel {
        width: min(620px, 92vw);
        max-height: 78vh;
        display: flex;
        flex-direction: column;
        background: var(--topbar-bg, #16213e);
        border: 1px solid var(--border-color, #2a2a4a);
        border-radius: 10px;
        box-shadow: 0 18px 50px rgba(0, 0, 0, 0.5);
        overflow: hidden;
    }

    .wn-head {
        display: flex;
        align-items: flex-start;
        justify-content: space-between;
        gap: 12px;
        padding: 14px 18px;
        border-bottom: 1px solid var(--border-color, #2a2a4a);
    }

    .wn-title {
        font-size: 14px;
        font-weight: 600;
        color: var(--text-primary, #e0e0e0);
    }

    .wn-sub {
        margin-top: 2px;
        font-size: 11.5px;
        color: var(--accent, #00d4ff);
    }

    .wn-x {
        background: transparent;
        border: none;
        color: var(--text-muted, #888);
        cursor: pointer;
        font-size: 12px;
        padding: 3px 6px;
        border-radius: 4px;
        line-height: 1;
    }

    .wn-x:hover {
        color: var(--text-primary, #e0e0e0);
        background: rgba(255, 255, 255, 0.07);
    }

    .wn-body {
        padding: 6px 18px 14px;
        overflow-y: auto;
        font-size: 12.5px;
        line-height: 1.55;
        color: var(--text-primary, #e0e0e0);
    }

    .wn-body :global(.wn-h) {
        margin: 16px 0 6px;
        font-size: 11px;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.7px;
        color: var(--text-muted, #888);
    }

    .wn-body :global(.wn-list) {
        margin: 0;
        padding-left: 20px;
    }

    .wn-body :global(.wn-list li) {
        margin: 5px 0;
    }

    .wn-body :global(p) {
        margin: 8px 0;
    }

    .wn-body :global(.wn-code) {
        font-family: Consolas, monospace;
        font-size: 11.5px;
        padding: 1px 4px;
        border-radius: 3px;
        background: var(--input-bg, #0f0f1a);
        border: 1px solid var(--border-color, #2a2a4a);
    }

    .wn-body :global(strong) {
        color: #fff;
        font-weight: 600;
    }

    .wn-body :global(.wn-rule) {
        border: none;
        border-top: 1px solid var(--border-color, #2a2a4a);
        margin: 14px 0;
    }

    .wn-muted {
        color: var(--text-muted, #888);
    }

    .wn-foot {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 10px 18px;
        border-top: 1px solid var(--border-color, #2a2a4a);
    }

    .wn-link {
        font-size: 11.5px;
        color: var(--accent, #00d4ff);
        cursor: pointer;
        text-decoration: underline;
    }

    .wn-done {
        background: transparent;
        border: 1px solid var(--accent, #00d4ff);
        color: var(--accent, #00d4ff);
        font-size: 11.5px;
        padding: 4px 14px;
        border-radius: 4px;
        cursor: pointer;
    }

    .wn-done:hover {
        background: var(--accent, #00d4ff);
        color: var(--bg-deep, #0f0f1a);
    }
</style>
