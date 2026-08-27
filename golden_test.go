package dxf

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden file")

// The golden file pins the exact byte output. Because rendering is
// deterministic, any change to the container - a reordered group code, a lost
// subclass marker, a shifted handle - shows up here as a diff rather than as a
// file some CAM package quietly refuses to open.
func TestGolden(t *testing.T) {
	d := New(WithUnits(Millimeters))
	d.AddLayer("OUTLINE", WithColor(Blue))
	d.AddLayer("POCKETS", WithColor(Red))
	d.AddLayer("ENGRAVE", WithColor(Yellow))
	d.AddTextStyle("OpenSans", "OpenSans.ttf")

	d.Line("OUTLINE", Point{0, 0}, Point{100, 0})
	d.Arc("OUTLINE", Point{100, 10}, 10, 270, 360)
	d.Circle("POCKETS", Point{50, 50}, 4.25)
	d.Polyline("POCKETS", []Vertex{
		{X: 10, Y: 10}, {X: 40, Y: 10, Bulge: BulgeQuarter}, {X: 50, Y: 20},
	}, true)
	d.Text("ENGRAVE", "GOLDEN", Point{50, 80}, 12,
		WithStyle("OpenSans"), WithJustify(Center, Middle))

	var buf bytes.Buffer
	if _, err := d.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("testdata", "golden.dxf")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Log("wrote", path)
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test -run TestGolden -update)", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("output differs from %s; re-run with -update if the change is intended", path)
		// Report the first differing line to make the diff actionable.
		got := bytes.Split(buf.Bytes(), []byte("\n"))
		exp := bytes.Split(want, []byte("\n"))
		for i := 0; i < len(got) && i < len(exp); i++ {
			if !bytes.Equal(got[i], exp[i]) {
				t.Errorf("first difference at line %d: got %q, want %q", i+1, got[i], exp[i])
				return
			}
		}
		t.Errorf("length differs: got %d lines, want %d", len(got), len(exp))
	}
}
