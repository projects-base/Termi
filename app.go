package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aymanbagabas/go-pty"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// TerminalEvent is used to send JSON data to frontend with TabID
type TerminalEvent struct {
	TabID string `json:"tabId"`
	Data  string `json:"data"`
}

// TerminalSession holds data for a single tab
type TerminalSession struct {
	ID               string
	PtyTerm          pty.Pty
	PtyCmd           *pty.Cmd
	Cwd              string
	CommandStartTime time.Time
	CommandPending   bool

	// writeMu serializes PTY writes. Wails runs every bound call in its own
	// goroutine, so without it two inputs can interleave mid-command.
	writeMu sync.Mutex
}

// FileEntry represents a single file or folder in the tree
type FileEntry struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	IsDir    bool        `json:"isDir"`
	Children []FileEntry `json:"children,omitempty"`
}

// Settings holds user-configurable preferences
type Settings struct {
	FontSize int    `json:"fontSize"`
	Theme    string `json:"theme"`
	Shell    string `json:"shell"`
}

// App struct
type App struct {
	ctx      context.Context
	sessions map[string]*TerminalSession
	settings Settings
	mu       sync.Mutex
}

// configDir returns the path to %APPDATA%/termi/, creating it if needed
func configDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, "termi")
	os.MkdirAll(dir, 0755)
	return dir
}

func defaultSettings() Settings {
	return Settings{
		FontSize: 14,
		Theme:    "dark",
		Shell:    "PowerShell",
	}
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		sessions: make(map[string]*TerminalSession),
		settings: defaultSettings(),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.settings = a.LoadSettings()
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------- SETTINGS ----------

// LoadSettings reads settings from disk, returning defaults if missing
func (a *App) LoadSettings() Settings {
	path := filepath.Join(configDir(), "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultSettings()
	}
	s := defaultSettings()
	json.Unmarshal(data, &s)
	return s
}

// SaveSettings writes settings to disk and updates in-memory copy
func (a *App) SaveSettings(s Settings) error {
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir(), "settings.json"), data, 0644)
}

// shellCommand returns the executable and args for the selected shell
func (a *App) shellCommand() (string, []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	exe := "powershell.exe"
	var args []string
	
	switch a.settings.Shell {
	case "CMD":
		exe = "cmd.exe"
	case "WSL":
		exe = "wsl.exe"
	default:
		exe = "powershell.exe"
		args = []string{"-NoLogo", "-ExecutionPolicy", "RemoteSigned"}
	}
	
	if fullPath, err := exec.LookPath(exe); err == nil {
		exe = fullPath
	}
	
	return exe, args
}

// ---------- MULTI-TAB TERMINAL OPS ----------

// StartTerminal initializes a new Windows ConPTY session and returns its tab ID.
// If initialDir is provided, the shell starts there (if supported by the shell).
func (a *App) StartTerminal(initialDir string) (string, error) {
	if initialDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			initialDir, _ = os.UserHomeDir()
		} else {
			initialDir = cwd
		}
	}

	shellExe, shellArgs := a.shellCommand()
	fmt.Printf("Starting %s via ConPTY...\n", shellExe)

	ptyTerm, err := pty.New()
	if err != nil {
		fmt.Println("Error starting pty:", err)
		return "", err
	}

	cmd := ptyTerm.Command(shellExe, shellArgs...)
	cmd.Dir = initialDir

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting command inside pty:", err)
		return "", err
	}

	sessionID := generateID()
	session := &TerminalSession{
		ID:      sessionID,
		PtyTerm: ptyTerm,
		PtyCmd:  cmd,
		Cwd:     initialDir,
	}

	a.mu.Lock()
	a.sessions[sessionID] = session
	a.mu.Unlock()

	// Emit initial CWD immediately
	go func() {
		runtime.EventsEmit(a.ctx, "cwd-change", TerminalEvent{
			TabID: sessionID,
			Data:  initialDir,
		})
	}()

	// Goroutine to continuously read stdout/stderr from the PTY and pipe it to Svelte
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptyTerm.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Println("PTY Read Error [", sessionID, "]:", err)
				break
			}
			chunk := string(buf[:n])
			
			// Broadcast output
			runtime.EventsEmit(a.ctx, "terminal-output", TerminalEvent{
				TabID: sessionID,
				Data:  chunk,
			})
			
			// Detect CWD
			a.detectCwdChange(sessionID, chunk)
		}
		
		// When PTY closes naturally (e.g. user typed 'exit')
		runtime.EventsEmit(a.ctx, "terminal-closed", sessionID)
		a.CloseTerminal(sessionID)
	}()

	return sessionID, nil
}

