# dxf

[![CI](https://github.com/gallowaysoftware/dxf/actions/workflows/ci.yml/badge.svg)](https://github.com/gallowaysoftware/dxf/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/gallowaysoftware/dxf.svg)](https://pkg.go.dev/github.com/gallowaysoftware/dxf)
[![Go Report Card](https://goreportcard.com/badge/github.com/gallowaysoftware/dxf)](https://goreportcard.com/report/github.com/gallowaysoftware/dxf)

Write 2D DXF files from Go, for CAM and laser/CNC workflows. No dependencies
beyond the standard library.

```go
d := dxf.New(dxf.WithUnits(dxf.Millimeters))
d.AddLayer("CUT", dxf.WithColor(dxf.Red))
d.Circle("CUT", dxf.Point{X: 50, Y: 25}, 4.25)

if err := d.Save("plate.dxf"); err != nil {
    log.Fatal(err)
}
```

```
go get github.com/gallowaysoftware/dxf
```

Requires Go 1.24 or later. CI runs the suite on 1.24, 1.26 and 1.27.

## Why

Go has no maintained DXF writer. If you generate toolpaths, nested cut sheets or
engraving layouts from code, the usual options are to shell out to Python and
`ezdxf`, or to hand-roll a DXF emitter and hope your CAM package accepts it.

The hard part is not the entities, it is the R2010 container around them: every
object needs a unique handle and a resolvable owner pointer, subclass markers
have to appear in a specific order, and a set of tables must exist whether you
use them or not. Get any of it wrong and a reader either refuses the file or,
worse, opens it with entities quietly missing.

This library gets that container right and keeps the API small.

## What it writes

| | |
|---|---|
| **Format** | AutoCAD R2010 (AC1024) |
| **Entities** | `LINE`, `CIRCLE`, `ARC`, `LWPOLYLINE` (with bulge arcs), `TEXT` |
| **Tables** | `LAYER`, `LTYPE`, `STYLE`, `APPID`, `DIMSTYLE`, `BLOCK_RECORD` |
| **Geometry** | 2D — everything is emitted at Z=0 in model space |

**Not supported:** blocks and inserts, dimensions, hatching, splines, ellipses,
`MTEXT`, paper space layouts, or reading DXF. If you need those, you need a full
DXF implementation, not this.

## Deterministic output

The same drawing always produces the same bytes. There are no embedded
timestamps and no generated UUIDs, so DXF output can be committed to a
repository and diffed like any other build artifact — regenerating a file that
did not change produces no diff at all.

That is deliberate. Most DXF writers stamp `$TDCREATE` and fresh handle UUIDs
into every file, so a rebuild dirties your working tree even when no dimension
moved.

## Bulges

`LWPOLYLINE` encodes arcs as a *bulge* on the vertex the arc leaves:
`tan(θ/4)`, where θ is the included angle. Positive sweeps counter-clockwise,
negative clockwise, zero draws a straight segment.

`BulgeQuarter` is the 90° case, which is most of what rounded corners need:

```go
w, h, r := 100.0, 60.0, 8.0
b := dxf.BulgeQuarter
d.Polyline("CUT", []dxf.Vertex{
    {X: r, Y: 0},     {X: w - r, Y: 0, Bulge: b},
    {X: w, Y: r},     {X: w, Y: h - r, Bulge: b},
    {X: w - r, Y: h}, {X: r, Y: h, Bulge: b},
    {X: 0, Y: h - r}, {X: 0, Y: r, Bulge: b},
}, true)
```

## Errors instead of broken files

`WriteTo` and `Save` validate before writing a single byte, and fail rather than
emit something a reader will mis-handle:

- an entity on a layer that was never added
- text in a style that was never defined
- a line break inside any string value, which would split a group-code pair

```go
d := dxf.New()
d.Circle("MISSING", dxf.Point{}, 1)
_, err := d.WriteTo(&buf)
// dxf: entity 0 (CIRCLE) references undefined layer "MISSING"
```

`TEXT` is single-line by definition, so a multi-line label is rejected rather
than silently flattened — it would otherwise be engraved into a real part.

## Testing

The test suite parses the generated output back into group-code pairs and
asserts on structure, rather than matching strings:

- handles are unique and every owner pointer resolves
- entities are owned by `*Model_Space`
- every mandatory table is present, sections appear in order
- each entity's group codes, subclass markers and coordinates round-trip
- a golden file pins the exact bytes, so container changes surface as a diff
- a fuzz target asserts that any input either errors or produces a file that is
  whole group-code pairs ending in `EOF`

Coverage is above 99%. The fuzz target found the line-break bug described above.
The suite runs under Go 1.24, 1.26 and 1.27.

```
go test ./...
go test -fuzz FuzzDrawingStaysWellFormed
go test -bench .
```

## License

MIT — see [LICENSE](LICENSE).
