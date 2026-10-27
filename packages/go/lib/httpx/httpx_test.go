package httpx_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"homelab/httpx"
)

type payload struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestNewValidatesBaseURL(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseURL string
		wantErr bool
	}{
		"absolute":      {baseURL: "http://example.test:8080", wantErr: false},
		"with path":     {baseURL: "http://example.test/api/v1", wantErr: false},
		"empty":         {baseURL: "", wantErr: true},
		"whitespace":    {baseURL: "   ", wantErr: true},
		"scheme only":   {baseURL: "example.test", wantErr: true},
		"missing host":  {baseURL: "http://", wantErr: true},
		"control chars": {baseURL: "http://exa\x7fmple", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := httpx.New(httpx.Config{BaseURL: tc.baseURL})
			if (err != nil) != tc.wantErr {
				t.Fatalf("New(%q) error = %v, wantErr %v", tc.baseURL, err, tc.wantErr)
			}
		})
	}
}

// A base URL carrying a path prefix must keep that prefix. Getting this wrong
// silently reroutes every request, which is why it is asserted directly.
func TestURLResolution(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		baseURL string
		path    string
		want    string
	}{
		"root base":            {baseURL: "http://h:1", path: "/v1/x", want: "http://h:1/v1/x"},
		"root base no slash":   {baseURL: "http://h:1", path: "v1/x", want: "http://h:1/v1/x"},
		"prefix base":          {baseURL: "http://h:1/api", path: "/x", want: "http://h:1/api/x"},
		"prefix base w/ slash": {baseURL: "http://h:1/api/", path: "/x", want: "http://h:1/api/x"},
		// A query has to survive as a query. Escaping the "?" into the
		// path yields a 404 that looks like a wrong endpoint rather than
		// a wrong URL, which is exactly how it was found.
		"query":          {baseURL: "http://h:1", path: "/x?a=1", want: "http://h:1/x?a=1"},
		"query escaped":  {baseURL: "http://h:1", path: "/x?a=b+c&d=%2F", want: "http://h:1/x?a=b+c&d=%2F"},
		"query repeated": {baseURL: "http://h:1", path: "/x?a=1&a=2", want: "http://h:1/x?a=1&a=2"},
		"empty query":    {baseURL: "http://h:1", path: "/x?", want: "http://h:1/x"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			c, err := httpx.New(httpx.Config{BaseURL: tc.baseURL})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if got := c.URL(tc.path); got != tc.want {
				t.Errorf("URL(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestBaseURLReported(t *testing.T) {
	t.Parallel()

	c, err := httpx.New(httpx.Config{BaseURL: "http://h:1/api"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if got, want := c.BaseURL(), "http://h:1/api"; got != want {
		t.Errorf("BaseURL() = %q, want %q", got, want)
	}
}

func TestDoDecodesJSONResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/items" {
			t.Errorf("path = %q, want /api/items", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"komga","count":7}`))
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL + "/api"})

	var got payload
	if err := c.Do(context.Background(), http.MethodGet, "/items", nil, &got); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if diff := cmp.Diff(payload{Name: "komga", Count: 7}, got); diff != "" {
		t.Errorf("decoded payload mismatch (-want +got):\n%s", diff)
	}
}

func TestDoEncodesRequestBody(t *testing.T) {
	t.Parallel()

	var got payload
	var contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		if err := decodeJSON(r, &got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL})

	want := payload{Name: "series", Count: 3}
	if err := c.Do(context.Background(), http.MethodPost, "/items", want, nil); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("request body mismatch (-want +got):\n%s", diff)
	}
	if contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}
}

func TestDoSendsConfiguredHeaders(t *testing.T) {
	t.Parallel()

	var apiKey, userAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("X-Api-Key")
		userAgent = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{
		BaseURL:   srv.URL,
		UserAgent: "homelab/test",
		Headers:   http.Header{"X-Api-Key": []string{"secret"}},
	})

	if err := c.Do(context.Background(), http.MethodGet, "/", nil, nil); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if apiKey != "secret" {
		t.Errorf("X-Api-Key = %q, want secret", apiKey)
	}
	if userAgent != "homelab/test" {
		t.Errorf("User-Agent = %q, want homelab/test", userAgent)
	}
}

// Config.Headers is copied on New, so a caller mutating its map afterwards
// cannot retroactively change what an existing client sends.
func TestHeadersAreCopied(t *testing.T) {
	t.Parallel()

	var apiKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("X-Api-Key")
	}))
	defer srv.Close()

	headers := http.Header{"X-Api-Key": []string{"original"}}
	c := newClient(t, httpx.Config{BaseURL: srv.URL, Headers: headers})
	headers.Set("X-Api-Key", "mutated")

	if err := c.Do(context.Background(), http.MethodGet, "/", nil, nil); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if apiKey != "original" {
		t.Errorf("X-Api-Key = %q, want original", apiKey)
	}
}

func TestDoReturnsStatusError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("no such library"))
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL, RetryMax: -1})

	var out payload
	err := c.Do(context.Background(), http.MethodGet, "/missing", nil, &out)

	var se *httpx.StatusError
	if !errors.As(err, &se) {
		t.Fatalf("Do() error = %v, want *httpx.StatusError", err)
	}
	if se.Code != http.StatusNotFound {
		t.Errorf("Code = %d, want %d", se.Code, http.StatusNotFound)
	}
	if string(se.Body) != "no such library" {
		t.Errorf("Body = %q, want %q", se.Body, "no such library")
	}
	if !httpx.IsNotFound(err) {
		t.Error("IsNotFound() = false, want true")
	}
	if diff := cmp.Diff(payload{}, out); diff != "" {
		t.Errorf("out was written on error (-want +got):\n%s", diff)
	}
}

func TestIsNotFoundRejectsOtherErrors(t *testing.T) {
	t.Parallel()

	tests := map[string]error{
		"nil":         nil,
		"plain error": errors.New("boom"),
		"other status": &httpx.StatusError{
			Code: http.StatusInternalServerError,
		},
	}

	for name, err := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if httpx.IsNotFound(err) {
				t.Errorf("IsNotFound(%v) = true, want false", err)
			}
		})
	}
}

func TestRetriesServerErrors(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"name":"ok","count":1}`))
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL, RetryMax: 3})

	var got payload
	if err := c.Do(context.Background(), http.MethodGet, "/", nil, &got); err != nil {
		t.Fatalf("Do() error = %v, want success after retries", err)
	}
	if n := attempts.Load(); n != 3 {
		t.Errorf("attempts = %d, want 3", n)
	}
}