// CloseTerminal cleanly stops a PTY session
func (a *App) CloseTerminal(tabID string) error {
	a.mu.Lock()
	session, exists := a.sessions[tabID]
	if exists {
		delete(a.sessions, tabID)
	}
	a.mu.Unlock()

	if exists && session.PtyTerm != nil {
		session.PtyTerm.Close()
	}
	return nil
}

// RestartTerminal tears down a tab's PTY and starts a fresh one in the same
// directory, returning the new session ID. Used when the configured shell changes.
func (a *App) RestartTerminal(tabID string) (string, error) {
	dir := a.GetWorkingDir(tabID)
	a.CloseTerminal(tabID)
	return a.StartTerminal(dir)
}

// detectCwdChange parses PTY output to detect PowerShell prompt and extract cwd
func (a *App) detectCwdChange(tabID string, chunk string) {
	a.mu.Lock()
	session, exists := a.sessions[tabID]
	if !exists {
		a.mu.Unlock()
		return
	}
	cwdCache := session.Cwd
	var cmdStartTime time.Time
	cmdPending := session.CommandPending
	a.mu.Unlock()

	lines := strings.Split(chunk, "\n")
	promptDetected := false
	newCwd := cwdCache

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// PowerShell prompt looks like: PS C:\Users\foo>
		if strings.HasPrefix(line, "PS ") && strings.Contains(line, ">") {
			start := 3
			end := strings.LastIndex(line, ">")
			if end > start {
				candidate := strings.TrimSpace(line[start:end])
				if len(candidate) >= 2 && candidate[1] == ':' {
					promptDetected = true
					newCwd = candidate
				}
			}
		}
		// CMD prompt looks like: C:\Users\foo>
		if len(line) >= 3 && line[1] == ':' && line[2] == '\\' && strings.HasSuffix(line, ">") {
			candidate := strings.TrimSuffix(line, ">")
			if len(candidate) >= 2 {
				promptDetected = true
				newCwd = candidate
			}
		}
	}

	a.mu.Lock()
	if promptDetected && session.Cwd != newCwd {
		session.Cwd = newCwd
		runtime.EventsEmit(a.ctx, "cwd-change", TerminalEvent{
			TabID: tabID,
			Data:  newCwd,
		})
	}

	// Toast: if a prompt reappeared and a command was pending, check duration
	if promptDetected && cmdPending {
		cmdStartTime = session.CommandStartTime
		session.CommandPending = false
		elapsed := time.Since(cmdStartTime).Seconds()
		if elapsed > 10 {
			runtime.EventsEmit(a.ctx, "command-complete", elapsed) // Could add tabID here if we want tab-specifc toasts
		}
	}
	a.mu.Unlock()
}

// ---------- PTY INPUT ----------

const (
	// Bulk input is fed to ConPTY in paced slices rather than one burst. A single
	// large write is not guaranteed to be drained before the console's input buffer
	// fills, and the shell only reads it between commands; pacing keeps the pressure
	// low enough that a long paste is not left half-delivered.
	ptyChunkSize  = 512
	ptyChunkPause = 3 * time.Millisecond
)

// session looks up a live session by tab ID.
func (a *App) session(tabID string) *TerminalSession {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.sessions[tabID]
}

// markCommandPending starts the long-command timer when input submits a line.
func (a *App) markCommandPending(session *TerminalSession, input string) {
	if !strings.ContainsAny(input, "\r\n") {
		return
	}
	a.mu.Lock()
	session.CommandStartTime = time.Now()
	session.CommandPending = true
	a.mu.Unlock()
}

// writeToPty feeds data to the PTY in bounded chunks. The per-session lock is held
// for the whole payload, so a concurrent write can never land in the middle of it.
func (a *App) writeToPty(session *TerminalSession, data []byte) error {
	session.writeMu.Lock()
	defer session.writeMu.Unlock()

	for len(data) > 0 {
		end := len(data)
		if end > ptyChunkSize {
			end = ptyChunkSize
		}
		for off := 0; off < end; {
			n, err := session.PtyTerm.Write(data[off:end])
			if err != nil {
				return err
			}
			if n <= 0 {
				return fmt.Errorf("pty write stalled after %d bytes", off)
			}
			off += n
		}
		data = data[end:]
		if len(data) > 0 {
			time.Sleep(ptyChunkPause)
		}
	}
	return nil
}

