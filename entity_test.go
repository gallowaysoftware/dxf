package dxf

import (
	"math"
	"testing"
)

// one renders a drawing containing a single entity and returns its record.
func one(t *testing.T, build func(*Drawing)) doc {
	t.Helper()
	d := New()
	d.AddLayer("L", WithColor(Green))
	d.AddTextStyle("Sty", "sty.ttf")
	build(d)
	ents := render(t, d).entities()
	if len(ents) != 1 {
		t.Fatalf("got %d entities, want 1", len(ents))
	}
	return ents[0]
}

func closeTo(t *testing.T, got, want float64, what string) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s = %v, want %v", what, got, want)
	}
}

func TestLineEntity(t *testing.T) {
	r := one(t, func(d *Drawing) { d.Line("L", Point{1.5, 2.5}, Point{30, -4}) })
	if r.kind() != "LINE" {
		t.Fatalf("kind = %s", r.kind())
	}
	if l, _ := r.first(8); l != "L" {
		t.Errorf("layer = %q, want L", l)
	}
	if sub, _ := r.first(100); sub != "AcDbEntity" {
		t.Errorf("first subclass = %q, want AcDbEntity", sub)
	}
	if subs := r.all(100); len(subs) != 2 || subs[1] != "AcDbLine" {
		t.Errorf("subclasses = %v, want [AcDbEntity AcDbLine]", subs)
	}
	closeTo(t, r.float(t, 10), 1.5, "start X")
	closeTo(t, r.float(t, 20), 2.5, "start Y")
	closeTo(t, r.float(t, 11), 30, "end X")
	closeTo(t, r.float(t, 21), -4, "end Y")
	closeTo(t, r.float(t, 30), 0, "start Z")
	closeTo(t, r.float(t, 31), 0, "end Z")
}

func TestCircleEntity(t *testing.T) {
	r := one(t, func(d *Drawing) { d.Circle("L", Point{10, 20}, 4.25) })
	if r.kind() != "CIRCLE" {
		t.Fatalf("kind = %s", r.kind())
	}
	if subs := r.all(100); len(subs) != 2 || subs[1] != "AcDbCircle" {
		t.Errorf("subclasses = %v", subs)
	}
	closeTo(t, r.float(t, 10), 10, "centre X")
	closeTo(t, r.float(t, 20), 20, "centre Y")
	closeTo(t, r.float(t, 40), 4.25, "radius")
}

// An ARC is a CIRCLE subclass plus AcDbArc: both markers must be present, in
// that order, or readers mis-parse the angles.
func TestArcEntity(t *testing.T) {
	r := one(t, func(d *Drawing) { d.Arc("L", Point{5, 6}, 7, 90, 270) })
	if r.kind() != "ARC" {
		t.Fatalf("kind = %s", r.kind())
	}
	subs := r.all(100)
	if len(subs) != 3 || subs[0] != "AcDbEntity" || subs[1] != "AcDbCircle" || subs[2] != "AcDbArc" {
		t.Fatalf("subclasses = %v, want [AcDbEntity AcDbCircle AcDbArc]", subs)
	}
	closeTo(t, r.float(t, 40), 7, "radius")
	closeTo(t, r.float(t, 50), 90, "start angle")
	closeTo(t, r.float(t, 51), 270, "end angle")
}

func TestPolylineVerticesAndCount(t *testing.T) {
	verts := []Vertex{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}}
	r := one(t, func(d *Drawing) { d.Polyline("L", verts, true) })
	if r.kind() != "LWPOLYLINE" {
		t.Fatalf("kind = %s", r.kind())
	}
	if n := r.int(t, 90); n != len(verts) {
		t.Errorf("vertex count = %d, want %d", n, len(verts))
	}
	xs, ys := r.all(10), r.all(20)
	if len(xs) != len(verts) || len(ys) != len(verts) {
		t.Fatalf("got %d X and %d Y values, want %d each", len(xs), len(ys), len(verts))
	}
}

func TestPolylineClosedFlag(t *testing.T) {
	for _, closed := range []bool{true, false} {
		r := one(t, func(d *Drawing) {
			d.Polyline("L", []Vertex{{X: 0, Y: 0}, {X: 1, Y: 1}}, closed)
		})
		want := 0
		if closed {
			want = 1
		}
		if got := r.int(t, 70); got != want {
			t.Errorf("closed=%v: flag = %d, want %d", closed, got, want)
		}
	}
}

// Bulges are written as group 42 and only for vertices that have one, so a
// straight-segment polyline carries no 42 at all.
func TestPolylineBulges(t *testing.T) {
	r := one(t, func(d *Drawing) {
		d.Polyline("L", []Vertex{
			{X: 0, Y: 0},
			{X: 10, Y: 0, Bulge: BulgeQuarter},
			{X: 20, Y: 0, Bulge: -0.5},
			{X: 30, Y: 0},
		}, false)
	})
	got := r.all(42)
	if len(got) != 2 {
		t.Fatalf("got %d bulge values %v, want 2", len(got), got)
	}
	r2 := one(t, func(d *Drawing) {
		d.Polyline("L", []Vertex{{X: 0, Y: 0}, {X: 1, Y: 0}}, false)
	})
	if b := r2.all(42); len(b) != 0 {
		t.Errorf("a straight polyline emitted bulges %v", b)
	}
}

func TestPolylineIsCopiedNotAliased(t *testing.T) {
	verts := []Vertex{{X: 0, Y: 0}, {X: 1, Y: 1}}
	d := New()
	d.AddLayer("L")
	d.Polyline("L", verts, false)
	verts[1].X = 999 // must not affect the drawing
	ents := render(t, d).entities()
	xs := ents[0].all(10)
	if xs[1] == "999" {
		t.Error("Polyline aliased the caller's slice")
	}
}

