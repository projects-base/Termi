package main

// Downloading and installing an update from inside the app, rather than
// sending the user to a web page to fetch the installer by hand.
//
// Three things make this more than "download and run":
//
//  1. We are about to execute a binary pulled off the network, so the download
//     is pinned to GitHub hosts and checked against the SHA256SUMS.txt that
//     ships with the release. That is an integrity check, not a signature -
//     see the note on verifyChecksum.
//  2. The installer asks Windows for elevation, so a UAC prompt appears. That
//     is the installer's manifest, not something this code can or should work
//     around.
//  3. The installer replaces termi.exe, which Windows will not let it do while
//     this process is running it. So the install is carried out by a small
//     detached helper: it waits for Termi to exit, runs the installer, then
//     starts the new build.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows/registry"
)

// Only these hosts are ever downloaded from. The release response is data
// fetched over the network: without this check, a tampered or redirected reply
// could point the download at any host at all and we would run whatever came
// back. GitHub serves release assets by redirecting to its object storage, so
// those hosts have to be allowed too.
var allowedDownloadHosts = map[string]bool{
	"github.com":                           true,
	"api.github.com":                       true,
	"objects.githubusercontent.com":        true,
	"release-assets.githubusercontent.com": true,
}

// Stages reported to the UI through the update-progress event.
const (
	stageDownloading = "downloading"
	stageVerifying   = "verifying"
	stageReady       = "ready"
	stageFailed      = "failed"
)

// UpdateProgress is one update-progress event.
type UpdateProgress struct {
	Stage    string `json:"stage"`
	Percent  int    `json:"percent"`
	Received int64  `json:"received"`
	Total    int64  `json:"total"`
	Message  string `json:"message"`
}

// downloading guards against a second download being started while one is
// already in flight - the banner disables its button, but an event arriving
// twice should not be able to run two of these.
var downloading atomic.Bool

// pendingInstaller is the verified installer waiting to be run, set by
// DownloadUpdate and consumed by InstallUpdate.
var pendingInstaller atomic.Value // string

// updatesDir is where installers are downloaded. Kept next to the rest of
// Termi's data so an uninstall takes it away too.
func updatesDir() string {
	dir := filepath.Join(configDir(), "updates")
	os.MkdirAll(dir, 0755)
	return dir
}

// uninstallKeyPath composes the registry key the NSIS installer writes, the
// same way wails_tools.nsh does: company name followed by product name, both
// from the embedded wails.json.
func uninstallKeyPath() string {
	return `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + appInfo.CompanyName + appInfo.ProductName
}

// installLocation finds the folder the installed copy of Termi lives in.
//
// InstallLocation is the obvious place to look, but installs made before 7.3
// never wrote it, so DisplayIcon - which points at the executable inside that
// same folder - is the fallback. Without it the first in-app update would fail
// for everyone already out there, which is every current user.
func installLocation() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, uninstallKeyPath(),
		registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("no installed copy of %s found", appInfo.ProductName)
	}
	defer key.Close()

	if loc, _, err := key.GetStringValue("InstallLocation"); err == nil {
		if dir := strings.Trim(strings.TrimSpace(loc), `"`); dir != "" {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir, nil
			}
		}
	}

	if icon, _, err := key.GetStringValue("DisplayIcon"); err == nil {
		// DisplayIcon can carry a ",0" icon index as well as quotes.
		icon = strings.Trim(strings.TrimSpace(icon), `"`)
		if i := strings.LastIndex(icon, ","); i > 2 {
			icon = icon[:i]
		}
		if dir := filepath.Dir(icon); dir != "" && dir != "." {
			if info, err := os.Stat(dir); err == nil && info.IsDir() {
				return dir, nil
			}
		}
	}

	return "", fmt.Errorf("could not work out where %s is installed", appInfo.ProductName)
}

// canSelfUpdate reports whether this build is the installed one, and so safe to
// replace in place.
//
// Running a portable exe while an installed copy exists elsewhere is the case
// this rules out: updating would silently replace the other copy and leave the
// one in front of the user untouched, which is worse than sending them to the
// release page.
func canSelfUpdate() bool {
	dir, err := installLocation()
	if err != nil {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return samePath(filepath.Dir(exe), dir)
}

// isAllowedDownloadURL keeps downloads on GitHub and on https.
func isAllowedDownloadURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	return allowedDownloadHosts[strings.ToLower(u.Hostname())]
}