// WriteToTerminal takes keystroke-sized input from the Svelte UI and pipes it to
// the actual PTY. Clipboard or multi-line text should go through PasteToTerminal.
func (a *App) WriteToTerminal(tabID string, input string) error {
	session := a.session(tabID)
	if session == nil || session.PtyTerm == nil {
		return nil
	}
	a.markCommandPending(session, input)
	return a.writeToPty(session, []byte(input))
}

// PasteToTerminal writes bulk text (clipboard, dropped paths, multi-line blocks) to
// the PTY, normalized to the shell's line ending.
//
// bracketed says whether the program in the foreground has turned on bracketed
// paste (DECSET 2004). When it has, the block is wrapped in the paste markers so
// the line editor takes it as one insertion rather than a stream of keystrokes.
// When it has not — cmd.exe, or a REPL running inside the shell — the markers
// would be echoed as literal text, so they are left off.
func (a *App) PasteToTerminal(tabID string, text string, bracketed bool) error {
	session := a.session(tabID)
	if session == nil || session.PtyTerm == nil {
		return nil
	}

	payload := normalizeInput(text)
	if payload == "" {
		return nil
	}
	a.markCommandPending(session, payload)

	if bracketed {
		payload = "\x1b[200~" + payload + "\x1b[201~"
	}
	return a.writeToPty(session, []byte(payload))
}

// normalizeInput turns arbitrary pasted text into something a console line editor
// accepts: no NULs, and exactly one \r per line break.
//
// A trailing newline is kept on a single-line paste, where running it straight away
// is what people expect, and dropped from a multi-line block, so the last line lands
// on the prompt for review rather than firing the moment it arrives.
func normalizeInput(text string) string {
	text = strings.ReplaceAll(text, "\x00", "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	body := strings.TrimRight(text, "\n")
	if !strings.Contains(body, "\n") && len(body) < len(text) {
		// Single line that came with a newline — keep one, so it runs.
		body += "\n"
	}
	return strings.ReplaceAll(body, "\n", "\r")
}

// ResizeTerminal resizes the PTY to match the xterm.js dimensions
func (a *App) ResizeTerminal(tabID string, cols int, rows int) {
	a.mu.Lock()
	session, exists := a.sessions[tabID]
	a.mu.Unlock()
	
	if exists && session.PtyTerm != nil {
		session.PtyTerm.Resize(cols, rows)
	}
}

// GetWorkingDir returns the current tracked working directory for a tab
func (a *App) GetWorkingDir(tabID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if session, exists := a.sessions[tabID]; exists {
		return session.Cwd
	}
	return ""
}

// ListDirectory returns one level of entries for a given path
func (a *App) ListDirectory(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []FileEntry
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fullPath := filepath.Join(path, e.Name())
		entry := FileEntry{
			Name:  e.Name(),
			Path:  fullPath,
			IsDir: e.IsDir(),
		}
		result = append(result, entry)
	}
	return result, nil
}

// ---------- FILE OPERATIONS ----------

// OpenFileInEditor opens a file with the system default application
func (a *App) OpenFileInEditor(path string) error {
	return exec.Command("cmd", "/c", "start", "", path).Start()
}

// RenameFile renames a file or directory
func (a *App) RenameFile(oldPath string, newPath string) error {
	return os.Rename(oldPath, newPath)
}

// DeleteFile removes a single file
func (a *App) DeleteFile(path string) error {
	return os.Remove(path)
}

// DeleteDirectory removes a directory and all contents
func (a *App) DeleteDirectory(path string) error {
	return os.RemoveAll(path)
}

// CreateDirectory creates a new directory at the given path
func (a *App) CreateDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// ---------- COMMAND HISTORY ----------

func historyPath() string {
	return filepath.Join(configDir(), "history.txt")
}

// SaveCommandHistory appends a command to the history file
func (a *App) SaveCommandHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	f, err := os.OpenFile(historyPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(cmd + "\n")
}

// LoadCommandHistory reads commands from the history file
func (a *App) LoadCommandHistory() []string {
	data, err := os.ReadFile(historyPath())
	if err != nil {
		return []string{}
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	seen := make(map[string]bool)
	var result []string
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" || seen[line] {
			continue
		}
		seen[line] = true
		result = append(result, line)
		if len(result) >= 500 {
			break
		}
	}
	return result
}

// ClearCommandHistory truncates the history file
func (a *App) ClearCommandHistory() {
	os.WriteFile(historyPath(), []byte{}, 0644)
}

// ---------- RECENT PATHS ----------

// RecentPath is a directory the user has worked in, newest first.
type RecentPath struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	LastUsed int64  `json:"lastUsed"`
}

const maxRecentPaths = 50

// recentMu guards the recent-paths file against concurrent read/modify/write.
var recentMu sync.Mutex

