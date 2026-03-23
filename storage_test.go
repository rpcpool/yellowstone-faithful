package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func uint64SliceEqual(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func uint64SliceToFileContent(slots []uint64) []byte {
	lines := make([]string, len(slots))
	for i, s := range slots {
		lines[i] = fmt.Sprintf("%d", s)
	}
	return []byte(strings.Join(lines, "\n"))
}

func Test_openBlocksFile_local(t *testing.T) {
	tempFile, err := os.CreateTemp("", "blocksfile-*.bin")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	expected := []uint64{100, 200, 300}
	content := uint64SliceToFileContent(expected)
	if _, err := tempFile.Write(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tempFile.Close()

	ctx := context.Background()
	data, err := openBlocksFile(ctx, tempFile.Name())
	if err != nil {
		t.Fatalf("openBlocksFile failed: %v", err)
	}
	if !uint64SliceEqual(data, expected) {
		t.Errorf("expected %v, got %v", expected, data)
	}
}

func Test_openBlocksFile_remote(t *testing.T) {
	expected := []uint64{400, 500, 600}
	mockData := uint64SliceToFileContent(expected)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(mockData)
	}))
	defer ts.Close()

	ctx := context.Background()
	data, err := openBlocksFile(ctx, ts.URL)
	if err != nil {
		t.Fatalf("openBlocksFile failed: %v", err)
	}
	if !uint64SliceEqual(data, expected) {
		t.Errorf("expected %v, got %v", expected, data)
	}
}
