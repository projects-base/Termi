package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const aFolder = `C:\Users\me\Projects`

// Settings are one file, and more than one thing writes it: the Settings panel,
// the update banner's Skip, and the first launch after an update. A writer that
// only owns one field must not take the whole file back to whatever it last
// read, or a setting the user just chose disappears.
//
// Two Termi windows is all it takes, and installing a release makes exactly
// that happen - the installer offers to launch Termi while one is already open.
func TestSkipUpdateVersionKeepsOtherSettings(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	// One window saves the user's startup folder.
	first := NewApp()
	s := defaultSettings()
	s.StartupDir = aFolder
	if err := first.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	// Another window, opened before that, is still holding the settings as
	// they were. It writes one unrelated field.
	second := NewApp()
	if err := second.SkipUpdateVersion("9.9.9"); err != nil {
		t.Fatal(err)
	}

	got := NewApp().LoadSettings()
	if got.StartupDir != aFolder {
		t.Errorf("startup folder was lost: got %q, want %q", got.StartupDir, aFolder)
	}
	if got.SkippedVersion != "9.9.9" {
		t.Errorf("skippedVersion = %q, want 9.9.9", got.SkippedVersion)
	}
}

// The first launch after an update writes lastRunVersion. It must not carry a
// stale copy of everything else with it - this is the write that happens
// immediately after installing a new release.
func TestTakeWhatsNewKeepsOtherSettings(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	first := NewApp()
	s := defaultSettings()
	s.StartupDir = aFolder
	s.LastRunVersion = "1.0.0"
	if err := first.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	// A window that loaded before the folder was chosen.
	second := NewApp()
	second.settings.LastRunVersion = "1.0.0"
	second.TakeWhatsNew()

	got := NewApp().LoadSettings()
	if got.StartupDir != aFolder {
		t.Errorf("startup folder was lost: got %q, want %q", got.StartupDir, aFolder)
	}
	if got.LastRunVersion != appInfo.Version {
		t.Errorf("lastRunVersion = %q, want %q", got.LastRunVersion, appInfo.Version)
	}
}

// The panel sends the fields it owns. It must not be able to blank the ones it
// does not - a skipped version, or the version that last ran.
func TestSaveSettingsDoesNotClearFieldsItDoesNotOwn(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	// Planted through the methods that own these fields, since SaveSettings
	// deliberately cannot set them.
	setup := NewApp()
	if err := setup.SkipUpdateVersion("8.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := setup.mutateSettings(func(cur *Settings) {
		cur.LastRunVersion = "7.4.0"
	}); err != nil {
		t.Fatal(err)
	}

	// What the Settings panel sends: its own fields, with the others empty.
	panel := NewApp()
	if err := panel.SaveSettings(Settings{
		FontSize:      16,
		Theme:         "light",
		Shell:         "CMD",
		StartupDir:    aFolder,
		NotifyUpdates: true,
	}); err != nil {
		t.Fatal(err)
	}

	got := NewApp().LoadSettings()
	if got.FontSize != 16 || got.Theme != "light" || got.Shell != "CMD" || got.StartupDir != aFolder {
		t.Errorf("the panel's own fields did not apply: %+v", got)
	}
	if got.SkippedVersion != "8.0.0" {
		t.Errorf("skippedVersion was cleared: %q", got.SkippedVersion)
	}
	if got.LastRunVersion != "7.4.0" {
		t.Errorf("lastRunVersion was cleared: %q", got.LastRunVersion)
	}
}

// Turning the update notice off has to survive, since it is the panel's own
// field and false is also the zero value.
func TestSaveSettingsCanTurnNotifyUpdatesOff(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	app := NewApp()
	s := defaultSettings()
	s.NotifyUpdates = false
	if err := app.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	if got := NewApp().LoadSettings(); got.NotifyUpdates {
		t.Error("notifyUpdates should have stayed off")
	}
}

// A settings file is replaced, never truncated in place, so an interrupted
// write cannot leave half a file behind for the next launch to fail on.
func TestWriteSettingsLeavesNoPartialFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	app := NewApp()
	s := defaultSettings()
	s.StartupDir = aFolder
	if err := app.SaveSettings(s); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "termi", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var round Settings
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}

	// No temporary file should be left next to it.
	entries, err := os.ReadDir(filepath.Join(dir, "termi"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "settings.json" && e.Name() != "updates" && e.Name() != "attachments" {
			t.Errorf("unexpected leftover file: %s", e.Name())
		}
	}
}