func recentPathsFile() string {
	return filepath.Join(configDir(), "recent.json")
}

func readRecentPaths() []RecentPath {
	data, err := os.ReadFile(recentPathsFile())
	if err != nil {
		return []RecentPath{}
	}
	var list []RecentPath
	if json.Unmarshal(data, &list) != nil {
		return []RecentPath{}
	}
	return list
}

func writeRecentPaths(list []RecentPath) {
	if len(list) > maxRecentPaths {
		list = list[:maxRecentPaths]
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(recentPathsFile(), data, 0644)
}

// samePath compares two Windows paths case-insensitively, ignoring separator style
// and any trailing slash (but keeping the one that belongs to a drive root).
func samePath(a, b string) bool {
	norm := func(v string) string {
		v = strings.ReplaceAll(strings.TrimSpace(v), "/", string(os.PathSeparator))
		if len(v) > 3 {
			v = strings.TrimRight(v, string(os.PathSeparator))
		}
		return strings.ToLower(v)
	}
	return norm(a) == norm(b)
}

// SaveRecentPath records a directory visit, moving it to the top of the list.
func (a *App) SaveRecentPath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return
	}

	recentMu.Lock()
	defer recentMu.Unlock()

	kept := []RecentPath{{
		Path:     path,
		Name:     filepath.Base(path),
		LastUsed: time.Now().Unix(),
	}}
	for _, r := range readRecentPaths() {
		if !samePath(r.Path, path) {
			kept = append(kept, r)
		}
	}
	writeRecentPaths(kept)
}

// LoadRecentPaths returns recent directories, newest first, dropping any that have
// been deleted or renamed since they were recorded.
func (a *App) LoadRecentPaths() []RecentPath {
	recentMu.Lock()
	defer recentMu.Unlock()

	stored := readRecentPaths()
	alive := make([]RecentPath, 0, len(stored))
	for _, r := range stored {
		if info, err := os.Stat(r.Path); err == nil && info.IsDir() {
			if r.Name == "" {
				r.Name = filepath.Base(r.Path)
			}
			alive = append(alive, r)
		}
	}
	if len(alive) != len(stored) {
		writeRecentPaths(alive)
	}
	return alive
}

// RemoveRecentPath drops a single entry and returns the remaining list.
func (a *App) RemoveRecentPath(path string) []RecentPath {
	recentMu.Lock()
	kept := []RecentPath{}
	for _, r := range readRecentPaths() {
		if !samePath(r.Path, path) {
			kept = append(kept, r)
		}
	}
	writeRecentPaths(kept)
	recentMu.Unlock()

	return a.LoadRecentPaths()
}

// ClearRecentPaths empties the recent-paths list.
func (a *App) ClearRecentPaths() {
	recentMu.Lock()
	defer recentMu.Unlock()
	writeRecentPaths([]RecentPath{})
}

// ---------- AUTOCOMPLETE ----------

// GetCompletions returns completion suggestions for a partial input
func (a *App) GetCompletions(partial string, cwd string) []string {
	partial = strings.TrimSpace(partial)
	if partial == "" {
		return []string{}
	}
	lower := strings.ToLower(partial)

	seen := make(map[string]bool)
	var results []string

	addIfMatch := func(s string) {
		sl := strings.ToLower(s)
		if strings.HasPrefix(sl, lower) && !seen[sl] {
			seen[sl] = true
			results = append(results, s)
		}
	}

	history := a.LoadCommandHistory()
	for _, h := range history {
		addIfMatch(h)
		if len(results) >= 10 {
			break
		}
	}

	commonCmds := []string{
		"dir", "cd", "cls", "exit", "echo", "type", "mkdir", "rmdir", "del", "copy", "move", "ren",
		"git", "git status", "git add", "git commit", "git push", "git pull", "git log", "git diff",
		"npm", "npm install", "npm run", "npm start", "npm test",
		"go", "go build", "go run", "go test", "go mod tidy",
		"code", "code .", "python", "node", "pip install",
		"ls", "cat", "grep", "curl", "wget", "ssh", "docker", "kubectl",
	}
	for _, c := range commonCmds {
		if len(results) >= 10 {
			break
		}
		addIfMatch(c)
	}

	if len(results) < 10 && cwd != "" {
		entries, err := os.ReadDir(cwd)
		if err == nil {
			for _, e := range entries {
				if len(results) >= 10 {
					break
				}
				addIfMatch(e.Name())
			}
		}
	}

	sort.Strings(results)
	if len(results) > 10 {
		results = results[:10]
	}
	return results
}
