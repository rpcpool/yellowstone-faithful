package splitcarfetcher

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRemoteFileSendsUserAgent(t *testing.T) {
	defer func(prev string) { UserAgent = prev }(UserAgent)
	UserAgent = "faithful-cli/v0.0.0-test"

	var mu sync.Mutex
	var got []string
	content := []byte("0123456789abcdef")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Method+" "+r.UserAgent())
		mu.Unlock()
		http.ServeContent(w, r, "file.index", time.Time{}, bytes.NewReader(content))
	}))
	defer srv.Close()

	rac, size, err := NewRemoteHTTPFileAsIoReaderAt(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}
	buf := make([]byte, 4)
	if _, err := rac.ReadAt(buf, 8); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "89ab" {
		t.Fatalf("read %q, want %q", buf, "89ab")
	}

	mu.Lock()
	defer mu.Unlock()
	want := []string{"HEAD " + UserAgent, "GET " + UserAgent}
	if len(got) != len(want) {
		t.Fatalf("requests = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("requests = %q, want %q", got, want)
		}
	}
}

func TestUserAgentTransportKeepsExplicitUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.UserAgent()
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "custom/1.0")
	resp, err := NewHTTPClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if got != "custom/1.0" {
		t.Fatalf("User-Agent = %q, want %q", got, "custom/1.0")
	}
}
