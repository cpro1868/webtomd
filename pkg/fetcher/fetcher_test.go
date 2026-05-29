package fetcher

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchSendsUserAgentAndReturnsBody(t *testing.T) {
	t.Parallel()

	userAgentCh := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			userAgentCh <- r.Header.Get("User-Agent")
			http.Redirect(w, r, "/final", http.StatusFound)
			return
		}

		if r.URL.Path != "/final" {
			http.NotFound(w, r)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "hello from body")
	}))
	defer server.Close()

	client := New(ClientConfig{Timeout: time.Second})
	page, err := client.Fetch(server.URL + "/redirect")
	if err != nil {
		t.Fatalf("fetch should succeed: %v", err)
	}

	if got := string(page.Body); got != "hello from body" {
		t.Fatalf("unexpected body: %q", got)
	}
	if page.URL != server.URL+"/final" {
		t.Fatalf("unexpected final URL: got %q want %q", page.URL, server.URL+"/final")
	}

	userAgent := <-userAgentCh
	if !strings.Contains(userAgent, "Mozilla/5.0") {
		t.Fatalf("expected browser-like user agent, got %q", userAgent)
	}
	if strings.Contains(strings.ToLower(userAgent), "web2md") {
		t.Fatalf("expected user agent to avoid bot marker, got %q", userAgent)
	}
}

func TestFetchRejectsNotFound(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := New(ClientConfig{Timeout: time.Second})
	_, err := client.Fetch(server.URL)
	if err == nil {
		t.Fatal("expected fetch to fail for 404 response")
	}
	if !strings.Contains(err.Error(), "无法访问该网页") {
		t.Fatalf("expected friendly error message, got %q", err.Error())
	}
}
