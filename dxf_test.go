package dxf

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func sample() *Drawing {
	d := New()
	d.AddLayer("CUT", WithColor(Red))
	d.AddLayer("ENGRAVE", WithColor(Yellow))
	d.AddTextStyle("OpenSans", "OpenSans.ttf")
	d.Line("CUT", Point{0, 0}, Point{100, 50})
	d.Circle("CUT", Point{50, 25}, 4.25)
	d.Arc("CUT", Point{10, 10}, 5, 90, 180)
	d.Polyline("CUT", []Vertex{
		{X: 0, Y: 0}, {X: 10, Y: 0, Bulge: BulgeQuarter}, {X: 10, Y: 10},
	}, true)
	d.Text("ENGRAVE", "PLATE", Point{50, 60}, 12,
		WithStyle("OpenSans"), WithJustify(Center, Middle))
	return d
}

func TestSectionsAppearInOrder(t *testing.T) {
	got := render(t, sample()).sections()
	want := []string{"HEADER", "TABLES", "BLOCKS", "ENTITIES", "OBJECTS"}
	if len(got) != len(want) {
		t.Fatalf("sections = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("section %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestFileEndsWithEOF(t *testing.T) {
	d := render(t, sample())
	last := d[len(d)-1]
	if last.Code != 0 || last.Val != "EOF" {
		t.Errorf("file ends with (%d, %q), want (0, EOF)", last.Code, last.Val)
	}
}

// Every handle in the file must be unique: duplicates make a reader discard
// objects silently.
func TestHandlesAreUnique(t *testing.T) {
	seen := map[string]int{}
	for _, p := range render(t, sample()) {
		if p.Code == 5 && p.Val != "FFFFF" { // 5 is also $HANDSEED in the header
			seen[p.Val]++
		}
	}
	if len(seen) == 0 {
		t.Fatal("no handles emitted")
	}
	for h, n := range seen {
		if n > 1 {
			t.Errorf("handle %s used %d times", h, n)
		}
	}
}

// Owner pointers (group 330) must name a handle that exists, or "0" for the
// root objects.
func TestOwnerPointersResolve(t *testing.T) {
	rendered := render(t, sample())
	handles := map[string]bool{"0": true}
	for _, p := range rendered {
		if p.Code == 5 && p.Val != "FFFFF" {
			handles[p.Val] = true
		}
	}
	for _, p := range rendered {
		if p.Code == 330 && !handles[p.Val] {
			t.Errorf("owner pointer %q names no existing handle", p.Val)
		}
	}
}

// Every entity must be owned by the *Model_Space block record.
func TestEntitiesOwnedByModelSpace(t *testing.T) {
	rendered := render(t, sample())

	var modelSpace string
	for _, rec := range rendered.section("TABLES").records() {
		if rec.kind() == "BLOCK_RECORD" {
			if name, _ := rec.first(2); name == "*Model_Space" {
				modelSpace, _ = rec.first(5)
			}
		}
	}
	if modelSpace == "" {
		t.Fatal("no *Model_Space block record found")
	}
	ents := rendered.entities()
	if len(ents) == 0 {
		t.Fatal("no entities emitted")
	}
	for _, e := range ents {
		owner, ok := e.first(330)
		if !ok || owner != modelSpace {
			t.Errorf("%s owner = %q, want *Model_Space handle %q", e.kind(), owner, modelSpace)
		}
	}
}

func TestMandatoryTablesPresent(t *testing.T) {
	var names []string
	for _, rec := range render(t, sample()).section("TABLES").records() {
		if rec.kind() == "TABLE" {
			n, _ := rec.first(2)
			names = append(names, n)
		}
	}
	for _, want := range []string{"VPORT", "VIEW", "UCS", "LTYPE", "LAYER", "STYLE", "APPID", "DIMSTYLE", "BLOCK_RECORD"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("table %s missing; got %v", want, names)
		}
	}
}

func TestLayerTable(t *testing.T) {
	got := map[string]int{}
	for _, rec := range render(t, sample()).section("TABLES").records() {
		if rec.kind() == "LAYER" {
			name, _ := rec.first(2)
			col, _ := rec.first(62)
			n, _ := strconv.Atoi(col)
			got[name] = n
		}
	}
	want := map[string]int{"0": int(White), "CUT": int(Red), "ENGRAVE": int(Yellow)}
	if len(got) != len(want) {
		t.Fatalf("layers = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("layer %s colour = %d, want %d", k, got[k], v)
		}
	}
}

func TestAddLayerReplacesRatherThanDuplicates(t *testing.T) {
	d := New()
	d.AddLayer("CUT", WithColor(Red))
	d.AddLayer("CUT", WithColor(Blue))
	if n := len(d.Layers()); n != 1 {
		t.Fatalf("got %d layers, want 1", n)
	}
	if c := d.Layers()[0].Color; c != Blue {
		t.Errorf("colour = %d, want Blue (%d)", c, Blue)
	}
}

func TestAddTextStyleReplacesRatherThanDuplicates(t *testing.T) {
	d := New()
	d.AddTextStyle("S", "a.ttf")
	d.AddTextStyle("S", "b.ttf")
	if n := len(d.TextStyles()); n != 1 {
		t.Fatalf("got %d styles, want 1", n)
	}
	if f := d.TextStyles()[0].Font; f != "b.ttf" {
		t.Errorf("font = %q, want b.ttf", f)
	}
}

func TestUnitsReachTheHeader(t *testing.T) {
	for _, tc := range []struct {
		units       Units
		measurement string
	}{
		{Millimeters, "1"},
		{Inches, "0"},
		{Feet, "0"},
		{Meters, "1"},
	} {
		d := New(WithUnits(tc.units))
		hdr := render(t, d).section("HEADER")
		var insunits, meas string
		for i, p := range hdr {
			if p.Code == 9 && p.Val == "$INSUNITS" && i+1 < len(hdr) {
				insunits = hdr[i+1].Val
			}
			if p.Code == 9 && p.Val == "$MEASUREMENT" && i+1 < len(hdr) {
				meas = hdr[i+1].Val
			}
		}
		if insunits != strconv.Itoa(int(tc.units)) {
			t.Errorf("units %d: $INSUNITS = %q, want %d", tc.units, insunits, tc.units)
		}
		if meas != tc.measurement {
			t.Errorf("units %d: $MEASUREMENT = %q, want %q", tc.units, meas, tc.measurement)
		}
	}
}

func TestEmptyDrawingIsStillValid(t *testing.T) {
	d := render(t, New())
	if got := d.sections(); len(got) != 5 {
		t.Errorf("sections = %v, want all 5 even with no entities", got)
	}
	if ents := d.entities(); len(ents) != 0 {
		t.Errorf("got %d entities in an empty drawing", len(ents))
	}
}

func TestUndefinedLayerIsAnError(t *testing.T) {
	d := New()
	d.Circle("NOPE", Point{0, 0}, 1)
	_, err := d.WriteTo(io.Discard)
	if err == nil {
		t.Fatal("want an error for an entity on an undefined layer")
	}
	if !strings.Contains(err.Error(), "NOPE") {
		t.Errorf("error %q should name the offending layer", err)
	}
}

func TestUndefinedTextStyleIsAnError(t *testing.T) {
	d := New()
	d.AddLayer("L")
	d.Text("L", "hi", Point{0, 0}, 5, WithStyle("Missing"))
	_, err := d.WriteTo(io.Discard)
	if err == nil {
		t.Fatal("want an error for text in an undefined style")
	}
	if !strings.Contains(err.Error(), "Missing") {
		t.Errorf("error %q should name the offending style", err)
	}
}

func TestLayerZeroNeedsNoDefinition(t *testing.T) {
	d := New()
	d.Circle("0", Point{0, 0}, 1)
	if _, err := d.WriteTo(io.Discard); err != nil {
		t.Errorf("layer 0 is implicit, got %v", err)
	}
}

func TestEmptyLayerNameIsAnError(t *testing.T) {
	d := New()
	d.AddLayer("")
	if _, err := d.WriteTo(io.Discard); err == nil {
		t.Error("want an error for an empty layer name")
	}
}

// errWriter fails after allowing n bytes through, to exercise error paths.
type errWriter struct {
	n   int
	err error
}

func (e *errWriter) Write(p []byte) (int, error) {
	if e.n <= 0 {
		return 0, e.err
	}
	if len(p) > e.n {
		n := e.n
		e.n = 0
		return n, e.err
	}
	e.n -= len(p)
	return len(p), nil
}

func TestWriteToPropagatesWriterErrors(t *testing.T) {
	var full bytes.Buffer
	if _, err := sample().WriteTo(&full); err != nil {
		t.Fatal(err)
	}
	size := full.Len()

	sentinel := errors.New("disk full")
	// Limits must be below the full size, or the writer never fails. Cover the
	// boundary at size-1 as well as failing immediately.
	for _, limit := range []int{0, 1, size / 2, size - 1} {
		_, err := sample().WriteTo(&errWriter{n: limit, err: sentinel})
		if !errors.Is(err, sentinel) {
			t.Errorf("limit %d of %d: err = %v, want %v", limit, size, err, sentinel)
		}
	}
	// And the exact size must succeed, which is what makes the cases above
	// meaningful rather than vacuous.
	if _, err := sample().WriteTo(&errWriter{n: size, err: sentinel}); err != nil {
		t.Errorf("a writer accepting exactly %d bytes should succeed, got %v", size, err)
	}
}

func TestSaveWritesTheSameBytesAsWriteTo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.dxf")
	if err := sample().Save(path); err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := sample().WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, buf.Bytes()) {
		t.Error("Save and WriteTo produced different bytes")
	}
}