// httpClient is used for the release metadata and the downloads. Redirects are
// checked as well as the original URL, since that is where a release asset
// actually comes from.
func updateHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if !isAllowedDownloadURL(req.URL.String()) {
				return fmt.Errorf("refusing to follow a redirect to %s", req.URL.Host)
			}
			return nil
		},
	}
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// pickReleaseAssets finds the installer and the checksum list among a release's
// files. The release also carries a standalone exe, which is deliberately not
// used: installing over the existing copy is what keeps the Start Menu entry,
// the PATH entry and the uninstaller consistent.
func pickReleaseAssets(assets []releaseAsset) (installer, checksums releaseAsset) {
	for _, as := range assets {
		switch name := strings.ToLower(as.Name); {
		case strings.HasSuffix(name, "-setup.exe"):
			installer = as
		case name == "sha256sums.txt":
			checksums = as
		}
	}
	return installer, checksums
}

// parseChecksum pulls one file's hash out of a SHA256SUMS.txt.
//
// The file is written by the release workflow as "<hash>  <name>" with CRLF
// line endings, but tolerating the other common shapes - a "*" binary-mode
// marker, a "./" prefix, blank lines - costs nothing and means a hand-made
// checksum file still works.
func parseChecksum(body, name string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		sum := strings.ToLower(fields[0])
		if len(sum) != 64 {
			continue
		}
		file := strings.ToLower(strings.Join(fields[1:], " "))
		file = strings.TrimPrefix(strings.TrimPrefix(file, "*"), "./")
		if file == want {
			return sum, nil
		}
	}
	return "", fmt.Errorf("no checksum listed for %s", name)
}

// fileSHA256 hashes a file in a streaming read, so a 6 MB installer never
// needs to be held in memory twice.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (a *App) emitProgress(p UpdateProgress) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "update-progress", p)
}

// CanInstallUpdate tells the UI whether to offer an in-app update or just a
// link to the release page.
func (a *App) CanInstallUpdate() bool {
	return canSelfUpdate()
}

// DownloadUpdate fetches and verifies the newest installer in the background,
// reporting through update-progress events. It returns straight away.
//
// The release is looked up again here rather than taking URLs from the
// frontend, so nothing that decides what gets executed crosses that boundary.
func (a *App) DownloadUpdate() {
	if !downloading.CompareAndSwap(false, true) {
		return
	}
	go func() {
		defer downloading.Store(false)
		path, err := a.fetchInstaller()
		if err != nil {
			pendingInstaller.Store("")
			a.emitProgress(UpdateProgress{Stage: stageFailed, Message: err.Error()})
			return
		}
		pendingInstaller.Store(path)
		a.emitProgress(UpdateProgress{Stage: stageReady, Percent: 100, Message: filepath.Base(path)})
	}()
}

