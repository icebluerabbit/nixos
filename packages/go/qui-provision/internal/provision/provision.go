// Package provision registers the qBittorrent instance in qui.
//
// qui keeps its instances in SQLite and has no declarative configuration for
// them, so the instance is created over the REST API at startup instead.
// That works because qui runs with authDisabled=true: it injects a synthetic
// admin and skips setup, so POST /api/instances needs no session. The call
// comes from 127.0.0.1, which has to be in authDisabledAllowedCIDRs — the
// allowlist is checked against the direct TCP peer, before RealIP.
//
// qBittorrent's WebUI has no auth for the apps subnet (authSubnetWhitelist),
// so the instance is created with empty credentials.
//
// Everything here is fail-open: a provisioning hiccup must not wedge the
// container. The one exception is a transport failure after qui has already
// answered, which means something is wrong that retrying would not fix.
package provision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Polling defaults. qui.service is Type=simple, so the unit can start before
// its listener is up; this is how long provisioning waits for it.
const (
	// DefaultAttempts is how many times the instance list is polled.
	DefaultAttempts = 60
	// DefaultInterval is how long to wait between attempts.
	DefaultInterval = 2 * time.Second
	// DefaultTimeout bounds one API call.
	DefaultTimeout = 10 * time.Second
)

// instancesPath is qui's instance collection.
const instancesPath = "/api/instances"

// Config is what the tool takes on the command line.
type Config struct {
	// APIBase is qui's own address, e.g. http://127.0.0.1:7476.
	APIBase string
	// QbtURL is the qBittorrent WebUI the instance points at. It is also
	// the identity provisioning is idempotent on.
	QbtURL string
	// Name is what the instance is called in qui's interface.
	Name string
}

// Options tune the polling. They exist so tests do not wait two minutes for
// the fail-open path; nothing sets them from the command line.
type Options struct {
	Attempts int
	Interval time.Duration
	Client   *http.Client
}

// withDefaults fills in whatever the caller left unset.
func (o Options) withDefaults() Options {
	if o.Attempts <= 0 {
		o.Attempts = DefaultAttempts
	}
	if o.Interval <= 0 {
		o.Interval = DefaultInterval
	}
	if o.Client == nil {
		o.Client = &http.Client{Timeout: DefaultTimeout}
	}
	return o
}

// instance is one entry in qui's instance list. Only the host matters here:
// it is what makes a second run a no-op.
type instance struct {
	Host string `json:"host"`
}

// Run registers the instance, reporting what it did to out.
func Run(ctx context.Context, out io.Writer, cfg Config, opts Options) error {
	opts = opts.withDefaults()

	instances, ok := waitForInstances(ctx, cfg, opts)
	if !ok {
		//nolint:errcheck // a report that cannot be written has nowhere to go
		fmt.Fprintln(out, "qui-provision: API never became ready; skipping (fail-open)")
		return nil
	}

	for _, existing := range instances {
		if existing.Host == cfg.QbtURL {
			//nolint:errcheck // a report that cannot be written has nowhere to go
			fmt.Fprintf(out, "qui-provision: instance for %s already present, nothing to do\n", cfg.QbtURL)
			return nil
		}
	}

	return create(ctx, out, cfg, opts)
}

// waitForInstances polls the real endpoint until qui answers, because a
// listener that is not up yet is the normal case rather than a fault.
func waitForInstances(ctx context.Context, cfg Config, opts Options) ([]instance, bool) {
	for attempt := range opts.Attempts {
		var instances []instance
		_, err := call(ctx, opts.Client, http.MethodGet, cfg.APIBase+instancesPath, nil, &instances)
		if err == nil {
			return instances, true
		}

		// The last attempt has nothing left to wait for.
		if attempt == opts.Attempts-1 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, false
		case <-time.After(opts.Interval):
		}
	}
	return nil, false
}

// create posts the new instance and reports the outcome to out.
//
// An HTTP error response is reported and tolerated: qui is up, it refused,
// and wedging the container over that helps nobody. A transport failure is
// returned instead, because qui answered a moment ago and a connection that
// has since broken is not something a restart loop should paper over.
func create(ctx context.Context, out io.Writer, cfg Config, opts Options) error {
	body := map[string]string{
		"name": cfg.Name,
		"host": cfg.QbtURL,
		// qBittorrent's WebUI trusts this subnet, so there is nothing to
		// authenticate with and nothing to store.
		"username": "",
		"password": "",
	}

	status, err := call(ctx, opts.Client, http.MethodPost, cfg.APIBase+instancesPath, body, nil)

	var refused *statusError
	if errors.As(err, &refused) {
		//nolint:errcheck // a report that cannot be written has nowhere to go
		fmt.Fprintf(out, "qui-provision: create failed HTTP %d: %s\n", refused.Status, refused.Body)
		return nil
	}
	if err != nil {
		return err
	}

	//nolint:errcheck // a report that cannot be written has nowhere to go
	fmt.Fprintf(out, "qui-provision: registered '%s' -> %s (HTTP %d)\n", cfg.Name, cfg.QbtURL, status)
	return nil
}

// statusError reports a response qui refused with.
type statusError struct {
	Status int
	Body   string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Status, e.Body)
}

// call performs one JSON request. A non-2xx response is a statusError so the
// caller can tell "qui said no" apart from "qui is not there".
func call(ctx context.Context, client *http.Client, method, url string, body, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return 0, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return resp.StatusCode, &statusError{Status: resp.StatusCode, Body: string(raw)}
	}

	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response: %w", err)
		}
	}
	return resp.StatusCode, nil
}