func TestSaveReportsValidationErrors(t *testing.T) {
	d := New()
	d.Circle("NOPE", Point{0, 0}, 1)
	path := filepath.Join(t.TempDir(), "out.dxf")
	if err := d.Save(path); err == nil {
		t.Fatal("want an error")
	}
}

func TestSaveReportsUnwritablePaths(t *testing.T) {
	if err := sample().Save(filepath.Join(t.TempDir(), "no-such-dir", "x.dxf")); err == nil {
		t.Error("want an error for an unwritable path")
	}
}

// Output must be reproducible: no timestamps, no generated UUIDs. This is what
// makes generated DXF safe to commit and diff.
func TestOutputIsDeterministic(t *testing.T) {
	var a, b bytes.Buffer
	if _, err := sample().WriteTo(&a); err != nil {
		t.Fatal(err)
	}
	if _, err := sample().WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Error("two renders of the same drawing differ")
	}
}

func TestEveryLineIsPartOfAPair(t *testing.T) {
	var buf bytes.Buffer
	if _, err := sample().WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines)%2 != 0 {
		t.Fatalf("file has %d lines, which cannot be whole group-code pairs", len(lines))
	}
}

func TestLenCountsEntities(t *testing.T) {
	if n := sample().Len(); n != 5 {
		t.Errorf("Len = %d, want 5", n)
	}
}

