package main

// Attachments: dropping or pasting a file into the command bar puts a usable
// path into the command and shows enough of the file to confirm it is the right
// one before you run anything.
//
// Two problems have to be solved separately. Chromium deliberately hides the
// real location of a pasted file - the DOM gives content and a name but no
// path - so the paths are read from the Windows clipboard directly (CF_HDROP).
// A pasted screenshot has no file at all, so it gets written to a staging
// folder first and the command references that.

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	// Enough rows to recognise a file, not enough to flood the UI.
	previewRows = 20
	previewCols = 12
	// Lines and bytes for a text preview.
	previewLines    = 40
	previewTextCap  = 16 << 10
	previewCellCap  = 200
	maxInlineImage  = 4 << 20  // beyond this, show metadata instead of a thumbnail
	maxAttachmentMB = 64 << 20 // refuse to stage a pasted blob larger than this
)

// FilePreview is what the command bar shows about one attached file.
type FilePreview struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Ext      string `json:"ext"`
	Size     int64  `json:"size"`
	SizeText string `json:"sizeText"`
	Modified string `json:"modified"`
	IsDir    bool   `json:"isDir"`

	// Kind drives which preview the UI renders: image, table, text or other.
	Kind    string `json:"kind"`
	DataURI string `json:"dataURI,omitempty"`

	// Tables (csv, tsv, xlsx)
	Columns []string   `json:"columns,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
	Sheet   string     `json:"sheet,omitempty"`

	Text      string `json:"text,omitempty"`
	Note      string `json:"note,omitempty"`
	Truncated bool   `json:"truncated"`
	Error     string `json:"error,omitempty"`
}

// ---------- Windows clipboard ----------

const cfHDROP = 15

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	shell32               = syscall.NewLazyDLL("shell32.dll")
	procOpenClipboard     = user32.NewProc("OpenClipboard")
	procCloseClipboard    = user32.NewProc("CloseClipboard")
	procGetClipboardData  = user32.NewProc("GetClipboardData")
	procIsFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procDragQueryFileW    = shell32.NewProc("DragQueryFileW")
)

// ClipboardFilePaths returns the absolute paths of files copied in Explorer, or
// an empty slice when the clipboard holds something else. The frontend calls
// this first on paste and only falls back to the pasted bytes if it comes back
// empty, so that a copied file is referenced where it actually lives rather
// than duplicated into the staging folder.
func (a *App) ClipboardFilePaths() []string {
	// Another process can hold the clipboard for a moment; it is not worth
	// failing a paste over.
	var opened bool
	for i := range 5 {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
			break
		}
		time.Sleep(time.Duration(10*(i+1)) * time.Millisecond)
	}
	if !opened {
		return []string{}
	}
	defer procCloseClipboard.Call()

	if avail, _, _ := procIsFormatAvailable.Call(cfHDROP); avail == 0 {
		return []string{}
	}
	hDrop, _, _ := procGetClipboardData.Call(cfHDROP)
	if hDrop == 0 {
		return []string{}
	}

	count, _, _ := procDragQueryFileW.Call(hDrop, 0xFFFFFFFF, 0, 0)
	paths := make([]string, 0, count)
	for idx := range int(count) {
		i := uintptr(idx)
		need, _, _ := procDragQueryFileW.Call(hDrop, i, 0, 0)
		if need == 0 {
			continue
		}
		buf := make([]uint16, need+1)
		procDragQueryFileW.Call(hDrop, i, uintptr(unsafe.Pointer(&buf[0])), need+1)
		if p := syscall.UTF16ToString(buf); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

// attachmentsDir holds blobs pasted from the clipboard that never had a file of
// their own, such as a screenshot. It sits under the config directory so it
// survives an upgrade along with everything else.
func attachmentsDir() string {
	dir := filepath.Join(configDir(), "attachments")
	os.MkdirAll(dir, 0755)
	return dir
}

// SavePastedBlob writes clipboard bytes to the staging folder and returns the
// path, so a pasted screenshot can be referenced by a command like any file.
// data is base64 because that survives the JS/Go boundary intact.
func (a *App) SavePastedBlob(name, data string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("could not decode the pasted data: %w", err)
	}
	if len(raw) > maxAttachmentMB {
		return "", fmt.Errorf("pasted data is %s, which is too large to stage", humanSize(int64(len(raw))))
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		ext = ".png"
	}
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if base == "" || base == "." || base == "image" {
		base = "pasted"
	}
	base = sanitizeFileName(base)

	target := filepath.Join(attachmentsDir(),
		fmt.Sprintf("%s-%s%s", base, time.Now().Format("20060102-150405"), ext))
	if err := os.WriteFile(target, raw, 0644); err != nil {
		return "", err
	}
	return target, nil
}

// sanitizeFileName strips what Windows will not accept in a file name.
func sanitizeFileName(s string) string {
	const bad = `<>:"/\|?*`
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r < 32 || strings.ContainsRune(bad, r) {
			continue
		}
		out = append(out, r)
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return "pasted"
	}
	if len(trimmed) > 60 {
		trimmed = trimmed[:60]
	}
	return trimmed
}