// A negative RetryMax disables retries, which provisioning code relies on when
// a non-2xx is a meaningful answer rather than a transient failure.
func TestRetriesDisabled(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL, RetryMax: -1})

	if err := c.Do(context.Background(), http.MethodGet, "/", nil, nil); err == nil {
		t.Fatal("Do() error = nil, want failure")
	}
	if n := attempts.Load(); n != 1 {
		t.Errorf("attempts = %d, want 1", n)
	}
}

func TestDoHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	c := newClient(t, httpx.Config{BaseURL: srv.URL, RetryMax: -1})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := c.Do(ctx, http.MethodGet, "/", nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Do() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestDoReportsDecodeFailure(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL})

	var out payload
	if err := c.Do(context.Background(), http.MethodGet, "/", nil, &out); err == nil {
		t.Fatal("Do() error = nil, want a decode failure")
	}
}

// HTTP is what upstream SDKs are handed, so it must be a usable client rather
// than the zero value.
func TestHTTPClientIsUsable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL})

	resp, err := c.HTTP().Get(srv.URL) //nolint:noctx // short-lived test request
	if err != nil {
		t.Fatalf("HTTP().Get() error = %v", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func newClient(t *testing.T, cfg httpx.Config) *httpx.Client {
	t.Helper()

	c, err := httpx.New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close() //nolint:errcheck // read-only body
	return json.NewDecoder(r.Body).Decode(dst)
}

func TestWithHeaderDoesNotMutateReceiver(t *testing.T) {
	t.Parallel()

	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
	}))
	defer srv.Close()

	anon := newClient(t, httpx.Config{BaseURL: srv.URL})
	authed := anon.WithBearer("token123")

	if err := authed.Do(context.Background(), http.MethodGet, "/", nil, nil); err != nil {
		t.Fatalf("Do() on authed client error = %v", err)
	}
	if err := anon.Do(context.Background(), http.MethodGet, "/", nil, nil); err != nil {
		t.Fatalf("Do() on anonymous client error = %v", err)
	}

	want := []string{"Bearer token123", ""}
	if diff := cmp.Diff(want, seen); diff != "" {
		t.Errorf("Authorization headers mismatch (-want +got):\n%s", diff)
	}
}

func TestWithHeaderKeepsExistingHeaders(t *testing.T) {
	t.Parallel()

	var apiKey, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey = r.Header.Get("X-Api-Key")
		auth = r.Header.Get("Authorization")
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{
		BaseURL: srv.URL,
		Headers: http.Header{"X-Api-Key": []string{"secret"}},
	}).WithBearer("token123")

	if err := c.Do(context.Background(), http.MethodGet, "/", nil, nil); err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if apiKey != "secret" {
		t.Errorf("X-Api-Key = %q, want secret", apiKey)
	}
	if auth != "Bearer token123" {
		t.Errorf("Authorization = %q, want Bearer token123", auth)
	}
}

func TestWaitReadyReturnsOnceHealthy(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path = %q, want /health", r.URL.Path)
		}
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.WaitReady(ctx, "/health", time.Millisecond); err != nil {
		t.Fatalf("WaitReady() error = %v", err)
	}
	if n := attempts.Load(); n != 3 {
		t.Errorf("attempts = %d, want 3", n)
	}
}

// The shell loop this replaces could not be interrupted; the Go one stops when
// systemd sends SIGTERM, which is what cancelling ctx models.
func TestWaitReadyStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newClient(t, httpx.Config{BaseURL: srv.URL})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.WaitReady(ctx, "/health", time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitReady() error = %v, want context.DeadlineExceeded", err)
	}
}

// A refused connection is the normal state while a service is still binding,
// and must be treated as "not ready yet" rather than a hard failure.
func TestWaitReadyToleratesConnectionRefused(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	c := newClient(t, httpx.Config{BaseURL: url})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.WaitReady(ctx, "/health", time.Millisecond)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitReady() error = %v, want it to keep polling until the deadline", err)
	}
}
