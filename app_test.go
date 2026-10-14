package main

import "testing"

func TestNormalizeInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text is untouched", "dir", "dir"},
		{"single line keeps its newline so it runs", "dir\n", "dir\r"},
		{"single line with CRLF keeps one CR", "dir\r\n", "dir\r"},
		{"lone CR is kept", "dir\r", "dir\r"},
		{"multi-line block drops the trailing newline", "one\ntwo\n", "one\rtwo"},
		{"multi-line CRLF block", "one\r\ntwo\r\nthree\r\n", "one\rtwo\rthree"},
		{"multi-line without trailing newline", "one\ntwo", "one\rtwo"},
		{"several trailing newlines collapse away", "one\ntwo\n\n\n", "one\rtwo"},
		{"NULs are stripped", "di\x00r", "dir"},
		{"blank lines inside are preserved", "one\n\ntwo", "one\r\rtwo"},
		{"empty stays empty", "", ""},
		{"newlines alone submit once", "\n\n", "\r"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeInput(tc.in); got != tc.want {
				t.Errorf("normalizeInput(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSamePath(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{`C:\Users\foo`, `C:\Users\foo`, true},
		{`C:\Users\foo`, `c:\users\FOO`, true},
		{`C:\Users\foo`, `C:/Users/foo`, true},
		{`C:\Users\foo\`, `C:\Users\foo`, true},
		{`  C:\Users\foo  `, `C:\Users\foo`, true},
		{`C:\`, `C:\`, true},
		{`C:\Users\foo`, `C:\Users\bar`, false},
		{`C:\Users\foo`, `C:\Users\foobar`, false},
	}

	for _, tc := range cases {
		if got := samePath(tc.a, tc.b); got != tc.want {
			t.Errorf("samePath(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
	}{
		{"5.0.0", [3]int{5, 0, 0}},
		{"v5.0.0", [3]int{5, 0, 0}},
		{"v5.0", [3]int{5, 0, 0}}, // two-part tags used in this repo
		{"v5", [3]int{5, 0, 0}},
		{"5.1.2", [3]int{5, 1, 2}},
		{"v5.1-beta2", [3]int{5, 1, 0}}, // prerelease suffix ignored
		{"5.0.0+build7", [3]int{5, 0, 0}},
		{" v5.0.0 ", [3]int{5, 0, 0}},
		{"garbage", [3]int{0, 0, 0}},
		{"", [3]int{0, 0, 0}},
	}
	for _, tc := range cases {
		if got := parseVersion(tc.in); got != tc.want {
			t.Errorf("parseVersion(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v5.1", "5.0.0", 1},  // newer release available
		{"v5.0", "5.0.0", 0},  // same, written differently
		{"v4.0", "5.0.0", -1}, // older release than this build
		{"v5.0.1", "5.0.0", 1},
		{"v6.0", "5.9.9", 1},
		{"v5.0-beta1", "5.0.0", 0}, // prerelease of the same version
		{"garbage", "5.0.0", -1},
	}
	for _, tc := range cases {
		if got := compareVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestLoadAppInfoFromEmbeddedConfig(t *testing.T) {
	info := loadAppInfo()
	if info.Version == "" || info.Version == "0.0.0" {
		t.Errorf("version not read from embedded wails.json: %q", info.Version)
	}
	if info.ProductName != "Termi" {
		t.Errorf("ProductName = %q, want %q", info.ProductName, "Termi")
	}
	// The UI shows this, so a three-part version is the expectation.
	if parseVersion(info.Version) == [3]int{0, 0, 0} {
		t.Errorf("version %q does not parse", info.Version)
	}
}
