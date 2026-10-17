package main

import (
	"encoding/json"
	"testing"
)

// shouldNotify is the whole of the launch banner's policy, so the cases that
// matter are the ones where a banner would be wrong: no update, the exact
// version already skipped, and - the easy one to get backwards - a release
// newer than the skipped one, which must still be announced.
func TestShouldNotify(t *testing.T) {
	cases := []struct {
		name      string
		available bool
		latest    string
		skipped   string
		want      bool
	}{
		{"no update available", false, "7.2.0", "", false},
		{"update available, nothing skipped", true, "7.2.0", "", true},
		{"the skipped version itself", true, "7.2.0", "7.2.0", false},
		{"two-part tag against a skipped three-part", true, "7.2", "7.2.0", false},
		{"newer than the skipped version", true, "8.0.0", "7.2.0", true},
		{"a minor bump past the skipped version", true, "7.3.0", "7.2.0", true},
		{"older than the skipped version", true, "7.1.0", "7.2.0", false},
		// A skip must never suppress forever: an unavailable check is already
		// false above, so this only guards the comparison direction.
		{"skipped is empty string", true, "0.0.1", "", true},
	}

	for _, c := range cases {
		got := shouldNotify(UpdateCheck{Available: c.available, LatestVersion: c.latest}, c.skipped)
		if got != c.want {
			t.Errorf("%s: shouldNotify(available=%v latest=%q, skipped=%q) = %v, want %v",
				c.name, c.available, c.latest, c.skipped, got, c.want)
		}
	}
}

// A settings.json written before the toggle existed has no notifyUpdates key.
// Because LoadSettings unmarshals over the defaults, that absence has to read
// as "on" - the opposite would turn the notice off for every existing user.
func TestSettingsDefaultNotifyUpdates(t *testing.T) {
	if !defaultSettings().NotifyUpdates {
		t.Fatal("notifications should default to on")
	}

	s := defaultSettings()
	if err := json.Unmarshal([]byte(`{"fontSize":16,"theme":"light"}`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.NotifyUpdates {
		t.Error("a settings file without notifyUpdates should keep the default of on")
	}
	if s.FontSize != 16 || s.Theme != "light" {
		t.Errorf("the rest of the file should still apply, got %+v", s)
	}

	// An explicit false has to survive, or the toggle could never be turned off.
	s = defaultSettings()
	if err := json.Unmarshal([]byte(`{"notifyUpdates":false}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.NotifyUpdates {
		t.Error("an explicit false must be honoured")
	}
}

// The Settings panel sends the whole object back, so a round trip has to
// preserve a skipped version it does not edit.
func TestSettingsRoundTripKeepsSkippedVersion(t *testing.T) {
	s := defaultSettings()
	s.SkippedVersion = "7.2.0"

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	loaded := defaultSettings()
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.SkippedVersion != "7.2.0" {
		t.Errorf("skippedVersion = %q, want 7.2.0", loaded.SkippedVersion)
	}
}
