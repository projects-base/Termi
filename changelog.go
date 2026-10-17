package main

// Reading the changelog, for two different moments.
//
// After an update, the entry wanted is this build's own, and it is already
// here: CHANGELOG.md is embedded, so that case needs no network and works
// offline.
//
// Before an update, the entry wanted belongs to a version this build has never
// seen, so it has to be fetched. The file is read from the repository at that
// tag; a release whose entry was never written falls back to the commit
// subjects in the range, so the panel is never simply empty.

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

//go:embed CHANGELOG.md
var changelogMarkdown string

// Changelog is one version's entry.
type Changelog struct {
	Version string `json:"version"`
	// Heading is the whole heading line as written, which carries the date.
	Heading string `json:"heading"`
	Body    string `json:"body"`
	// Source is "embedded", "remote", "commits" or "" - shown to no one, but
	// it makes the fallback visible when something looks wrong.
	Source string `json:"source"`
	Error  string `json:"error"`
}

// changelogSection pulls one version's entry out of a changelog document.
//
// Headings are matched on the normalised version rather than on the text, so
// "## 7.3.0 - 2026-10-17", "## 7.3" and "## v7.3.0" all answer for 7.3.0. The
// body runs to the next "## " heading.
func changelogSection(doc, version string) (Changelog, bool) {
	want := parseVersion(version)
	lines := strings.Split(doc, "\n")

	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if !strings.HasPrefix(trimmed, "## ") {
			continue
		}
		heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
		// The first word of the heading is the version; the rest is the date
		// or a note such as "unreleased".
		first := heading
		if sp := strings.IndexAny(first, " \t"); sp > 0 {
			first = first[:sp]
		}
		if parseVersion(first) != want {
			continue
		}

		var body []string
		for _, next := range lines[i+1:] {
			if strings.HasPrefix(strings.TrimRight(next, "\r"), "## ") {
				break
			}
			body = append(body, strings.TrimRight(next, "\r"))
		}
		return Changelog{
			Version: version,
			Heading: heading,
			Body:    strings.TrimSpace(strings.Join(body, "\n")),
			Source:  "embedded",
		}, true
	}
	return Changelog{}, false
}

// WhatsNew returns this build's own changelog entry, for the first launch after
// an update. No network involved.
func (a *App) WhatsNew() Changelog {
	entry, ok := changelogSection(changelogMarkdown, appInfo.Version)
	if !ok {
		return Changelog{
			Version: appInfo.Version,
			Error:   fmt.Sprintf("No changelog entry for %s", appInfo.Version),
		}
	}
	return entry
}

// PendingVersionNotes returns the changelog entry for a version that is not
// installed yet, so the update bar can show what is coming before it is
// downloaded.
func (a *App) PendingVersionNotes(version string) Changelog {
	version = strings.TrimSpace(strings.TrimPrefix(version, "v"))
	if version == "" {
		return Changelog{Error: "No version to look up"}
	}

	client := updateHTTPClient(15 * time.Second)

	// The tag is written two-part in this repo (v7.3) while the version is
	// three-part, so both spellings are worth trying before giving up.
	tags := []string{"v" + version}
	if parts := strings.Split(version, "."); len(parts) == 3 && parts[2] == "0" {
		tags = append(tags, "v"+parts[0]+"."+parts[1])
	}

	for _, tag := range tags {
		doc, err := fetchChangelogAt(client, tag)
		if err != nil {
			continue
		}
		if entry, ok := changelogSection(doc, version); ok {
			entry.Source = "remote"
			return entry
		}
	}

	// No entry was written for it. The commit subjects for the range are a
	// poorer read, but they are honest and better than an empty panel.
	if entry, err := commitsSince(client, appInfo.Version, version); err == nil {
		return entry
	}

	return Changelog{
		Version: version,
		Error:   "No notes published for this version",
	}
}

