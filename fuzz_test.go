package dxf

import (
	"bytes"
	"math"
	"strconv"
	"strings"
	"testing"
)

// FuzzDrawingStaysWellFormed asserts the structural invariant that matters no
// matter what a caller passes: the output is always a whole number of
// group-code pairs, every code parses as an integer, and the file terminates
// with EOF. A generator feeding user-supplied labels and machine-derived
// coordinates must never be able to produce a truncated or ragged file.
func FuzzDrawingStaysWellFormed(f *testing.F) {
	f.Add("PLATE 1", 0.0, 0.0, 1.0, 0.0)
	f.Add("", 1e9, -1e9, 0.001, 0.5)
	f.Add("multi\nline", 0.1, 0.2, 0.3, -1.0)
	f.Add("unicode Ø × ° é", 1.0/3, 2.0/3, 12.0, 0.4142135623730951)

	f.Fuzz(func(t *testing.T, label string, x, y, height, bulge float64) {
		// NaN and Inf are not representable in DXF; callers are responsible for
		// finite geometry, so skip rather than assert on them.
		for _, v := range []float64{x, y, height, bulge} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Skip()
			}
		}

		d := New()
		d.AddLayer("L")
		d.Circle("L", Point{x, y}, math.Abs(height)+1)
		d.Line("L", Point{x, y}, Point{y, x})
		d.Arc("L", Point{x, y}, math.Abs(height)+1, 0, 90)
		d.Polyline("L", []Vertex{{X: x, Y: y}, {X: y, Y: x, Bulge: bulge}}, true)
		d.Text("L", label, Point{x, y}, height)

		// The invariant is a disjunction: either the drawing is rejected with a
		// clear error, or what it writes is structurally well-formed. Silently
		// emitting a corrupt file is the only unacceptable outcome.
		var buf bytes.Buffer
		if _, err := d.WriteTo(&buf); err != nil {
			if buf.Len() != 0 {
				t.Fatalf("WriteTo failed with %v but still wrote %d bytes", err, buf.Len())
			}
			return
		}

		out := buf.String()
		if !strings.HasSuffix(out, "EOF\n") {
			t.Fatal("output does not end with EOF")
		}
		lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(lines)%2 != 0 {
			t.Fatalf("got %d lines, which is not whole group-code pairs", len(lines))
		}
		for i := 0; i < len(lines); i += 2 {
			if _, err := strconv.Atoi(strings.TrimSpace(lines[i])); err != nil {
				t.Fatalf("line %d %q is not a group code", i+1, lines[i])
			}
		}
	})
}
