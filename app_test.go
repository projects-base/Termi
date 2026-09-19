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
