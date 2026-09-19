import { WriteToTerminal, PasteToTerminal } from "../wailsjs/go/main/App.js";

/**
 * Ordered input queue for the PTY, one queue per tab.
 *
 * Every Wails call runs on its own goroutine, so firing WriteToTerminal twice in
 * quick succession lets the two payloads reach the PTY out of order or interleaved
 * — which is what shreds a long paste into chunks. Here each tab keeps a FIFO and
 * only issues the next call once the previous promise has settled, so the shell
 * sees exactly the bytes the UI produced, in order.
 */

// tabId -> { items: [{ kind, data, bracketed }], running: boolean }
const queues = new Map();

// Safety valve: never let an unbounded amount of text pile up for one tab.
const MAX_PENDING_CHARS = 1 << 20; // 1 MiB

let errorHandler = null;

/** Register a callback invoked when a write to the PTY fails. */
export function onWriteError(fn) {
    errorHandler = fn;
}

function queueFor(tabId) {
    let q = queues.get(tabId);
    if (!q) {
        q = { items: [], running: false };
        queues.set(tabId, q);
    }
    return q;
}

function pendingChars(q) {
    return q.items.reduce((total, item) => total + item.data.length, 0);
}

function enqueue(tabId, kind, data, bracketed = false) {
    if (!tabId || !data) return;
    const q = queueFor(tabId);

    if (pendingChars(q) + data.length > MAX_PENDING_CHARS) {
        if (errorHandler) errorHandler("Input dropped — too much pending text");
        return;
    }

    // Consecutive keystrokes coalesce into a single call; a paste stays its own
    // item because the backend wraps it in a bracketed-paste sequence.
    const tail = q.items[q.items.length - 1];
    if (kind === "write" && tail && tail.kind === "write") {
        tail.data += data;
    } else {
        q.items.push({ kind, data, bracketed });
    }

    if (!q.running) drain(tabId, q);
}

async function drain(tabId, q) {
    q.running = true;
    try {
        while (q.items.length > 0) {
            const item = q.items.shift();
            try {
                if (item.kind === "paste") {
                    await PasteToTerminal(tabId, item.data, item.bracketed);
                } else {
                    await WriteToTerminal(tabId, item.data);
                }
            } catch (err) {
                console.error("PTY write failed", err);
                if (errorHandler) errorHandler("Failed to send input to the terminal");
            }
        }
    } finally {
        q.running = false;
    }
}

/** Send keystroke-sized input (a key press, a control sequence). */
export function writeToTab(tabId, data) {
    enqueue(tabId, "write", data);
}

/** Send bulk text (clipboard, dropped path, multi-line block). */
export function pasteToTab(tabId, text) {
    enqueue(tabId, "paste", text);
}

/**
 * Send a command line: the body as a paste (so long commands survive intact),
 * then the Enter that submits it.
 */
export function runInTab(tabId, command) {
    pasteToTab(tabId, command);
    writeToTab(tabId, "\r");
}

/** Drop anything still queued for a tab that is going away. */
export function disposeTabQueue(tabId) {
    const q = queues.get(tabId);
    if (q) q.items = [];
    queues.delete(tabId);
}
