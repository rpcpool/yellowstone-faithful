package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// mockReadCloser implements io.ReadCloser for testing
type mockReadCloser struct {
	data []byte
	read bool
}

func (m *mockReadCloser) Read(p []byte) (int, error) {
	if m.read {
		return 0, io.EOF
	}
	m.read = true
	copy(p, m.data)
	return len(m.data), io.EOF
}

func (m *mockReadCloser) Close() error {
	return nil
}

func Test_openBlocksFile_local(t *testing.T) {
	tempFile, err := os.CreateTemp("", "blocksfile-*.bin")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	content := []byte("test-blocks-data")
	if _, err := tempFile.Write(content); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tempFile.Close()

	ctx := context.Background()
	data, err := openBlocksFile(ctx, tempFile.Name())
	if err != nil {
		t.Fatalf("openBlocksFile failed: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("expected %q, got %q", content, data)
	}
}

func Test_openBlocksFile_remote(t *testing.T) {
	mockData := []byte("remote-blocks-data")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(mockData)
	}))
	defer ts.Close()

	ctx := context.Background()
	data, err := openBlocksFile(ctx, ts.URL)
	if err != nil {
		t.Fatalf("openBlocksFile failed: %v", err)
	}
	if string(data) != string(mockData) {
		t.Errorf("expected %q, got %q", mockData, data)
	}
}
