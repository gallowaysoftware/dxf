package dxf

import (
	"io"
	"testing"
)

// A realistic workload: the 770-hole backing plate this library was written for.
func BenchmarkHoleField(b *testing.B) {
	d := New()
	d.AddLayer("HOLES", WithColor(Green))
	for x := 0; x < 35; x++ {
		for y := 0; y < 22; y++ {
			d.Circle("HOLES", Point{float64(x) * 25, float64(y) * 25}, 4.25)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := d.WriteTo(io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTextHeavy(b *testing.B) {
	d := New()
	d.AddLayer("ENGRAVE", WithColor(Yellow))
	for i := 0; i < 500; i++ {
		d.Text("ENGRAVE", "ELEMENT", Point{float64(i), float64(i)}, 5,
			WithJustify(Center, Middle))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := d.WriteTo(io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}