func (a *App) fetchInstaller() (string, error) {
	client := updateHTTPClient(30 * time.Second)

	req, err := http.NewRequest(http.MethodGet, releasesAPI, nil)
	if err != nil {
		return "", fmt.Errorf("could not ask GitHub for the release")
	}
	req.Header.Set("User-Agent", "Termi/"+appInfo.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("no connection to GitHub")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string         `json:"tag_name"`
		Assets  []releaseAsset `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&release); err != nil {
		return "", fmt.Errorf("could not read GitHub's response")
	}

	installer, checksums := pickReleaseAssets(release.Assets)
	if installer.URL == "" {
		return "", fmt.Errorf("release %s has no installer to download", release.TagName)
	}
	if checksums.URL == "" {
		// Refusing is the right call: without the list there is nothing to
		// check the download against, and this installer runs as admin.
		return "", fmt.Errorf("release %s publishes no checksums, so the download cannot be verified", release.TagName)
	}
	if !isAllowedDownloadURL(installer.URL) || !isAllowedDownloadURL(checksums.URL) {
		return "", fmt.Errorf("the release points somewhere other than GitHub")
	}

	// A half-finished download from a previous attempt must never be mistaken
	// for a verified one.
	target := filepath.Join(updatesDir(), installer.Name)
	part := target + ".part"
	os.Remove(part)

	if err := a.download(client, installer, part); err != nil {
		os.Remove(part)
		return "", err
	}

	a.emitProgress(UpdateProgress{Stage: stageVerifying, Percent: 100, Total: installer.Size, Received: installer.Size})
	if err := verifyChecksum(client, checksums, installer.Name, part); err != nil {
		os.Remove(part)
		return "", err
	}

	os.Remove(target)
	if err := os.Rename(part, target); err != nil {
		os.Remove(part)
		return "", fmt.Errorf("could not save the installer")
	}
	cleanOldInstallers(target)
	return target, nil
}

func (a *App) download(client *http.Client, asset releaseAsset, dest string) error {
	req, err := http.NewRequest(http.MethodGet, asset.URL, nil)
	if err != nil {
		return fmt.Errorf("could not request the installer")
	}
	req.Header.Set("User-Agent", "Termi/"+appInfo.Version)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("the download could not be started")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the download failed with %d", resp.StatusCode)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("could not write to the downloads folder")
	}
	defer f.Close()

	total := asset.Size
	if total <= 0 {
		total = resp.ContentLength
	}

	var received int64
	buf := make([]byte, 64<<10)
	// The event bus is not worth flooding: at 64 KB a chunk a 6 MB installer
	// would otherwise emit a hundred events.
	last := time.Now()
	a.emitProgress(UpdateProgress{Stage: stageDownloading, Total: total})

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return fmt.Errorf("could not write the installer to disk")
			}
			received += int64(n)
			if time.Since(last) > 150*time.Millisecond {
				last = time.Now()
				a.emitProgress(UpdateProgress{
					Stage:    stageDownloading,
					Percent:  percentOf(received, total),
					Received: received,
					Total:    total,
				})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("the download was interrupted")
		}
	}

	if total > 0 && received != total {
		return fmt.Errorf("the download ended early")
	}
	return nil
}

func percentOf(received, total int64) int {
	if total <= 0 {
		return 0
	}
	p := int(received * 100 / total)
	if p > 100 {
		return 100
	}
	return p
}

// verifyChecksum compares the download against the hash published with the
// release.
//
// This proves the file arrived intact and matches what the release lists. It is
// not a signature: both the installer and the list come from the same place, so
// it cannot prove who built them. Proving that needs the signed installer the
// README describes. What it does rule out is a truncated or corrupted download,
// and a swapped asset, before the file is run as administrator.
func verifyChecksum(client *http.Client, checksums releaseAsset, name, path string) error {
	req, err := http.NewRequest(http.MethodGet, checksums.URL, nil)
	if err != nil {
		return fmt.Errorf("could not request the checksums")
	}
	req.Header.Set("User-Agent", "Termi/"+appInfo.Version)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("could not fetch the checksums")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching the checksums failed with %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return fmt.Errorf("could not read the checksums")
	}

	want, err := parseChecksum(string(body), name)
	if err != nil {
		return err
	}
	got, err := fileSHA256(path)
	if err != nil {
		return fmt.Errorf("could not hash the download")
	}
	if got != want {
		return fmt.Errorf("the download does not match its published checksum, so it was discarded")
	}
	return nil
}

// cleanOldInstallers keeps the updates folder from growing by one installer per
// release.
func cleanOldInstallers(keep string) {
	entries, err := os.ReadDir(updatesDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(updatesDir(), e.Name())
		if samePath(path, keep) {
			continue
		}
		os.Remove(path)
	}
}

// InstallUpdate runs the downloaded installer and closes Termi so its files can
// be replaced.
//
// The work is handed to a detached PowerShell helper because this process has
// to be gone before the installer can overwrite termi.exe. The helper waits,
// runs the installer elevated - which is when the UAC prompt appears - and then
// starts Termi again. It starts Termi even if the install failed or the
// elevation prompt was declined, so a refused update never leaves the user
// without their terminal.
func (a *App) InstallUpdate() error {
	installer, _ := pendingInstaller.Load().(string)
	if installer == "" {
		return fmt.Errorf("no downloaded update to install")
	}
	if _, err := os.Stat(installer); err != nil {
		return fmt.Errorf("the downloaded installer is no longer there")
	}

	dir, err := installLocation()
	if err != nil {
		return err
	}
	exe := filepath.Join(dir, filepath.Base(mustExecutable()))

	script := fmt.Sprintf(`$ErrorActionPreference = 'Continue'
# Give Termi a moment to close; the installer cannot replace a running exe.
Start-Sleep -Seconds 2
try {
  Start-Process -FilePath %s -ArgumentList '/S' -Verb RunAs -Wait
} catch {
}
Start-Process -FilePath %s`, psQuote(installer), psQuote(exe))

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-Command", script)
	// Without this the helper flashes a console window as Termi disappears.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start the installer")
	}
	// Let the helper outlive this process.
	go cmd.Process.Release()

	runtime.Quit(a.ctx)
	return nil
}

// mustExecutable is os.Executable with a sensible fallback, used only to
// recover the exe's file name.
func mustExecutable() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return appInfo.ProductName + ".exe"
}

// psQuote wraps a path for PowerShell as a literal single-quoted string, where
// the only character needing escaping is the single quote itself.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
