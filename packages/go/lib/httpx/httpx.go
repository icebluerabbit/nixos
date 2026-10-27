// Package httpx provides the HTTP client every homelab tool talks to service
// APIs through: bounded retries with exponential backoff, a shared user agent,
// per-client default headers and JSON request/response handling.
//
// Client.HTTP exposes the underlying *http.Client so upstream SDKs — the
// qBittorrent, *arr, Matrix and Forgejo clients — run on the same retry and
// timeout policy as the hand-written ones rather than their own defaults.
package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// Default client tuning. Overridable per Config field.
const (
	DefaultTimeout    = 30 * time.Second
	DefaultRetryMax   = 3
	DefaultRetryWait  = 500 * time.Millisecond
	DefaultRetryLimit = 10 * time.Second
	// DefaultProbeInterval is how often WaitReady re-probes.
	DefaultProbeInterval = time.Second
	// probeTimeout bounds a single readiness probe. Short on purpose: a
	// service that has not finished binding should fail fast so the next
	// probe comes round on schedule.
	probeTimeout = 5 * time.Second
	// maxErrorBody bounds how much of a failing response StatusError keeps,
	// so a service returning an HTML error page cannot blow up a log line.
	maxErrorBody = 8 << 10
)

// Config describes a client for one service endpoint.
type Config struct {
	// BaseURL is the service root, e.g. "http://10.10.30.5:8080". Request
	// paths are resolved against it. Required.
	BaseURL string
	// Timeout bounds a single attempt, not the whole retry sequence.
	// Zero means DefaultTimeout.
	Timeout time.Duration
	// RetryMax is the number of retries after the first attempt. Zero
	// means DefaultRetryMax; use a negative value to disable retries.
	RetryMax int
	// UserAgent is sent on every request when non-empty.
	UserAgent string
	// Headers are sent on every request, e.g. an API key. Copied on New.
	Headers http.Header
	// Logger receives retry diagnostics at debug level. Nil discards them.
	Logger *slog.Logger
}

// Client is a JSON-oriented HTTP client bound to one service.
type Client struct {
	base    *url.URL
	http    *http.Client
	headers http.Header
	// probe is a single-attempt client used by WaitReady. Polling through
	// the retrying client would stack its backoff on top of the poll
	// interval, so a service coming up would be noticed seconds late.
	probe *http.Client
}

// ErrNoBaseURL reports a Config with an empty BaseURL.
var ErrNoBaseURL = errors.New("httpx: BaseURL is required")

// StatusError is returned for any response outside 2xx. Callers discriminate
// with errors.As and inspect Code, or use the IsNotFound helper.
type StatusError struct {
	Code   int
	Status string
	Method string
	URL    string
	// Body holds up to maxErrorBody bytes of the response, for diagnostics.
	Body []byte
}

func (e *StatusError) Error() string {
	if len(e.Body) == 0 {
		return fmt.Sprintf("httpx: %s %s: %s", e.Method, e.URL, e.Status)
	}
	return fmt.Sprintf("httpx: %s %s: %s: %s", e.Method, e.URL, e.Status, e.Body)
}

// IsNotFound reports whether err is a StatusError carrying 404. Provisioning
// code uses it to distinguish "absent, so create it" from a real failure.
func IsNotFound(err error) bool {
	var se *StatusError
	return errors.As(err, &se) && se.Code == http.StatusNotFound
}

// New builds a client for cfg.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, ErrNoBaseURL
	}

	// A base URL without a trailing slash would make ResolveReference drop
	// its last path segment, silently rerouting every request.
	raw := cfg.BaseURL
	if !strings.HasSuffix(raw, "/") {
		raw += "/"
	}
	base, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("httpx: parse BaseURL: %w", err)
	}
	if base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("httpx: BaseURL %q is not absolute", cfg.BaseURL)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	retries := cfg.RetryMax
	switch {
	case retries == 0:
		retries = DefaultRetryMax
	case retries < 0:
		retries = 0
	}

	rc := retryablehttp.NewClient()
	rc.RetryMax = retries
	rc.RetryWaitMin = DefaultRetryWait
	rc.RetryWaitMax = DefaultRetryLimit
	rc.HTTPClient.Timeout = timeout
	rc.Logger = leveledLogger{cfg.Logger}

	headers := make(http.Header, len(cfg.Headers)+1)
	for k, vs := range cfg.Headers {
		headers[http.CanonicalHeaderKey(k)] = append([]string(nil), vs...)
	}
	if cfg.UserAgent != "" {
		headers.Set("User-Agent", cfg.UserAgent)
	}

	return &Client{
		base:    base,
		http:    rc.StandardClient(),
		headers: headers,
		probe:   &http.Client{Timeout: min(timeout, probeTimeout)},
	}, nil
}

