package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSortCarFiles(t *testing.T) {

	fixturesDir := "fixtures"

	carFiles := []string{
		filepath.Join(fixturesDir, "epoch-0-3.car"),
		filepath.Join(fixturesDir, "epoch-0-2.car"),
		filepath.Join(fixturesDir, "epoch-0-1.car"),
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

func TestSortCarURLs(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the filename from the URL path
		filename := filepath.Base(r.URL.Path)
		fixturesDir := "fixtures"
		filePath := filepath.Join(fixturesDir, filename)

		// Open the local CAR file
		file, err := os.Open(filePath)
		if err != nil {
			t.Fatalf("failed to open fixture file %s: %v", filePath, err)
		}
		defer file.Close()

		// Get file info for Content-Length header
		fileInfo, err := file.Stat()
		if err != nil {
			t.Fatalf("failed to get file info: %v", err)
		}

		// Handle HEAD requests
		if r.Method == "HEAD" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
			return
		}

		// Handle range requests
		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			// Parse the range header
			start, end := int64(0), fileInfo.Size()
			fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			if start < 0 {
				start = 0
			}
			if end > fileInfo.Size() {
				end = fileInfo.Size()
			}

			// Seek to the start position
			_, err = file.Seek(start, 0)
			if err != nil {
				t.Fatalf("failed to seek in file: %v", err)
			}

			// Set response headers
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end-1, fileInfo.Size()))
			w.WriteHeader(http.StatusPartialContent)

			// Copy the requested range to the response
			_, err = io.Copy(w, file)
			if err != nil {
				t.Fatalf("failed to copy file content: %v", err)
			}
			return
		}

		// Handle regular GET requests
		http.ServeFile(w, r, filePath)
	}))
	defer server.Close()

	// Create URLs for our test files
	carURLs := []string{
		server.URL + "/epoch-0-3.car",
		server.URL + "/epoch-0-2.car",
		server.URL + "/epoch-0-1.car",
	}

	// Call SortCarURLs
	result, err := SortCarURLs(carURLs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the results
	if len(result) != 3 {
		t.Fatalf("unexpected result length: got %d, want 3", len(result))
	}

	expectedResults := []struct {
		name      string
		firstSlot int64
		size      int64
	}{
		{server.URL + "/epoch-0-1.car", 0, 96932},
		{server.URL + "/epoch-0-2.car", 10, 100027},
		{server.URL + "/epoch-0-3.car", 20, 99487},
	}

	for i, expected := range expectedResults {
		if result[i].name != expected.name {
			t.Errorf("unexpected name at index %d: got %s, want %s", i, result[i].name, expected.name)
		}
		if result[i].firstSlot != expected.firstSlot {
			t.Errorf("unexpected firstSlot at index %d: got %d, want %d", i, result[i].firstSlot, expected.firstSlot)
		}
		if result[i].size != expected.size {
			t.Errorf("unexpected size at index %d: got %d, want %d", i, result[i].size, expected.size)
		}
	}
}

func TestSortCarFiles_EmptyInput(t *testing.T) {
	result, err := SortCarFiles([]string{})

	if err != nil {
		t.Fatalf("unexpected error for empty input: %s", err)
	}

	if len(result) != 0 {
		t.Fatalf("expected empty result for empty input, got %d items", len(result))
	}
}

func TestSortCarFiles_NonExistentFile(t *testing.T) {
	nonExistentFile := filepath.Join("fixtures", "non-existent.car")
	_, err := SortCarFiles([]string{nonExistentFile})

	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("unexpected error message: %s", err)
	}
}

func TestSortCarFiles_InvalidCAR(t *testing.T) {
	invalidCarFile := filepath.Join("fixtures", "invalid.car")

	// Create an invalid CAR file for testing
	err := os.WriteFile(invalidCarFile, []byte("invalid car content"), 0644)
	if err != nil {
		t.Fatalf("failed to create invalid CAR file: %s", err)
	}
	defer os.Remove(invalidCarFile)

	_, err = SortCarFiles([]string{invalidCarFile})

	if err == nil {
		t.Fatal("expected error for invalid CAR file, got nil")
	}

	if !strings.Contains(err.Error(), "failed to create CarReader") {
		t.Fatalf("unexpected error message: %s", err)
	}
}

func TestSortCarURLs_EmptyInput(t *testing.T) {
	result, err := SortCarURLs([]string{})

	if err != nil {
		t.Fatalf("unexpected error for empty input: %s", err)
	}

	if len(result) != 0 {
		t.Fatalf("expected empty result for empty input, got %d items", len(result))
	}
}

func TestSortCarURLs_InvalidURL(t *testing.T) {
	invalidURL := "http://invalid.url/non-existent.car"
	_, err := SortCarURLs([]string{invalidURL})

	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}

	if !strings.Contains(err.Error(), "failed to get first slot from URL") {
		t.Fatalf("unexpected error message: %s", err)
	}
}

func TestSortCarURLs_MixedValidAndInvalidURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve a valid CAR file for testing
		fixturesDir := "fixtures"
		filePath := filepath.Join(fixturesDir, "epoch-0-1.car")
		http.ServeFile(w, r, filePath)
	}))
	defer server.Close()

	validURL := server.URL + "/valid.car"
	invalidURL := "http://invalid.url/non-existent.car"

	result, err := SortCarURLs([]string{validURL, invalidURL})

	if err == nil {
		t.Fatal("expected error for mixed valid and invalid URLs, got nil")
	}

	if !strings.Contains(err.Error(), "failed to get first slot from URL") {
		t.Fatalf("unexpected error message: %s", err)
	}

	if len(result) != 0 {
		t.Fatalf("expected empty result for error case, got %d items", len(result))
	}
}
