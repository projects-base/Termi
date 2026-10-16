package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestColumnIndex(t *testing.T) {
	cases := []struct {
		ref  string
		want int
	}{
		{"A1", 0},
		{"B2", 1},
		{"Z9", 25},
		{"AA1", 26},
		{"AB1", 27},
		{"BQ14", 68},
		{"", -1},
		{"14", -1},
	}
	for _, c := range cases {
		if got := columnIndex(c.ref); got != c.want {
			t.Errorf("columnIndex(%q) = %d, want %d", c.ref, got, c.want)
		}
	}
}

func TestFormatExcelDate(t *testing.T) {
	cases := []struct {
		serial float64
		want   string
	}{
		{1, "1900-01-01"},
		{59, "1900-02-28"},
		// 61 is 1 March 1900. Excel counts a 29 February 1900 that never
		// existed, so everything past it must be pulled back a day.
		{61, "1900-03-01"},
		{44927, "2023-01-01"},
		{45000, "2023-03-15"},
		{45000.5, "2023-03-15 12:00"},
		{0.5, "12:00:00"},
	}
	for _, c := range cases {
		if got := formatExcelDate(c.serial); got != c.want {
			t.Errorf("formatExcelDate(%v) = %q, want %q", c.serial, got, c.want)
		}
	}
}