// WithHeader returns a copy of the client that also sends key: value.
//
// Bearer tokens are the usual reason: a service is reached anonymously to log
// in, then authenticated for everything after. The receiver is unchanged, so
// the anonymous client stays usable.
func (c *Client) WithHeader(key, value string) *Client {
	headers := make(http.Header, len(c.headers)+1)
	for k, vs := range c.headers {
		headers[k] = append([]string(nil), vs...)
	}
	headers.Set(key, value)

	clone := *c
	clone.headers = headers
	return &clone
}

// WithBearer returns a copy of the client authenticated with a bearer token.
func (c *Client) WithBearer(token string) *Client {
	return c.WithHeader("Authorization", "Bearer "+token)
}

// WaitReady polls path until it answers 2xx, then returns.
//
// It is bounded only by ctx. Under systemd that is the right shape: the unit's
// TimeoutStartSec decides how long a service gets to come up, and a SIGTERM in
// the meantime stops the poll instead of leaving it spinning — which the shell
// `until curl -sf …; do sleep 1; done` this replaces could not do.
//
// interval is the gap between probes; zero means DefaultProbeInterval.
func (c *Client) WaitReady(ctx context.Context, path string, interval time.Duration) error {
	if interval <= 0 {
		interval = DefaultProbeInterval
	}

	target := c.URL(path)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := c.ready(ctx, target); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("httpx: waiting for %s: %w", target, ctx.Err())
		case <-ticker.C:
		}
	}
}

// ready performs one readiness probe.
func (c *Client) ready(ctx context.Context, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return err
	}
	for k, vs := range c.headers {
		req.Header[k] = append([]string(nil), vs...)
	}

	resp, err := c.probe.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &StatusError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Method: http.MethodGet,
			URL:    target,
		}
	}
	return nil
}

// HTTP returns the underlying client, for handing to an upstream SDK so it
// inherits this client's retry and timeout policy. The returned client does
// not apply Config.Headers; SDKs carry their own authentication.
func (c *Client) HTTP() *http.Client { return c.http }

// BaseURL reports the resolved service root.
func (c *Client) BaseURL() string { return strings.TrimSuffix(c.base.String(), "/") }

// URL resolves a request path against the base URL.
//
// A "?" splits path from query. Without this the whole string lands in
// url.URL.Path, which escapes the "?" to %3F and turns a perfectly good query
// into a nonexistent path — a 404 that reads as if the endpoint were wrong.
//
// The path half is still assigned to Path rather than RawPath, so a path
// segment carrying reserved characters is encoded exactly once. The query half
// goes to RawQuery verbatim: callers build it with url.Values.Encode, which has
// already escaped it, and re-escaping would double-encode.
func (c *Client) URL(path string) string {
	raw, query, _ := strings.Cut(path, "?")
	return c.base.ResolveReference(&url.URL{
		Path:     strings.TrimPrefix(raw, "/"),
		RawQuery: query,
	}).String()
}

// Do issues a request and decodes a JSON response.
//
// body, when non-nil, is JSON-encoded as the request payload. out, when
// non-nil, receives the decoded response; pass nil to discard it. Any non-2xx
// status is returned as a *StatusError without touching out.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("httpx: encode request body: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	target := c.URL(path)
	req, err := http.NewRequestWithContext(ctx, method, target, payload)
	if err != nil {
		return fmt.Errorf("httpx: build request: %w", err)
	}
	for k, vs := range c.headers {
		req.Header[k] = append([]string(nil), vs...)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if out != nil {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("httpx: %s %s: %w", method, target, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return &StatusError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Method: method,
			URL:    target,
			Body:   bytes.TrimSpace(snippet),
		}
	}

	if out == nil {
		// Drain so the connection returns to the pool.
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("httpx: decode %s %s: %w", method, target, err)
	}
	return nil
}

// leveledLogger adapts slog to retryablehttp's logging interface. Everything
// lands at debug: retry chatter is diagnostic, and a request that ultimately
// fails is already reported through the returned error.
type leveledLogger struct{ log *slog.Logger }

func (l leveledLogger) emit(msg string, kv []any) {
	if l.log == nil {
		return
	}
	l.log.Debug(msg, kv...)
}

func (l leveledLogger) Error(msg string, kv ...any) { l.emit(msg, kv) }
func (l leveledLogger) Warn(msg string, kv ...any)  { l.emit(msg, kv) }
func (l leveledLogger) Info(msg string, kv ...any)  { l.emit(msg, kv) }
func (l leveledLogger) Debug(msg string, kv ...any) { l.emit(msg, kv) }
