package dxf

// This file writes the R2010 container: the header, the mandatory symbol
// tables, the model/paper space blocks and the root object dictionary. None of
// it is optional - AutoCAD and most readers reject a file missing any of it -
// and the group-code order within each record is fixed by the spec.

func (d *Drawing) header(e *emitter) {
	e.tag(0, "SECTION")
	e.tag(2, "HEADER")
	e.tag(9, "$ACADVER")
	e.tag(1, "AC1024")
	e.tag(9, "$HANDSEED")
	e.tag(5, "FFFFF")
	e.tag(9, "$INSUNITS")
	e.i(70, int(d.units))
	e.tag(9, "$MEASUREMENT")
	e.i(70, measurement(d.units))
	e.tag(0, "ENDSEC")
}

// measurement reports the $MEASUREMENT flag: 1 for metric, 0 for imperial. It
// decides which linetype and hatch pattern file a reader reaches for.
func measurement(u Units) int {
	switch u {
	case Inches, Feet:
		return 0
	default:
		return 1
	}
}

// table opens a symbol table and returns the handle its records must own.
func (e *emitter) table(name string, count int) string {
	h := e.handle()
	e.tag(0, "TABLE")
	e.tag(2, name)
	e.tag(5, h)
	e.tag(330, "0")
	e.tag(100, "AcDbSymbolTable")
	e.i(70, count)
	return h
}

func (e *emitter) record(kind, owner string, subclasses ...string) {
	e.tag(0, kind)
	e.tag(5, e.handle())
	e.tag(330, owner)
	e.tag(100, "AcDbSymbolTableRecord")
	for _, sc := range subclasses {
		e.tag(100, sc)
	}
}

func (d *Drawing) tables(e *emitter) {
	e.tag(0, "SECTION")
	e.tag(2, "TABLES")

	// VPORT, VIEW and UCS must exist but may be empty.
	for _, t := range []string{"VPORT", "VIEW", "UCS"} {
		e.table(t, 0)
		e.tag(0, "ENDTAB")
	}

	// ByBlock and ByLayer are required to exist in R2000 and later.
	lt := e.table("LTYPE", 3)
	for _, n := range []string{"ByBlock", "ByLayer", "CONTINUOUS"} {
		e.record("LTYPE", lt, "AcDbLinetypeTableRecord")
		e.tag(2, n)
		e.i(70, 0)
		e.tag(3, "Solid line")
		e.i(72, 65)
		e.i(73, 0)
		e.f(40, 0)
	}
	e.tag(0, "ENDTAB")

	lay := e.table("LAYER", len(d.layers)+1)
	writeLayer := func(name string, color ACI) {
		e.record("LAYER", lay, "AcDbLayerTableRecord")
		e.tag(2, name)
		e.i(70, 0)
		e.i(62, int(color))
		e.tag(6, "CONTINUOUS")
		e.tag(390, "0")
	}
	writeLayer("0", White)
	for _, l := range d.layers {
		writeLayer(l.Name, l.Color)
	}
	e.tag(0, "ENDTAB")

	st := e.table("STYLE", len(d.styles)+1)
	writeStyle := func(name, font string) {
		e.record("STYLE", st, "AcDbTextStyleTableRecord")
		e.tag(2, name)
		e.i(70, 0)
		e.f(40, 0)
		e.f(41, 1)
		e.f(50, 0)
		e.i(71, 0)
		e.f(42, 2.5)
		e.tag(3, font)
		e.tag(4, "")
	}
	writeStyle("Standard", "txt")
	for _, s := range d.styles {
		writeStyle(s.Name, s.Font)
	}
	e.tag(0, "ENDTAB")

	app := e.table("APPID", 1)
	e.record("APPID", app, "AcDbRegAppTableRecord")
	e.tag(2, "ACAD")
	e.i(70, 0)
	e.tag(0, "ENDTAB")

	dim := e.table("DIMSTYLE", 1)
	e.tag(100, "AcDbDimStyleTable")
	e.i(71, 0)
	e.record("DIMSTYLE", dim, "AcDbDimStyleTableRecord")
	e.tag(2, "Standard")
	e.i(70, 0)
	e.tag(0, "ENDTAB")

	br := e.table("BLOCK_RECORD", 2)
	// Capture the model space handle before the record consumes it: every
	// entity names it as owner.
	e.modelSpace = e.peek()
	e.record("BLOCK_RECORD", br, "AcDbBlockTableRecord")
	e.tag(2, "*Model_Space")
	e.tag(340, "0")
	e.record("BLOCK_RECORD", br, "AcDbBlockTableRecord")
	e.tag(2, "*Paper_Space")
	e.tag(340, "0")
	e.tag(0, "ENDTAB")

	e.tag(0, "ENDSEC")
}

func (e *emitter) blockPair(name string) {
	e.tag(0, "BLOCK")
	e.tag(5, e.handle())
	e.tag(330, e.modelSpace)
	e.tag(100, "AcDbEntity")
	e.tag(8, "0")
	e.tag(100, "AcDbBlockBegin")
	e.tag(2, name)
	e.i(70, 0)
	e.f(10, 0)
	e.f(20, 0)
	e.f(30, 0)
	e.tag(3, name)
	e.tag(1, "")
	e.tag(0, "ENDBLK")
	e.tag(5, e.handle())
	e.tag(330, e.modelSpace)
	e.tag(100, "AcDbEntity")
	e.tag(8, "0")
	e.tag(100, "AcDbBlockEnd")
}

func (d *Drawing) blocks(e *emitter) {
	e.tag(0, "SECTION")
	e.tag(2, "BLOCKS")
	e.blockPair("*Model_Space")
	e.blockPair("*Paper_Space")
	e.tag(0, "ENDSEC")
}

func (d *Drawing) objects(e *emitter) {
	e.tag(0, "SECTION")
	e.tag(2, "OBJECTS")
	root := e.handle()
	e.tag(0, "DICTIONARY")
	e.tag(5, root)
	e.tag(330, "0")
	e.tag(100, "AcDbDictionary")
	e.i(281, 1)
	e.tag(3, "ACAD_GROUP")
	e.tag(350, e.peek())
	e.tag(0, "DICTIONARY")
	e.tag(5, e.handle())
	e.tag(330, root)
	e.tag(100, "AcDbDictionary")
	e.i(281, 1)
	e.tag(0, "ENDSEC")
}
