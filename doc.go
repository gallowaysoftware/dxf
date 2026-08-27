/*
Package dxf writes 2D DXF drawings for CAM and laser/CNC workflows.

It produces AutoCAD R2010 (AC1024) files containing layers, text styles and the
entity types that 2D toolpath work actually uses: LINE, CIRCLE, ARC, LWPOLYLINE
(with bulge arcs) and TEXT. Files are written from scratch with no dependencies
beyond the standard library.

# Scope

This is a writer, not a reader, and deliberately covers a small surface:

  - Entities: LINE, CIRCLE, ARC, LWPOLYLINE, TEXT.
  - Tables: LAYER, LTYPE, STYLE, APPID, DIMSTYLE, BLOCK_RECORD.
  - Geometry is 2D. Every entity is emitted at Z=0 in model space.

There is no block/insert support, no dimensions, no hatching, no splines and no
paper space layout. If you need those, you need a full DXF implementation.

What it does guarantee is that the R2010 container is correct: every object
carries a unique handle and a resolvable owner pointer, subclass markers appear
in the required order, and the mandatory tables are present. Output is
deterministic - the same drawing always produces the same bytes, with no
embedded timestamps or generated UUIDs - so files can be committed and diffed.

# Usage

	d := dxf.New(dxf.WithUnits(dxf.Millimeters))
	d.AddLayer("CUT", dxf.WithColor(dxf.Red))
	d.Circle("CUT", dxf.Point{X: 50, Y: 50}, 4.25)
	if err := d.Save("plate.dxf"); err != nil {
		log.Fatal(err)
	}

Bulges turn polyline segments into arcs. A bulge is tan(theta/4) where theta is
the included angle, so [BulgeQuarter] gives a 90 degree arc:

	d.Polyline("CUT", []dxf.Vertex{
		{X: 10, Y: 0},
		{X: 40, Y: 0, Bulge: dxf.BulgeQuarter},
		{X: 50, Y: 10},
	}, true)
*/
package dxf
