package dxf

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// A DXF file is a flat stream of (group code, value) pairs. Parsing it back is
// the only honest way to assert on structure, so the tests do that rather than
// matching substrings.

type pair struct {
	Code int
	Val  string
}

type doc []pair

func render(t *testing.T, d *Drawing) doc {
	t.Helper()
	var buf bytes.Buffer
	n, err := d.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("WriteTo reported %d bytes, wrote %d", n, buf.Len())
	}
	return parse(t, buf.Bytes())
}

func parse(t *testing.T, b []byte) doc {
	t.Helper()
	var out doc
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		codeLine := sc.Text()
		code, err := strconv.Atoi(strings.TrimSpace(codeLine))
		if err != nil {
			t.Fatalf("group code %q is not an integer: %v", codeLine, err)
		}
		if !sc.Scan() {
			t.Fatalf("group code %d has no value: the file ends mid-pair", code)
		}
		out = append(out, pair{code, sc.Text()})
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// sections returns the section names in the order they appear.
func (d doc) sections() []string {
	var names []string
	for i, p := range d {
		if p.Code == 0 && p.Val == "SECTION" && i+1 < len(d) && d[i+1].Code == 2 {
			names = append(names, d[i+1].Val)
		}
	}
	return names
}

// section returns the pairs inside the named section, exclusive of its markers.
func (d doc) section(name string) doc {
	for i, p := range d {
		if p.Code == 0 && p.Val == "SECTION" && i+1 < len(d) && d[i+1].Code == 2 && d[i+1].Val == name {
			for j := i + 2; j < len(d); j++ {
				if d[j].Code == 0 && d[j].Val == "ENDSEC" {
					return d[i+2 : j]
				}
			}
		}
	}
	return nil
}

// records splits a run of pairs into groups starting at each code-0 marker.
func (d doc) records() []doc {
	var out []doc
	start := -1
	for i, p := range d {
		if p.Code == 0 {
			if start >= 0 {
				out = append(out, d[start:i])
			}
			start = i
		}
	}
	if start >= 0 {
		out = append(out, d[start:])
	}
	return out
}

// entities returns the records in the ENTITIES section.
func (d doc) entities() []doc { return d.section("ENTITIES").records() }

// first returns the first value for a group code, and whether it was present.
func (r doc) first(code int) (string, bool) {
	for _, p := range r {
		if p.Code == code {
			return p.Val, true
		}
	}
	return "", false
}

// all returns every value for a group code, in order.
func (r doc) all(code int) []string {
	var out []string
	for _, p := range r {
		if p.Code == code {
			out = append(out, p.Val)
		}
	}
	return out
}

func (r doc) kind() string { v, _ := r.first(0); return v }

func (r doc) float(t *testing.T, code int) float64 {
	t.Helper()
	s, ok := r.first(code)
	if !ok {
		t.Fatalf("group %d missing from %s record", code, r.kind())
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("group %d = %q is not a float: %v", code, s, err)
	}
	return f
}

func (r doc) int(t *testing.T, code int) int {
	t.Helper()
	s, ok := r.first(code)
	if !ok {
		t.Fatalf("group %d missing from %s record", code, r.kind())
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("group %d = %q is not an int: %v", code, s, err)
	}
	return n
}
