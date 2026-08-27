package dxf

// HAlign is a TEXT horizontal justification (DXF group 72).
type HAlign int

// Horizontal justifications.
const (
	Left   HAlign = 0
	Center HAlign = 1
	Right  HAlign = 2
)

// VAlign is a TEXT vertical justification (DXF group 73).
type VAlign int

// Vertical justifications.
const (
	Baseline VAlign = 0
	Bottom   VAlign = 1
	Middle   VAlign = 2
	Top      VAlign = 3
)

type entity struct {
	kind   string
	layer  string
	style  string
	text   string
	verts  []Vertex
	closed bool
	a, b   Point
	radius float64
	height float64
	start  float64
	end    float64
	rot    float64
	halign HAlign
	valign VAlign
}

// Line appends a LINE entity from a to b.
func (d *Drawing) Line(layer string, a, b Point) {
	d.entities = append(d.entities, entity{kind: "LINE", layer: layer, a: a, b: b})
}

// Circle appends a CIRCLE entity.
func (d *Drawing) Circle(layer string, centre Point, radius float64) {
	d.entities = append(d.entities, entity{kind: "CIRCLE", layer: layer, a: centre, radius: radius})
}

// Arc appends an ARC entity. Angles are in degrees measured counter-clockwise
// from the positive X axis, and the arc is always drawn counter-clockwise from
// start to end.
func (d *Drawing) Arc(layer string, centre Point, radius, startDeg, endDeg float64) {
	d.entities = append(d.entities, entity{
		kind: "ARC", layer: layer, a: centre, radius: radius, start: startDeg, end: endDeg,
	})
}

// Polyline appends an LWPOLYLINE. Set closed to join the last vertex back to
// the first. Per-vertex Bulge values turn segments into arcs.
func (d *Drawing) Polyline(layer string, verts []Vertex, closed bool) {
	d.entities = append(d.entities, entity{
		kind: "LWPOLYLINE", layer: layer, verts: append([]Vertex(nil), verts...), closed: closed,
	})
}

// TextOption configures a TEXT entity.
type TextOption func(*entity)

// WithStyle draws the text in a named style previously passed to
// [Drawing.AddTextStyle].
func WithStyle(name string) TextOption { return func(e *entity) { e.style = name } }

// WithJustify sets how the text is positioned relative to its insertion point.
// The default is [Left]/[Baseline].
func WithJustify(h HAlign, v VAlign) TextOption {
	return func(e *entity) { e.halign, e.valign = h, v }
}

// WithRotation rotates the text, in degrees counter-clockwise.
func WithRotation(deg float64) TextOption { return func(e *entity) { e.rot = deg } }

// Text appends a single-line TEXT entity of the given cap height.
func (d *Drawing) Text(layer, s string, at Point, height float64, opts ...TextOption) {
	e := entity{kind: "TEXT", layer: layer, text: s, a: at, height: height}
	for _, o := range opts {
		o(&e)
	}
	d.entities = append(d.entities, e)
}

// entityHead writes the AcDbEntity preamble shared by every entity.
func (e *emitter) entityHead(kind, layer string) {
	e.tag(0, kind)
	e.tag(5, e.handle())
	e.tag(330, e.modelSpace)
	e.tag(100, "AcDbEntity")
	e.tag(8, layer)
}

func (d *Drawing) entitiesSection(e *emitter) {
	e.tag(0, "SECTION")
	e.tag(2, "ENTITIES")
	for _, en := range d.entities {
		switch en.kind {
		case "LINE":
			e.entityHead(en.kind, en.layer)
			e.tag(100, "AcDbLine")
			e.f(10, en.a.X)
			e.f(20, en.a.Y)
			e.f(30, 0)
			e.f(11, en.b.X)
			e.f(21, en.b.Y)
			e.f(31, 0)
		case "CIRCLE":
			e.entityHead(en.kind, en.layer)
			e.tag(100, "AcDbCircle")
			e.f(10, en.a.X)
			e.f(20, en.a.Y)
			e.f(30, 0)
			e.f(40, en.radius)
		case "ARC":
			e.entityHead(en.kind, en.layer)
			e.tag(100, "AcDbCircle")
			e.f(10, en.a.X)
			e.f(20, en.a.Y)
			e.f(30, 0)
			e.f(40, en.radius)
			e.tag(100, "AcDbArc")
			e.f(50, en.start)
			e.f(51, en.end)
		case "LWPOLYLINE":
			e.entityHead(en.kind, en.layer)
			e.tag(100, "AcDbPolyline")
			e.i(90, len(en.verts))
			flags := 0
			if en.closed {
				flags = 1
			}
			e.i(70, flags)
			for _, v := range en.verts {
				e.f(10, v.X)
				e.f(20, v.Y)
				if v.Bulge != 0 {
					e.f(42, v.Bulge)
				}
			}
		case "TEXT":
			e.entityHead(en.kind, en.layer)
			e.tag(100, "AcDbText")
			e.f(10, en.a.X)
			e.f(20, en.a.Y)
			e.f(30, 0)
			e.f(40, en.height)
			e.tag(1, en.text)
			if en.rot != 0 {
				e.f(50, en.rot)
			}
			if en.style != "" {
				e.tag(7, en.style)
			}
			e.i(72, int(en.halign))
			// When justification is anything but Left/Baseline the real
			// insertion point is the second alignment point, so it has to
			// repeat the coordinates.
			e.f(11, en.a.X)
			e.f(21, en.a.Y)
			e.f(31, 0)
			// DXF requires the AcDbText subclass marker a second time before
			// the vertical justification code. This is not a typo.
			e.tag(100, "AcDbText")
			e.i(73, int(en.valign))
		}
	}
	e.tag(0, "ENDSEC")
}
