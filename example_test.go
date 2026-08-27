package dxf_test

import (
	"bytes"
	"fmt"
	"log"
	"math"

	"github.com/gallowaysoftware/dxf"
)

func Example() {
	d := dxf.New(dxf.WithUnits(dxf.Millimeters))
	d.AddLayer("CUT", dxf.WithColor(dxf.Red))
	d.Circle("CUT", dxf.Point{X: 50, Y: 25}, 4.25)

	var buf bytes.Buffer
	if _, err := d.WriteTo(&buf); err != nil {
		log.Fatal(err)
	}
	fmt.Println("bytes written:", buf.Len() > 0)
	// Output: bytes written: true
}

// A rounded rectangle is the common CAM case: four straight runs joined by
// quarter-arcs, expressed as bulges on alternate vertices.
func ExampleDrawing_Polyline() {
	d := dxf.New()
	d.AddLayer("CUT")

	w, h, r := 100.0, 60.0, 8.0
	b := dxf.BulgeQuarter
	d.Polyline("CUT", []dxf.Vertex{
		{X: r, Y: 0}, {X: w - r, Y: 0, Bulge: b},
		{X: w, Y: r}, {X: w, Y: h - r, Bulge: b},
		{X: w - r, Y: h}, {X: r, Y: h, Bulge: b},
		{X: 0, Y: h - r}, {X: 0, Y: r, Bulge: b},
	}, true)

	fmt.Println(d.Len(), "entity")
	// Output: 1 entity
}

// A bulge is tan(theta/4) for the arc's included angle, so any sweep is
// expressible, not just quarter turns.
func ExampleVertex() {
	bulge := func(deg float64) float64 { return math.Tan(deg * math.Pi / 180 / 4) }
	fmt.Printf("90 deg: %.4f\n", bulge(90))
	fmt.Printf("180 deg: %.4f\n", bulge(180))
	// Output:
	// 90 deg: 0.4142
	// 180 deg: 1.0000
}

func ExampleDrawing_Text() {
	d := dxf.New()
	d.AddLayer("ENGRAVE", dxf.WithColor(dxf.Yellow))
	d.AddTextStyle("OpenSans", "OpenSans.ttf")
	d.Text("ENGRAVE", "PART 1", dxf.Point{X: 50, Y: 30}, 12,
		dxf.WithStyle("OpenSans"),
		dxf.WithJustify(dxf.Center, dxf.Middle))
	fmt.Println(d.Len(), "entity")
	// Output: 1 entity
}

// Referencing a layer that was never added is reported rather than written out
// as a silently broken file.
func ExampleDrawing_WriteTo_undefinedLayer() {
	d := dxf.New()
	d.Circle("MISSING", dxf.Point{}, 1)
	_, err := d.WriteTo(&bytes.Buffer{})
	fmt.Println(err)
	// Output: dxf: entity 0 (CIRCLE) references undefined layer "MISSING"
}
