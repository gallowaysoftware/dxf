package dxf

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// Units is the drawing's insertion unit, written to the $INSUNITS header
// variable. The values are AutoCAD's.
type Units int

// Units understood by AutoCAD. Unitless is the DXF default; most CAM workflows
// want Millimeters or Inches.
const (
	Unitless    Units = 0
	Inches      Units = 1
	Feet        Units = 2
	Millimeters Units = 4
	Centimeters Units = 5
	Meters      Units = 6
)

// Point is a location in the drawing. Geometry is 2D; every entity is emitted
// at Z=0.
type Point struct{ X, Y float64 }

// Vertex is one LWPOLYLINE point. Bulge is tan(theta/4) for the arc leaving
// this vertex, where theta is the arc's included angle: zero draws a straight
// segment, positive sweeps counter-clockwise, negative clockwise.
type Vertex struct {
	X, Y  float64
	Bulge float64
}

// BulgeQuarter is the bulge value for a 90 degree arc, the common case when
// rounding a rectangular corner.
var BulgeQuarter = 0.4142135623730951 // tan(pi/8)

// Drawing is a DXF document under construction. The zero value is not usable;
// call [New].
type Drawing struct {
	units    Units
	layers   []Layer
	styles   []TextStyle
	entities []entity
}

// Layer is a drawing layer.
type Layer struct {
	Name  string
	Color ACI
}

// TextStyle names a font for TEXT entities. Font is recorded in the file; a
// viewer substitutes its own if the font is not installed.
type TextStyle struct {
	Name string
	Font string
}

// Option configures a [Drawing] at construction.
type Option func(*Drawing)

// WithUnits sets the drawing's insertion units. The default is [Millimeters].
func WithUnits(u Units) Option { return func(d *Drawing) { d.units = u } }

// New returns an empty drawing in millimetres. Layer "0" always exists.
func New(opts ...Option) *Drawing {
	d := &Drawing{units: Millimeters}
	for _, o := range opts {
		o(d)
	}
	return d
}

// LayerOption configures a layer.
type LayerOption func(*Layer)

// WithColor sets a layer's AutoCAD Color Index.
func WithColor(c ACI) LayerOption { return func(l *Layer) { l.Color = c } }

// AddLayer defines a layer. Redefining a layer replaces its settings rather
// than adding a duplicate. Layer "0" is implicit and need not be added.
func (d *Drawing) AddLayer(name string, opts ...LayerOption) {
	l := Layer{Name: name, Color: White}
	for _, o := range opts {
		o(&l)
	}
	for i := range d.layers {
		if d.layers[i].Name == name {
			d.layers[i] = l
			return
		}
	}
	d.layers = append(d.layers, l)
}

// AddTextStyle defines a text style that [Drawing.Text] can reference by name.
// Redefining a style replaces it.
func (d *Drawing) AddTextStyle(name, font string) {
	s := TextStyle{Name: name, Font: font}
	for i := range d.styles {
		if d.styles[i].Name == name {
			d.styles[i] = s
			return
		}
	}
	d.styles = append(d.styles, s)
}

// Layers returns the layers defined so far, excluding the implicit "0".
func (d *Drawing) Layers() []Layer { return append([]Layer(nil), d.layers...) }

// TextStyles returns the text styles defined so far, excluding "Standard".
func (d *Drawing) TextStyles() []TextStyle { return append([]TextStyle(nil), d.styles...) }

// Len reports how many entities the drawing holds.
func (d *Drawing) Len() int { return len(d.entities) }

// Save writes the drawing to path, truncating any existing file.
func (d *Drawing) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := d.WriteTo(f); err != nil {
		f.Close()
		return fmt.Errorf("dxf: writing %s: %w", path, err)
	}
	return f.Close()
}