// ---------- preview ----------

var imageExts = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".gif": "image/gif", ".bmp": "image/bmp", ".webp": "image/webp",
	".ico": "image/x-icon", ".svg": "image/svg+xml", ".avif": "image/avif",
}

// PreviewFile describes a file well enough to confirm it is the intended one.
// Errors are returned inside the struct rather than as a Go error: a preview
// that cannot be produced should still show the name and size, not collapse
// the whole attachment.
func (a *App) PreviewFile(path string) FilePreview {
	p := FilePreview{Path: path, Name: filepath.Base(path), Kind: "other"}
	p.Ext = strings.ToLower(filepath.Ext(path))

	info, err := os.Stat(path)
	if err != nil {
		p.Error = "Cannot read this file"
		if os.IsNotExist(err) {
			p.Error = "This file no longer exists"
		}
		return p
	}
	p.Size = info.Size()
	p.SizeText = humanSize(info.Size())
	p.Modified = info.ModTime().Format("2 Jan 2006 15:04")

	if info.IsDir() {
		p.IsDir = true
		p.Kind = "dir"
		if entries, err := os.ReadDir(path); err == nil {
			p.Note = fmt.Sprintf("%d items", len(entries))
		}
		return p
	}

	switch {
	case imageExts[p.Ext] != "":
		p.Kind = "image"
		if info.Size() > maxInlineImage {
			p.Note = "Too large to show a thumbnail"
			return p
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			p.Error = "Cannot read this image"
			return p
		}
		p.DataURI = "data:" + imageExts[p.Ext] + ";base64," +
			base64.StdEncoding.EncodeToString(raw)

	case p.Ext == ".csv" || p.Ext == ".tsv":
		p.Kind = "table"
		sep := ','
		if p.Ext == ".tsv" {
			sep = '\t'
		}
		if err := readDelimited(path, sep, &p); err != nil {
			p.Error = err.Error()
		}

	case p.Ext == ".xlsx" || p.Ext == ".xlsm":
		p.Kind = "table"
		if err := readXLSX(path, &p); err != nil {
			p.Error = err.Error()
		}

	case p.Ext == ".xls":
		// The old binary format needs a real parser; the name and size still
		// tell you whether you picked the right file.
		p.Note = "Legacy .xls cannot be previewed - save it as .xlsx to see inside"

	default:
		if looksTextual(path) {
			p.Kind = "text"
			if err := readTextHead(path, &p); err != nil {
				p.Error = err.Error()
			}
		}
	}
	return p
}

func humanSize(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(n)/(1024*1024*1024))
}

// looksTextual sniffs the head of a file for NUL bytes, which is a good enough
// test to decide whether showing it as text would be useful or garbage.
func looksTextual(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return true // empty file: nothing to garble
	}
	return !bytes.Contains(buf[:n], []byte{0})
}

func readTextHead(path string, p *FilePreview) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read this file")
	}
	defer f.Close()

	buf := make([]byte, previewTextCap)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return fmt.Errorf("cannot read this file")
	}
	text := strings.ReplaceAll(string(buf[:n]), "\r\n", "\n")

	lines := strings.Split(text, "\n")
	if len(lines) > previewLines {
		lines = lines[:previewLines]
		p.Truncated = true
	}
	if int64(n) < p.Size {
		p.Truncated = true
	}
	p.Text = strings.Join(lines, "\n")
	if p.Truncated {
		p.Note = fmt.Sprintf("first %d lines", len(lines))
	}
	return nil
}

func readDelimited(path string, sep rune, p *FilePreview) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read this file")
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = sep
	r.FieldsPerRecord = -1 // ragged rows are common in exports
	r.LazyQuotes = true

	var rows [][]string
	for len(rows) <= previewRows {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// A malformed row should not lose the rows already parsed.
			p.Note = "stopped early: " + err.Error()
			break
		}
		rows = append(rows, rec)
	}
	if len(rows) == 0 {
		p.Note = "This file is empty"
		return nil
	}
	if len(rows) > previewRows {
		rows = rows[:previewRows]
		p.Truncated = true
	}
	applyTable(p, rows)
	return nil
}

