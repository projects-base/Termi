package main

import (
	"strings"
	"testing"
)

const testChangelog = `# Changelog

Some preamble that is not part of any entry.

## 7.4.0 — 2026-10-17

### Added

- What's new, in the app.

## 7.3.0 — 2026-10-17

### Added

- A new version tells you about itself.

### Fixed

- The installer no longer stacks a duplicate PATH entry.

## 7.1.0 — 2026-10-17

### Fixed

- Pasting a screenshot works.

## 3.0.0 — 2026-04-07

The first tagged release.
`

func TestChangelogSection(t *testing.T) {
	entry, ok := changelogSection(testChangelog, "7.3.0")
	if !ok {
		t.Fatal("7.3.0 should be found")
	}
	if entry.Heading != "7.3.0 — 2026-10-17" {
		t.Errorf("heading = %q", entry.Heading)
	}
	// The body must stop at the next heading, not run into 7.1.0.
	if strings.Contains(entry.Body, "Pasting a screenshot") {
		t.Error("the body ran past the end of its own section")
	}
	if !strings.Contains(entry.Body, "duplicate PATH entry") {
		t.Error("the body should include every subsection of its own entry")
	}
	if !strings.Contains(entry.Body, "### Added") {
		t.Error("subsection headings should be kept")
	}
	// The preamble above the first heading belongs to nothing.
	if strings.Contains(entry.Body, "Some preamble") {
		t.Error("the preamble leaked into an entry")
	}
	if entry.Source != "embedded" {
		t.Errorf("source = %q, want embedded", entry.Source)
	}

	// The last entry in the file has no following heading to stop at.
	entry, ok = changelogSection(testChangelog, "3.0.0")
	if !ok {
		t.Fatal("3.0.0 should be found")
	}
	if entry.Body != "The first tagged release." {
		t.Errorf("last entry body = %q", entry.Body)
	}

	if _, ok := changelogSection(testChangelog, "9.9.9"); ok {
		t.Error("a version with no entry must not be found")
	}
}

// Headings are matched on the version, not on the text, because the tag is
// written two-part in this repo while wails.json is three-part.
func TestChangelogSectionVersionForms(t *testing.T) {
	doc := "## 7.3\n\nTwo-part heading.\n"
	for _, v := range []string{"7.3", "7.3.0", "v7.3.0"} {
		if _, ok := changelogSection(doc, v); !ok {
			t.Errorf("%q should match a '## 7.3' heading", v)
		}
	}

	doc = "## v7.3.0 - dated\n\nPrefixed heading.\n"
	if _, ok := changelogSection(doc, "7.3.0"); !ok {
		t.Error("a heading written with a leading v should still match")
	}

	// A near miss must not match.
	if _, ok := changelogSection("## 7.3.1\n\nx\n", "7.3.0"); ok {
		t.Error("7.3.1 must not answer for 7.3.0")
	}
}

// CRLF is what this file has on disk after a checkout on Windows, so a parser
// that leaves the carriage return in place would put it in every heading.
func TestChangelogSectionHandlesCRLF(t *testing.T) {
	doc := "## 7.4.0 — 2026-10-17\r\n\r\n### Added\r\n\r\n- A thing.\r\n\r\n## 7.3.0 — old\r\n\r\n- Older.\r\n"
	entry, ok := changelogSection(doc, "7.4.0")
	if !ok {
		t.Fatal("not found")
	}
	if strings.Contains(entry.Heading, "\r") || strings.Contains(entry.Body, "\r") {
		t.Errorf("carriage returns survived: heading=%q body=%q", entry.Heading, entry.Body)
	}
	if strings.Contains(entry.Body, "Older") {
		t.Error("the body ran past its section")
	}
}

func TestTagFor(t *testing.T) {
	cases := []struct{ in, want string }{
		// A zero patch is tagged two-part in this repo.
		{"7.4.0", "v7.4"},
		{"7.3.0", "v7.3"},
		{"v7.3.0", "v7.3"},
		// A non-zero patch keeps all three parts.
		{"7.3.1", "v7.3.1"},
		{"6.1.0", "v6.1"},
	}
	for _, c := range cases {
		if got := tagFor(c.in); got != c.want {
			t.Errorf("tagFor(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The real file ships in the binary, so it has to parse and it has to have an
// entry for this build - otherwise the first launch after an update shows an
// error instead of the notes.
func TestEmbeddedChangelogHasThisVersion(t *testing.T) {
	entry, ok := changelogSection(changelogMarkdown, appInfo.Version)
	if !ok {
		t.Fatalf("CHANGELOG.md has no entry for %s, which is this build", appInfo.Version)
	}
	if strings.TrimSpace(entry.Body) == "" {
		t.Errorf("the entry for %s is empty", appInfo.Version)
	}
	if !strings.Contains(entry.Heading, appInfo.Version) {
		t.Errorf("heading %q should name %s", entry.Heading, appInfo.Version)
	}
}

// Every version the changelog claims to document should be findable, which
// catches a heading typed in a shape the parser cannot read.
func TestEmbeddedChangelogHeadingsAllParse(t *testing.T) {
	var found int
	for _, line := range strings.Split(changelogMarkdown, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "## ") {
			continue
		}
		found++
		heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
		version := heading
		if sp := strings.IndexAny(version, " \t"); sp > 0 {
			version = version[:sp]
		}
		if parseVersion(version) == [3]int{} {
			t.Errorf("heading %q does not start with a version the parser can read", heading)
			continue
		}
		if _, ok := changelogSection(changelogMarkdown, version); !ok {
			t.Errorf("heading %q cannot be looked up by its own version", heading)
		}
	}
	if found < 5 {
		t.Errorf("only %d version headings found; the file should document the releases", found)
	}
}
