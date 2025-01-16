package linkedlog

import "testing"

func TestBitmap(t *testing.T) {
	{
		bm := NewBitmap()
		if !bm.IsEmpty() {
			t.Fatal("expected empty bitmap")
		}
		bm.Set(0, true)
		if bm.IsEmpty() {
			t.Fatal("expected non-empty bitmap")
		}
		if !bm.Get(0) {
			t.Fatal("expected bit to be set")
		}
		bm.Set(0, false)
		if !bm.IsEmpty() {
			t.Fatal("expected empty bitmap")
		}

		bm.Set(1, true)
		if bm.IsEmpty() {
			t.Fatal("expected non-empty bitmap")
		}
		if !bm.Get(1) {
			t.Fatal("expected bit to be set")
		}
	}
}