// applyTable normalises a ragged grid into a rectangle with a header row, so
// the UI can render it without special cases.
func applyTable(p *FilePreview, rows [][]string) {
	width := 0
	for _, r := range rows {
		if len(r) > width {
			width = len(r)
		}
	}
	if width > previewCols {
		width = previewCols
		p.Truncated = true
	}

	norm := func(r []string) []string {
		out := make([]string, width)
		for i := 0; i < width; i++ {
			if i < len(r) {
				out[i] = clampCell(r[i])
			}
		}
		return out
	}

	p.Columns = norm(rows[0])
	p.Rows = make([][]string, 0, len(rows)-1)
	for _, r := range rows[1:] {
		p.Rows = append(p.Rows, norm(r))
	}
	if p.Note == "" && p.Truncated {
		p.Note = fmt.Sprintf("first %d rows", len(p.Rows))
	}
}

func clampCell(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "\r", " "), "\n", " ")
	if len(s) > previewCellCap {
		return s[:previewCellCap] + "…"
	}
	return s
}

// ---------- minimal xlsx reader ----------
//
// Only enough of the format to preview the first rows of the first sheet, which
// avoids pulling in a spreadsheet library for a thumbnail. Dates are the one
// part worth real effort: a spreadsheet of serial numbers tells you nothing
// about whether you grabbed the right file.

func readXLSX(path string, p *FilePreview) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("this does not look like a valid .xlsx file")
	}
	defer zr.Close()

	files := map[string]*zip.File{}
	var sheetNames []string
	for _, f := range zr.File {
		files[f.Name] = f
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			sheetNames = append(sheetNames, f.Name)
		}
	}
	if len(sheetNames) == 0 {
		return fmt.Errorf("this workbook has no sheets")
	}
	sort.Strings(sheetNames)

	shared := readSharedStrings(files["xl/sharedStrings.xml"])
	dateStyles := readDateStyles(files["xl/styles.xml"])
	p.Sheet = firstSheetLabel(files["xl/workbook.xml"])

	rows, err := readSheetRows(files[sheetNames[0]], shared, dateStyles)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		p.Note = "The first sheet is empty"
		return nil
	}
	if len(rows) > previewRows {
		rows = rows[:previewRows]
		p.Truncated = true
	}
	applyTable(p, rows)
	return nil
}

// firstSheetLabel reports the display name of the first sheet, which is what
// the user recognises - "Sheet1" is rarely what they called it.
func firstSheetLabel(f *zip.File) string {
	if f == nil {
		return ""
	}
	rc, err := f.Open()
	if err != nil {
		return ""
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	for {
		tok, err := dec.Token()
		if err != nil {
			return ""
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "sheet" {
			for _, attr := range se.Attr {
				if attr.Name.Local == "name" {
					return attr.Value
				}
			}
			return ""
		}
	}
}

func readSharedStrings(f *zip.File) []string {
	if f == nil {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()

	var out []string
	dec := xml.NewDecoder(rc)
	var cur strings.Builder
	inItem, inText := false, false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				inItem, inText = true, false
				cur.Reset()
			case "t":
				inText = true
			}
		case xml.CharData:
			if inItem && inText {
				cur.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "t":
				inText = false
			case "si":
				// A styled cell is split across several <r><t> runs, so the
				// pieces are concatenated rather than taken one at a time.
				out = append(out, cur.String())
				inItem = false
			}
		}
	}
	return out
}

// readDateStyles maps a cell style index to true when that style formats its
// number as a date or time.
func readDateStyles(f *zip.File) map[int]bool {
	styles := map[int]bool{}
	if f == nil {
		return styles
	}
	rc, err := f.Open()
	if err != nil {
		return styles
	}
	defer rc.Close()

	// Built-in formats that are dates or times.
	builtinDate := map[int]bool{
		14: true, 15: true, 16: true, 17: true, 18: true, 19: true, 20: true,
		21: true, 22: true, 45: true, 46: true, 47: true,
	}
	custom := map[int]bool{}

	dec := xml.NewDecoder(rc)
	inCellXfs, xfIndex := false, 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "numFmt":
				var id int
				var code string
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "numFmtId":
						id, _ = strconv.Atoi(a.Value)
					case "formatCode":
						code = a.Value
					}
				}
				if looksLikeDateFormat(code) {
					custom[id] = true
				}
			case "cellXfs":
				inCellXfs = true
			case "xf":
				if inCellXfs {
					for _, a := range t.Attr {
						if a.Name.Local == "numFmtId" {
							id, _ := strconv.Atoi(a.Value)
							if builtinDate[id] || custom[id] {
								styles[xfIndex] = true
							}
						}
					}
					xfIndex++
				}
			}
		case xml.EndElement:
			if t.Name.Local == "cellXfs" {
				inCellXfs = false
			}
		}
	}
	return styles
}

