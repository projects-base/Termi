package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The release workflow writes "<hash>  <name>" with CRLF line endings, so the
// carriage return is the case that matters most here - left in place it would
// make every single comparison fail.
func TestParseChecksum(t *testing.T) {
	body := "ea4228cb2d6919cd5d4387f9fa8a4abb06fbbbc5f24bea31d8125554bb48ed97  Termi-v7.1.0-setup.exe\r\n" +
		"5a75e663c64c79c3e39d7080e5c0ac4f0dd38e4d0ab30a1aeac2f377915fdf24  Termi-v7.1.0.exe\r\n"

	got, err := parseChecksum(body, "Termi-v7.1.0-setup.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ea4228cb2d6919cd5d4387f9fa8a4abb06fbbbc5f24bea31d8125554bb48ed97" {
		t.Errorf("hash = %q", got)
	}

	// The second entry must not be confused with the first: one name is a
	// prefix of the other once "-setup" is dropped.
	got, err = parseChecksum(body, "Termi-v7.1.0.exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(got, "5a75e663") {
		t.Errorf("hash = %q, want the second entry", got)
	}

	if _, err := parseChecksum(body, "Termi-v9.9.9-setup.exe"); err == nil {
		t.Error("a name that is not listed must be an error, not an empty hash")
	}
}

func TestParseChecksumTolerance(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"binary mode marker", "a" + strings.Repeat("b", 63) + " *file.exe\n"},
		{"leading ./", "a" + strings.Repeat("b", 63) + "  ./file.exe\n"},
		{"blank lines and a comment", "\n# generated\n\na" + strings.Repeat("b", 63) + "  file.exe\n"},
		{"no trailing newline", "a" + strings.Repeat("b", 63) + "  file.exe"},
	}
	for _, c := range cases {
		if _, err := parseChecksum(c.body, "file.exe"); err != nil {
			t.Errorf("%s: %v", c.name, err)
		}
	}

	// A line whose first field is not a 64-character hash is not a checksum,
	// and must not be read as one.
	if _, err := parseChecksum("sha256 file.exe\n", "file.exe"); err == nil {
		t.Error("a short first field should not be accepted as a hash")
	}
}

// The installer is run as administrator, so where it is allowed to come from is
// a security boundary rather than a tidiness rule.
func TestIsAllowedDownloadURL(t *testing.T) {
	allowed := []string{
		"https://github.com/projects-base/Termi/releases/download/v7.2/Termi-v7.2.0-setup.exe",
		"https://objects.githubusercontent.com/github-production-release-asset/1/2",
		"https://release-assets.githubusercontent.com/github-production-release-asset/x",
		"https://api.github.com/repos/projects-base/Termi/releases/latest",
	}
	for _, u := range allowed {
		if !isAllowedDownloadURL(u) {
			t.Errorf("should be allowed: %s", u)
		}
	}

	refused := []string{
		// Plain http, even on an allowed host.
		"http://github.com/projects-base/Termi/releases/download/v7.2/setup.exe",
		// A host that merely ends with the allowed one.
		"https://github.com.attacker.example/setup.exe",
		// A host that merely contains it.
		"https://evil.example/github.com/setup.exe",
		// Credentials in the userinfo section, which some parsers mistake for
		// the host.
		"https://github.com@evil.example/setup.exe",
		"https://raw.githubusercontent.com/projects-base/Termi/setup.exe",
		"file:///C:/Windows/System32/calc.exe",
		"",
		"not a url at all",
	}
	for _, u := range refused {
		if isAllowedDownloadURL(u) {
			t.Errorf("should be refused: %s", u)
		}
	}
}

func TestPickReleaseAssets(t *testing.T) {
	assets := []releaseAsset{
		{Name: "SHA256SUMS.txt", URL: "https://github.com/s"},
		{Name: "Termi-v7.2.0.exe", URL: "https://github.com/portable", Size: 11},
		{Name: "Termi-v7.2.0-setup.exe", URL: "https://github.com/setup", Size: 6},
	}
	installer, checksums := pickReleaseAssets(assets)
	// The standalone exe must not be picked: installing over the existing copy
	// is what keeps the shortcuts, the PATH entry and the uninstaller right.
	if installer.URL != "https://github.com/setup" {
		t.Errorf("installer = %+v", installer)
	}
	if checksums.URL != "https://github.com/s" {
		t.Errorf("checksums = %+v", checksums)
	}

	// A release with neither must come back empty rather than guessing.
	installer, checksums = pickReleaseAssets([]releaseAsset{{Name: "notes.md"}})
	if installer.URL != "" || checksums.URL != "" {
		t.Errorf("expected no assets, got %+v and %+v", installer, checksums)
	}
}

func TestPercentOf(t *testing.T) {
	cases := []struct {
		received, total int64
		want            int
	}{
		{0, 100, 0},
		{50, 100, 50},
		{100, 100, 100},
		// An unknown total must not divide by zero.
		{500, 0, 0},
		{500, -1, 0},
		// A server that sends more than it promised must not report 120%.
		{120, 100, 100},
	}
	for _, c := range cases {
		if got := percentOf(c.received, c.total); got != c.want {
			t.Errorf("percentOf(%d, %d) = %d, want %d", c.received, c.total, got, c.want)
		}
	}
}

// The install path is pasted into a PowerShell command, so quoting it wrongly
// would be a command-injection hole rather than a cosmetic bug.
func TestPSQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{`C:\Program Files\Projects Base\Termi\termi.exe`, `'C:\Program Files\Projects Base\Termi\termi.exe'`},
		{`C:\it's here\x.exe`, `'C:\it''s here\x.exe'`},
		{`'; Start-Process calc; '`, `'''; Start-Process calc; '''`},
	}
	for _, c := range cases {
		if got := psQuote(c.in); got != c.want {
			t.Errorf("psQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFileSHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(path, []byte("abc"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := fileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	// The well-known SHA-256 of "abc".
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got != want {
		t.Errorf("fileSHA256 = %q, want %q", got, want)
	}

	if _, err := fileSHA256(filepath.Join(dir, "missing")); err == nil {
		t.Error("hashing a missing file should fail")
	}
}

// The uninstall key has to match what the installer writes, or the updater
// cannot find the install folder. The space in "Projects Base" is load-bearing.
func TestUninstallKeyPath(t *testing.T) {
	got := uninstallKeyPath()
	if !strings.HasSuffix(got, appInfo.CompanyName+appInfo.ProductName) {
		t.Errorf("key = %q, should end with %q", got, appInfo.CompanyName+appInfo.ProductName)
	}
	if !strings.HasPrefix(got, `Software\Microsoft\Windows\CurrentVersion\Uninstall\`) {
		t.Errorf("key = %q", got)
	}
}