func TestTextEntity(t *testing.T) {
	r := one(t, func(d *Drawing) {
		d.Text("L", "HELLO", Point{12, 34}, 5, WithStyle("Sty"))
	})
	if r.kind() != "TEXT" {
		t.Fatalf("kind = %s", r.kind())
	}
	if s, _ := r.first(1); s != "HELLO" {
		t.Errorf("text = %q", s)
	}
	closeTo(t, r.float(t, 40), 5, "height")
	if s, _ := r.first(7); s != "Sty" {
		t.Errorf("style = %q, want Sty", s)
	}
	closeTo(t, r.float(t, 10), 12, "insert X")
	closeTo(t, r.float(t, 20), 34, "insert Y")
}

// DXF requires the AcDbText subclass marker twice in a TEXT record: once before
// the main group codes and again before the vertical justification. Readers
// reject or mis-parse the record without the second one.
func TestTextHasTheSubclassMarkerTwice(t *testing.T) {
	r := one(t, func(d *Drawing) { d.Text("L", "x", Point{0, 0}, 1) })
	subs := r.all(100)
	n := 0
	for _, s := range subs {
		if s == "AcDbText" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("AcDbText appears %d times in %v, want 2", n, subs)
	}
	if subs[len(subs)-1] != "AcDbText" {
		t.Errorf("last subclass = %q, want AcDbText immediately before group 73", subs[len(subs)-1])
	}
}

func TestTextJustification(t *testing.T) {
	for _, tc := range []struct {
		name string
		h    HAlign
		v    VAlign
	}{
		{"default", Left, Baseline},
		{"middle-centre", Center, Middle},
		{"top-right", Right, Top},
		{"bottom-left", Left, Bottom},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := one(t, func(d *Drawing) {
				d.Text("L", "x", Point{7, 8}, 1, WithJustify(tc.h, tc.v))
			})
			if got := r.int(t, 72); got != int(tc.h) {
				t.Errorf("halign = %d, want %d", got, tc.h)
			}
			if got := r.int(t, 73); got != int(tc.v) {
				t.Errorf("valign = %d, want %d", got, tc.v)
			}
			// The second alignment point must repeat the insertion point, or
			// justified text lands at the origin.
			closeTo(t, r.float(t, 11), 7, "align point X")
			closeTo(t, r.float(t, 21), 8, "align point Y")
		})
	}
}

func TestTextRotation(t *testing.T) {
	r := one(t, func(d *Drawing) {
		d.Text("L", "x", Point{0, 0}, 1, WithRotation(45))
	})
	closeTo(t, r.float(t, 50), 45, "rotation")

	plain := one(t, func(d *Drawing) { d.Text("L", "x", Point{0, 0}, 1) })
	if v := plain.all(50); len(v) != 0 {
		t.Errorf("unrotated text emitted rotation %v", v)
	}
}

func TestTextWithoutStyleOmitsGroup7(t *testing.T) {
	r := one(t, func(d *Drawing) { d.Text("L", "x", Point{0, 0}, 1) })
	if v, ok := r.first(7); ok {
		t.Errorf("style = %q, want it omitted so the reader uses Standard", v)
	}
}

// Text is written verbatim; content must survive unchanged, including spaces
// and punctuation that appear in real engraving labels.
func TestTextContentIsPreserved(t *testing.T) {
	for _, s := range []string{
		"PERIODIC TABLE OF THE ELEMENTS",
		"+0.40",
		"Frame rail, top/bottom #1  900 x 40 x 18",
		"BACKING PLATE 900 x 570",
	} {
		r := one(t, func(d *Drawing) { d.Text("L", s, Point{0, 0}, 1) })
		if got, _ := r.first(1); got != s {
			t.Errorf("text = %q, want %q", got, s)
		}
	}
}

func TestEntitiesKeepInsertionOrder(t *testing.T) {
	d := New()
	d.AddLayer("L")
	d.Circle("L", Point{0, 0}, 1)
	d.Line("L", Point{0, 0}, Point{1, 1})
	d.Circle("L", Point{2, 2}, 1)
	want := []string{"CIRCLE", "LINE", "CIRCLE"}
	ents := render(t, d).entities()
	if len(ents) != len(want) {
		t.Fatalf("got %d entities, want %d", len(ents), len(want))
	}
	for i, w := range want {
		if ents[i].kind() != w {
			t.Errorf("entity %d = %s, want %s", i, ents[i].kind(), w)
		}
	}
}

// Coordinates must round-trip at the precision CAM needs, without float noise
// leaking into the file.
func TestCoordinatePrecision(t *testing.T) {
	for _, v := range []float64{0, 0.001, 1.0 / 3.0, 1137.5, -22.5, 965.2, 1e-6} {
		r := one(t, func(d *Drawing) { d.Circle("L", Point{v, -v}, 1) })
		closeTo(t, r.float(t, 10), v, "X")
		closeTo(t, r.float(t, 20), -v, "Y")
	}
}

func TestNoScientificNotationAtCAMScale(t *testing.T) {
	// Values in the mm range must not be written as 1e+03, which some CAM
	// importers mis-read.
	r := one(t, func(d *Drawing) { d.Circle("L", Point{1000, 2500}, 1) })
	for _, code := range []int{10, 20} {
		s, _ := r.first(code)
		for _, c := range s {
			if c == 'e' || c == 'E' {
				t.Errorf("group %d = %q uses scientific notation", code, s)
			}
		}
	}
}