// looksLikeDateFormat spots a date or time format code while ignoring the
// literal text and colour sections Excel allows inside one.
func looksLikeDateFormat(code string) bool {
	if code == "" {
		return false
	}
	stripped := make([]rune, 0, len(code))
	inQuote, inBracket := false, false
	for _, r := range code {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == '[':
			inBracket = true
		case r == ']':
			inBracket = false
		case !inQuote && !inBracket:
			stripped = append(stripped, r)
		}
	}
	s := strings.ToLower(string(stripped))
	return strings.ContainsAny(s, "ymd") || strings.Contains(s, "h:") || strings.Contains(s, "hh")
}

func readSheetRows(f *zip.File, shared []string, dateStyles map[int]bool) ([][]string, error) {
	if f == nil {
		return nil, fmt.Errorf("this workbook has no readable sheet")
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("cannot read the first sheet")
	}
	defer rc.Close()

	var rows [][]string
	dec := xml.NewDecoder(rc)

	var row []string
	var cellRef, cellType string
	var styleIdx int
	var value strings.Builder
	inValue, inCell := false, false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // keep whatever parsed cleanly
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				row = nil
			case "c":
				inCell, cellType, cellRef, styleIdx = true, "", "", 0
				value.Reset()
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "r":
						cellRef = a.Value
					case "t":
						cellType = a.Value
					case "s":
						styleIdx, _ = strconv.Atoi(a.Value)
					}
				}
			case "v", "t":
				if inCell {
					inValue = true
				}
			}
		case xml.CharData:
			if inValue {
				value.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v", "t":
				inValue = false
			case "c":
				inCell = false
				// Place the cell at its real column so gaps stay gaps.
				if col := columnIndex(cellRef); col >= 0 {
					for len(row) <= col {
						row = append(row, "")
					}
					row[col] = renderCell(cellType, value.String(), styleIdx, shared, dateStyles)
				} else {
					row = append(row, renderCell(cellType, value.String(), styleIdx, shared, dateStyles))
				}
			case "row":
				rows = append(rows, row)
				if len(rows) > previewRows {
					return rows, nil
				}
			}
		}
	}
	return rows, nil
}

func renderCell(cellType, raw string, styleIdx int, shared []string, dateStyles map[int]bool) string {
	switch cellType {
	case "s": // index into the shared string table
		if i, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && i >= 0 && i < len(shared) {
			return shared[i]
		}
		return ""
	case "inlineStr", "str":
		return raw
	case "b":
		if strings.TrimSpace(raw) == "1" {
			return "TRUE"
		}
		return "FALSE"
	case "e":
		return raw // an Excel error such as #N/A
	}

	if raw == "" {
		return ""
	}
	if dateStyles[styleIdx] {
		if f, err := strconv.ParseFloat(raw, 64); err == nil {
			return formatExcelDate(f)
		}
	}
	return raw
}

// formatExcelDate converts a spreadsheet serial number into something readable.
func formatExcelDate(serial float64) string {
	t := excelSerialToTime(serial)
	switch {
	case serial < 1: // time of day only
		return t.Format("15:04:05")
	case serial == math.Trunc(serial): // whole day
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02 15:04")
}

func excelSerialToTime(serial float64) time.Time {
	// Excel's 1900 system counts 29 February 1900, a day that never existed,
	// so every serial after it is one ahead of the real date.
	days := serial
	if days >= 61 {
		days -= 1
	}
	whole := math.Floor(days)
	frac := days - whole
	base := time.Date(1899, 12, 31, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, int(whole)).
		Add(time.Duration(frac * 24 * float64(time.Hour)))
}

// columnIndex turns the letters of a cell reference such as "BQ14" into a
// zero-based column number, or -1 when there are none.
func columnIndex(ref string) int {
	n := 0
	seen := false
	for _, r := range ref {
		switch {
		case r >= 'A' && r <= 'Z':
			n = n*26 + int(r-'A') + 1
			seen = true
		case r >= 'a' && r <= 'z':
			n = n*26 + int(r-'a') + 1
			seen = true
		default:
			if seen {
				return n - 1
			}
			return -1
		}
	}
	if !seen {
		return -1
	}
	return n - 1
}