// WriteTo renders the drawing. It returns an error if an entity references a
// layer or text style that was never defined, which is the usual way a
// generated drawing ends up silently wrong.
func (d *Drawing) WriteTo(w io.Writer) (int64, error) {
	if err := d.validate(); err != nil {
		return 0, err
	}
	cw := &countingWriter{w: bufio.NewWriter(w)}
	e := &emitter{w: cw, next: firstHandle}

	d.header(e)
	d.tables(e)
	d.blocks(e)
	d.entitiesSection(e)
	d.objects(e)
	e.tag(0, "EOF")

	if e.err != nil {
		return cw.n, e.err
	}
	if err := cw.w.Flush(); err != nil {
		return cw.n, err
	}
	return cw.n, nil
}

// hasLineBreak reports whether s contains a character that would terminate a
// DXF value early. Values are line-delimited, so an embedded newline splits one
// group-code pair into two and corrupts everything after it.
func hasLineBreak(s string) bool { return strings.ContainsAny(s, "\r\n") }

func (d *Drawing) validate() error {
	known := map[string]bool{"0": true}
	for _, l := range d.layers {
		if l.Name == "" {
			return fmt.Errorf("dxf: layer name must not be empty")
		}
		if hasLineBreak(l.Name) {
			return fmt.Errorf("dxf: layer name %q contains a line break", l.Name)
		}
		known[l.Name] = true
	}
	styles := map[string]bool{"": true, "Standard": true}
	for _, s := range d.styles {
		if s.Name == "" {
			return fmt.Errorf("dxf: text style name must not be empty")
		}
		if hasLineBreak(s.Name) || hasLineBreak(s.Font) {
			return fmt.Errorf("dxf: text style %q contains a line break", s.Name)
		}
		styles[s.Name] = true
	}
	for i, en := range d.entities {
		if !known[en.layer] {
			return fmt.Errorf("dxf: entity %d (%s) references undefined layer %q", i, en.kind, en.layer)
		}
		if en.kind != "TEXT" {
			continue
		}
		if !styles[en.style] {
			return fmt.Errorf("dxf: entity %d (TEXT) references undefined text style %q", i, en.style)
		}
		// TEXT is a single-line entity by definition. Rejecting is better than
		// silently rewriting a label that will be engraved into a real part.
		if hasLineBreak(en.text) {
			return fmt.Errorf("dxf: entity %d (TEXT) contains a line break; TEXT is single-line, split it into separate entities", i)
		}
	}
	return nil
}

// countingWriter tracks bytes written so WriteTo can honour io.WriterTo.
type countingWriter struct {
	w   *bufio.Writer
	n   int64
	err error
}

func (c *countingWriter) WriteString(s string) {
	if c.err != nil {
		return
	}
	n, err := c.w.WriteString(s)
	c.n += int64(n)
	c.err = err
}

// firstHandle keeps generated handles clear of the low values AutoCAD reserves.
const firstHandle = 0x100

// emitter writes group-code pairs and allocates handles.
type emitter struct {
	w    *countingWriter
	next int
	// modelSpace is the *Model_Space block record handle: every entity owner.
	modelSpace string
	err        error
}

func (e *emitter) handle() string {
	e.next++
	return strconv.FormatInt(int64(e.next), 16)
}

// peek returns the handle the next call to handle() will produce, for owner
// pointers that must be written before the object they refer to.
func (e *emitter) peek() string { return strconv.FormatInt(int64(e.next+1), 16) }

func (e *emitter) tag(code int, value string) {
	e.w.WriteString(strconv.Itoa(code))
	e.w.WriteString("\n")
	e.w.WriteString(value)
	e.w.WriteString("\n")
	if e.w.err != nil && e.err == nil {
		e.err = e.w.err
	}
}

func (e *emitter) i(code, v int) { e.tag(code, strconv.Itoa(v)) }

// f writes a float with 12 significant digits: enough to round-trip mm-scale
// geometry without printing accumulated float noise such as 60.99999999999999.
func (e *emitter) f(code int, v float64) {
	e.tag(code, strconv.FormatFloat(v, 'g', 12, 64))
}