func TestLooksLikeDateFormat(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"", false},
		{"0.00", false},
		{"#,##0", false},
		{"yyyy-mm-dd", true},
		{"d/m/yy", true},
		{"hh:mm:ss", true},
		// The quoted text and the colour section must not be mistaken for
		// format letters - "Days" contains both d and y.
		{`0" Days"`, false},
		{`[Red]#,##0`, false},
		{`[$-409]dd/mm/yyyy`, true},
	}
	for _, c := range cases {
		if got := looksLikeDateFormat(c.code); got != c.want {
			t.Errorf("looksLikeDateFormat(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

func TestSanitizeFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"report", "report"},
		{`a/b\c:d*e?f"g<h>i|j`, "abcdefghij"},
		{"   ", "pasted"},
		{"", "pasted"},
		{"with space", "with space"},
	}
	for _, c := range cases {
		if got := sanitizeFileName(c.in); got != c.want {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPreviewCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.csv")
	// A ragged third row, which exports produce often enough to matter.
	body := "name,role,city\nAda,engineer,London\nGrace\nAlan,maths,Cambridge\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	app := &App{}
	p := app.PreviewFile(path)

	if p.Error != "" {
		t.Fatalf("unexpected error: %s", p.Error)
	}
	if p.Kind != "table" {
		t.Errorf("kind = %q, want table", p.Kind)
	}
	if got, want := len(p.Columns), 3; got != want {
		t.Fatalf("columns = %d, want %d", got, want)
	}
	if p.Columns[0] != "name" || p.Columns[2] != "city" {
		t.Errorf("columns = %v", p.Columns)
	}
	if got, want := len(p.Rows), 3; got != want {
		t.Fatalf("rows = %d, want %d", got, want)
	}
	// The short row must be padded, not dropped, so the grid stays square.
	if got, want := len(p.Rows[1]), 3; got != want {
		t.Errorf("ragged row width = %d, want %d", got, want)
	}
	if p.Rows[1][0] != "Grace" || p.Rows[1][1] != "" {
		t.Errorf("ragged row = %v", p.Rows[1])
	}
	if p.SizeText == "" || p.Modified == "" {
		t.Error("size and modified time should always be populated")
	}
}

func TestPreviewCSVTruncatesLongFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.csv")
	var body strings.Builder
	body.WriteString("id\n")
	for i := range 500 {
		body.WriteRune(rune('0' + i%10))
		body.WriteByte('\n')
	}
	if err := os.WriteFile(path, []byte(body.String()), 0644); err != nil {
		t.Fatal(err)
	}

	p := (&App{}).PreviewFile(path)
	if !p.Truncated {
		t.Error("a 500-row file should be marked truncated")
	}
	if len(p.Rows) > previewRows {
		t.Errorf("rows = %d, want at most %d", len(p.Rows), previewRows)
	}
}

func TestPreviewMissingFile(t *testing.T) {
	p := (&App{}).PreviewFile(filepath.Join(t.TempDir(), "nope.csv"))
	if p.Error == "" {
		t.Error("a missing file should report an error")
	}
	// The name still has to come through: it is how you spot the wrong path.
	if p.Name != "nope.csv" {
		t.Errorf("name = %q, want nope.csv", p.Name)
	}
}

func TestPreviewTextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.log")
	if err := os.WriteFile(path, []byte("line one\r\nline two\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p := (&App{}).PreviewFile(path)
	if p.Kind != "text" {
		t.Fatalf("kind = %q, want text", p.Kind)
	}
	// CRLF would otherwise show as stray characters in the preview.
	if p.Text != "line one\nline two\n" {
		t.Errorf("text = %q", p.Text)
	}
}

func TestPreviewBinaryIsNotShownAsText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blob.dat")
	if err := os.WriteFile(path, []byte{0x01, 0x00, 0x02, 0x03}, 0644); err != nil {
		t.Fatal(err)
	}
	p := (&App{}).PreviewFile(path)
	if p.Kind != "other" {
		t.Errorf("kind = %q, want other", p.Kind)
	}
	if p.Text != "" {
		t.Errorf("binary content should not be rendered as text, got %q", p.Text)
	}
}

// writeTestXLSX builds a minimal but real workbook: shared strings, an inline
// string, a number, a bool, a date-styled serial and a gap in the middle of a
// row - the cases the reader has to get right.
func writeTestXLSX(t *testing.T, path string) {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	add("xl/workbook.xml", `<?xml version="1.0"?>
<workbook><sheets><sheet name="Staff" sheetId="1" r:id="rId1"/></sheets></workbook>`)

	// Two shared strings; the second is split into runs, as styled cells are.
	add("xl/sharedStrings.xml", `<?xml version="1.0"?>
<sst count="2" uniqueCount="2">
  <si><t>name</t></si>
  <si><r><t>start </t></r><r><t>date</t></r></si>
</sst>`)

	// cellXfs index 1 uses numFmtId 14, a built-in date format.
	add("xl/styles.xml", `<?xml version="1.0"?>
<styleSheet>
  <numFmts><numFmt numFmtId="164" formatCode="0&quot; Days&quot;"/></numFmts>
  <cellXfs count="3">
    <xf numFmtId="0"/>
    <xf numFmtId="14"/>
    <xf numFmtId="164"/>
  </cellXfs>
</styleSheet>`)

	add("xl/worksheets/sheet1.xml", `<?xml version="1.0"?>
<worksheet><sheetData>
  <row r="1">
    <c r="A1" t="s"><v>0</v></c>
    <c r="B1" t="s"><v>1</v></c>
    <c r="C1" t="inlineStr"><is><t>active</t></is></c>
    <c r="D1" t="inlineStr"><is><t>tenure</t></is></c>
  </row>
  <row r="2">
    <c r="A2" t="inlineStr"><is><t>Ada</t></is></c>
    <c r="B2" s="1"><v>45000</v></c>
    <c r="C2" t="b"><v>1</v></c>
    <c r="D2" s="2"><v>12</v></c>
  </row>
  <row r="3">
    <c r="A3" t="inlineStr"><is><t>Grace</t></is></c>
    <c r="D3" t="e"><v>#N/A</v></c>
  </row>
</sheetData></worksheet>`)

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPreviewXLSX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "staff.xlsx")
	writeTestXLSX(t, path)

	p := (&App{}).PreviewFile(path)
	if p.Error != "" {
		t.Fatalf("unexpected error: %s", p.Error)
	}
	if p.Kind != "table" {
		t.Fatalf("kind = %q, want table", p.Kind)
	}
	if p.Sheet != "Staff" {
		t.Errorf("sheet = %q, want Staff", p.Sheet)
	}

	// Shared strings resolve, and a run-split string is joined back together.
	want := []string{"name", "start date", "active", "tenure"}
	if len(p.Columns) != len(want) {
		t.Fatalf("columns = %v, want %v", p.Columns, want)
	}
	for i := range want {
		if p.Columns[i] != want[i] {
			t.Errorf("column %d = %q, want %q", i, p.Columns[i], want[i])
		}
	}

	if len(p.Rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(p.Rows))
	}
	// A date-styled serial must read as a date, not as 45000.
	if p.Rows[0][1] != "2023-03-15" {
		t.Errorf("date cell = %q, want 2023-03-15", p.Rows[0][1])
	}
	if p.Rows[0][2] != "TRUE" {
		t.Errorf("bool cell = %q, want TRUE", p.Rows[0][2])
	}
	// numFmtId 164 is `0" Days"`, which only looks like a date format.
	if p.Rows[0][3] != "12" {
		t.Errorf("number cell = %q, want 12", p.Rows[0][3])
	}
	// The missing B3 and C3 must leave gaps, keeping D3 in its own column.
	if p.Rows[1][0] != "Grace" || p.Rows[1][1] != "" || p.Rows[1][2] != "" {
		t.Errorf("sparse row = %v", p.Rows[1])
	}
	if p.Rows[1][3] != "#N/A" {
		t.Errorf("error cell = %q, want #N/A", p.Rows[1][3])
	}
}

func TestPreviewCorruptXLSX(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.xlsx")
	if err := os.WriteFile(path, []byte("not a zip"), 0644); err != nil {
		t.Fatal(err)
	}
	p := (&App{}).PreviewFile(path)
	if p.Error == "" {
		t.Error("a corrupt workbook should report an error")
	}
	if p.Name != "broken.xlsx" {
		t.Errorf("name = %q", p.Name)
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{2048, "2.0 KB"},
		{5 << 20, "5.0 MB"},
	}
	for _, c := range cases {
		if got := humanSize(c.n); got != c.want {
			t.Errorf("humanSize(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