func TestBulgeQuarterIsA90DegreeArc(t *testing.T) {
	if want := math.Tan(math.Pi / 8); math.Abs(BulgeQuarter-want) > 1e-15 {
		t.Errorf("BulgeQuarter = %v, want tan(pi/8) = %v", BulgeQuarter, want)
	}
}

func TestLineBreaksInStringsAreRejected(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*Drawing)
	}{
		{"text with newline", func(d *Drawing) {
			d.AddLayer("L")
			d.Text("L", "two\nlines", Point{0, 0}, 5)
		}},
		{"text with carriage return", func(d *Drawing) {
			d.AddLayer("L")
			d.Text("L", "two\rlines", Point{0, 0}, 5)
		}},
		{"layer name with newline", func(d *Drawing) {
			d.AddLayer("bad\nname")
		}},
		{"style font with newline", func(d *Drawing) {
			d.AddTextStyle("S", "bad\nfont.ttf")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := New()
			tc.build(d)
			if _, err := d.WriteTo(io.Discard); err == nil {
				t.Error("want an error: a line break would split a group-code pair")
			}
		})
	}
}

// A failed render must not leave a partial file behind on the writer.
func TestValidationFailsBeforeWritingAnything(t *testing.T) {
	d := New()
	d.Circle("MISSING", Point{0, 0}, 1)
	var buf bytes.Buffer
	n, err := d.WriteTo(&buf)
	if err == nil {
		t.Fatal("want an error")
	}
	if n != 0 || buf.Len() != 0 {
		t.Errorf("wrote %d bytes (n=%d) before failing validation", buf.Len(), n)
	}
}
