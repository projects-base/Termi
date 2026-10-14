package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// wails.json is embedded so the version shown in the UI, the version compiled
// into the exe, and the version the release workflow checks the tag against all
// come from one place and cannot drift.
//
//go:embed wails.json
var wailsConfigJSON []byte

// releasesAPI is the GitHub endpoint for the newest published release.
// /latest excludes drafts and prereleases, which is what we want for an
// update prompt.
const releasesAPI = "https://api.github.com/repos/projects-base/Termi/releases/latest"

// AppInfo describes this build for the UI's About section.
type AppInfo struct {
	Version     string `json:"version"`
	ProductName string `json:"productName"`
	CompanyName string `json:"companyName"`
	Copyright   string `json:"copyright"`
}

// UpdateCheck is the outcome of looking for a newer release. A failed check is
// reported in Error rather than as a Go error, so the UI can stay quiet about
// the usual causes (offline, rate limited) without a rejected promise.
type UpdateCheck struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Available      bool   `json:"available"`
	ReleaseURL     string `json:"releaseURL"`
	PublishedAt    string `json:"publishedAt"`
	Error          string `json:"error"`
}

var appInfo = loadAppInfo()

func loadAppInfo() AppInfo {
	var cfg struct {
		Name string `json:"name"`
		Info struct {
			ProductName    string `json:"productName"`
			ProductVersion string `json:"productVersion"`
			CompanyName    string `json:"companyName"`
			Copyright      string `json:"copyright"`
		} `json:"info"`
	}
	if err := json.Unmarshal(wailsConfigJSON, &cfg); err != nil {
		return AppInfo{Version: "0.0.0", ProductName: "Termi"}
	}

	name := cfg.Info.ProductName
	if name == "" {
		name = cfg.Name
	}
	version := cfg.Info.ProductVersion
	if version == "" {
		version = "0.0.0"
	}
	return AppInfo{
		Version:     version,
		ProductName: name,
		CompanyName: cfg.Info.CompanyName,
		Copyright:   cfg.Info.Copyright,
	}
}

// GetAppInfo returns this build's name and version for the UI.
func (a *App) GetAppInfo() AppInfo {
	return appInfo
}

// CheckForUpdate asks GitHub for the newest release and compares it with this
// build. It never blocks for long: the request is capped at 10 seconds.
func (a *App) CheckForUpdate() UpdateCheck {
	result := UpdateCheck{CurrentVersion: appInfo.Version}

	req, err := http.NewRequest(http.MethodGet, releasesAPI, nil)
	if err != nil {
		result.Error = "Could not build the update request"
		return result
	}
	// GitHub rejects requests without a User-Agent.
	req.Header.Set("User-Agent", "Termi/"+appInfo.Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Error = "No connection to GitHub"
		return result
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		result.Error = "No releases published yet"
		return result
	case resp.StatusCode == http.StatusForbidden, resp.StatusCode == http.StatusTooManyRequests:
		result.Error = "GitHub rate limit reached — try again later"
		return result
	case resp.StatusCode != http.StatusOK:
		result.Error = fmt.Sprintf("GitHub returned %d", resp.StatusCode)
		return result
	}

	// Cap the read; the payload is a few KB and we only need three fields.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		result.Error = "Could not read GitHub's response"
		return result
	}

	var release struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		result.Error = "Could not read GitHub's response"
		return result
	}
	if release.TagName == "" {
		result.Error = "No releases published yet"
		return result
	}

	result.LatestVersion = strings.TrimPrefix(release.TagName, "v")
	result.ReleaseURL = release.HTMLURL
	result.PublishedAt = release.PublishedAt
	result.Available = compareVersions(release.TagName, appInfo.Version) > 0
	return result
}

// OpenURL hands a link to the system browser. Used for the release page.
func (a *App) OpenURL(url string) {
	if !strings.HasPrefix(url, "https://") {
		return
	}
	runtime.BrowserOpenURL(a.ctx, url)
}

// parseVersion turns "v5.0", "5.0.0" or "5.1.0-beta2" into comparable parts.
// Tags in this repo are written two-part (v5.0) while wails.json is three-part
// (5.0.0), so missing parts count as zero.
func parseVersion(v string) [3]int {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}

	var out [3]int
	for i, part := range strings.Split(v, ".") {
		if i > 2 {
			break
		}
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			break
		}
		out[i] = n
	}
	return out
}

// compareVersions reports whether a is older (-1), the same (0) or newer (+1)
// than b, ignoring any prerelease suffix.
func compareVersions(a, b string) int {
	av, bv := parseVersion(a), parseVersion(b)
	for i := range 3 {
		switch {
		case av[i] < bv[i]:
			return -1
		case av[i] > bv[i]:
			return 1
		}
	}
	return 0
}