// fetchChangelogAt reads CHANGELOG.md from the repository at a tag.
//
// The contents API is used rather than raw.githubusercontent.com so that the
// host stays one already trusted for release metadata.
func fetchChangelogAt(client *http.Client, tag string) (string, error) {
	url := fmt.Sprintf(
		"https://api.github.com/repos/projects-base/Termi/contents/CHANGELOG.md?ref=%s", tag)
	body, err := getJSON(client, url)
	if err != nil {
		return "", err
	}

	var payload struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.Encoding != "base64" {
		return "", fmt.Errorf("unexpected encoding %q", payload.Encoding)
	}
	// The API wraps the base64 at 60 characters.
	clean := strings.NewReplacer("\n", "", "\r", "").Replace(payload.Content)
	decoded, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// commitsSince lists the commit subjects between two versions' tags.
//
// The compare endpoint wants the older tag first, so the two are ordered here
// rather than trusting the caller: asking for a pending version from a newer
// local build - a development build ahead of every release - would otherwise
// send them the wrong way round and get nothing back.
func commitsSince(client *http.Client, from, to string) (Changelog, error) {
	base, head := from, to
	if compareVersions(base, head) > 0 {
		base, head = head, base
	}
	url := fmt.Sprintf(
		"https://api.github.com/repos/projects-base/Termi/compare/%s...%s",
		tagFor(base), tagFor(head))
	body, err := getJSON(client, url)
	if err != nil {
		return Changelog{}, err
	}

	var payload struct {
		Commits []struct {
			Commit struct {
				Message string `json:"message"`
			} `json:"commit"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return Changelog{}, err
	}
	if len(payload.Commits) == 0 {
		return Changelog{}, fmt.Errorf("no commits in range")
	}

	var out []string
	// Newest first reads better than the API's oldest-first order.
	for i := len(payload.Commits) - 1; i >= 0; i-- {
		subject := payload.Commits[i].Commit.Message
		if nl := strings.IndexByte(subject, '\n'); nl >= 0 {
			subject = subject[:nl]
		}
		subject = strings.TrimSpace(subject)
		// A version bump is not news to the person reading this.
		if subject == "" || strings.HasPrefix(strings.ToLower(subject), "bump the version") {
			continue
		}
		out = append(out, "- "+subject)
	}
	if len(out) == 0 {
		return Changelog{}, fmt.Errorf("nothing worth listing in range")
	}

	return Changelog{
		Version: to,
		Heading: to,
		Body: "No changelog entry was written for this release, so here is what " +
			"changed in it:\n\n" + strings.Join(out, "\n"),
		Source: "commits",
	}, nil
}

// tagFor guesses the tag for a version. Tags in this repo are two-part when the
// patch is zero (v7.3) and three-part otherwise.
func tagFor(version string) string {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if parts := strings.Split(version, "."); len(parts) == 3 && parts[2] == "0" {
		return "v" + parts[0] + "." + parts[1]
	}
	return "v" + version
}

func getJSON(client *http.Client, url string) ([]byte, error) {
	if !isAllowedDownloadURL(url) {
		return nil, fmt.Errorf("refusing to fetch %s", url)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Termi/"+appInfo.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub returned %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// TakeWhatsNew reports whether this launch is the first one after an update,
// and records the current version so the next launch is not.
//
// A fresh install returns false: greeting someone with a list of changes to a
// version they have never run is noise, not news.
func (a *App) TakeWhatsNew() bool {
	var previous string
	// Read-change-write against the file rather than against this process's
	// copy. This write lands immediately after a release is installed, which
	// is exactly when a second window is likely to be open and to have saved
	// something that must not be undone.
	a.mutateSettings(func(cur *Settings) {
		previous = strings.TrimSpace(cur.LastRunVersion)
		cur.LastRunVersion = appInfo.Version
	})

	if previous == "" {
		return false
	}
	return compareVersions(appInfo.Version, previous) > 0
}
