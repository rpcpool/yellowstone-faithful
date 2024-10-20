package main

import (
	"path/filepath"
	"testing"
)

func TestSortCarFiles(t *testing.T) {

	fixturesDir := "fixtures"

	carFiles := []string{
		filepath.Join(fixturesDir, "epoch-0-1.car"),
		filepath.Join(fixturesDir, "epoch-0-2.car"),
		filepath.Join(fixturesDir, "epoch-0-3.car"),
	}

	result, err := SortCarFiles(carFiles)

	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(result) != 3 {
		t.Fatalf("unexpected result length: %d", len(result))
	}

	expectedResults := []struct {
		name      string
		firstSlot int64
		size      int64
	}{
		{filepath.Join(fixturesDir, "epoch-0-1.car"), 0, 96932},
		{filepath.Join(fixturesDir, "epoch-0-2.car"), 10, 100027},
		{filepath.Join(fixturesDir, "epoch-0-3.car"), 20, 99487},
	}

	for i, expected := range expectedResults {
		if result[i].name != expected.name {
			t.Fatalf("unexpected name: %s", result[i].name)
		}
		if result[i].firstSlot != expected.firstSlot {
			t.Fatalf("unexpected firstSlot: %d", result[i].firstSlot)
		}
		if result[i].size != expected.size {
			t.Fatalf("unexpected size: %d", result[i].size)
		}
	}

}
