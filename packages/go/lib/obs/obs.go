// Package obs provides the logging and metrics setup shared by every homelab
// tool.
//
// Logs go to stderr and metrics are exposed on a local Prometheus endpoint for
// the Vector sidecar to scrape. Shipping is Vector's job: there is deliberately
// no exporter, no OTLP client and no network egress here.
//
// The log format resolves itself. On a terminal the records are styled and
// coloured for a human; anywhere else — under systemd, in CI, through a pipe —
// they are JSON, which is what Vector's Loki pipeline parses without a
// decoding step. One binary, no flag to remember, correct in both places.
package obs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	charm "github.com/charmbracelet/log"
	"github.com/muesli/termenv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/term"
)

// Config describes the observability surface of a single tool. The zero value
// is usable: it picks a format to suit the terminal, logs at info level to
// stderr, and exposes no metrics endpoint.
type Config struct {
	// Service names the tool. When set it is attached to every log record.
	Service string
	// Version is attached to every log record alongside Service.
	Version string
	// Level is one of debug, info, warn, error. Empty means info.
	Level string
	// Format is auto, json or text. Empty means auto: styled records when
	// Out is a terminal, JSON otherwise. text forces the styled handler
	// even off a terminal, which is mostly useful when debugging.
	Format string
	// Color is auto, always or never. Empty means auto, which honours
	// NO_COLOR and CLICOLOR_FORCE before falling back to terminal
	// detection.
	Color string
	// Out receives log records. Nil means os.Stderr.
	Out io.Writer
	// MetricsAddr is the listen address for the Prometheus endpoint, e.g.
	// "127.0.0.1:9100". Empty disables the endpoint entirely.
	MetricsAddr string
	// MetricsPath is the endpoint path. Empty means /metrics.
	MetricsPath string
}

// Obs is the constructed observability context for a tool.
type Obs struct {
	// Logger is the root logger. Tools derive from it with slog.With.
	Logger *slog.Logger
	// Registry holds this tool's metrics. It is not the global default
	// registry, so tests can construct an Obs without cross-contamination.
	Registry *prometheus.Registry
	// Renderer styles human-facing output with the same colour profile the
	// logger resolved, so a tool's product and its logs never disagree
	// about whether the terminal supports colour.
	Renderer *lipgloss.Renderer

	cfg Config
}

// ErrBadLevel reports an unparseable Config.Level.
var ErrBadLevel = errors.New("obs: unknown log level")

// ErrBadFormat reports an unparseable Config.Format.
var ErrBadFormat = errors.New("obs: unknown log format")

// ErrBadColor reports an unparseable Config.Color.
var ErrBadColor = errors.New("obs: unknown color mode")

// New builds the logger, colour profile and metrics registry described by cfg.
func New(cfg Config) (*Obs, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	out := cfg.Out
	if out == nil {
		out = os.Stderr
	}

	color, err := resolveColor(cfg.Color, out)
	if err != nil {
		return nil, err
	}

	styled, err := resolveStyled(cfg.Format, out)
	if err != nil {
		return nil, err
	}

	profile := termenv.Ascii
	if color {
		profile = termenv.EnvColorProfile()
		// A profile detected from the environment can still come back as
		// Ascii when TERM is unset, which is the common case under
		// systemd. An explicit request for colour is honoured anyway.
		if profile == termenv.Ascii {
			profile = termenv.ANSI
		}
	}

	renderer := lipgloss.NewRenderer(out)
	renderer.SetColorProfile(profile)

	var handler slog.Handler
	if styled {
		handler = newCharmHandler(out, level, profile)
	} else {
		handler = slog.NewJSONHandler(out, &slog.HandlerOptions{Level: level})
	}

	logger := slog.New(handler)
	if attrs := identityAttrs(cfg); len(attrs) > 0 {
		logger = logger.With(attrs...)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return &Obs{
		Logger:   logger,
		Registry: registry,
		Renderer: renderer,
		cfg:      cfg,
	}, nil
}

// newCharmHandler builds the human-facing handler. Timestamps are wall-clock
// only: a terminal reader has the date from context, and journald stamps the
// JSON path itself, so a full RFC3339 prefix on every line is noise.
func newCharmHandler(out io.Writer, level slog.Level, profile termenv.Profile) slog.Handler {
	h := charm.NewWithOptions(out, charm.Options{
		Level:           charm.Level(level),
		ReportTimestamp: true,
		TimeFormat:      time.TimeOnly,
	})
	h.SetColorProfile(profile)
	return h
}

// identityAttrs returns the service/version attributes attached to every
// record, omitting whichever of the two the caller left empty.
func identityAttrs(cfg Config) []any {
	var attrs []any
	if cfg.Service != "" {
		attrs = append(attrs, slog.String("service", cfg.Service))
	}
	if cfg.Version != "" {
		attrs = append(attrs, slog.String("version", cfg.Version))
	}
	return attrs
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrBadLevel, s)
	}
}

// resolveStyled decides between the human handler and the JSON one.
func resolveStyled(format string, out io.Writer) (bool, error) {
	switch strings.ToLower(format) {
	case "", "auto":
		return isTerminal(out), nil
	case "text":
		return true, nil
	case "json":
		return false, nil
	default:
		return false, fmt.Errorf("%w: %q", ErrBadFormat, format)
	}
}

// resolveColor applies the NO_COLOR and CLICOLOR_FORCE conventions before
// falling back to terminal detection. An explicit always/never wins over both,
// because a caller that asked was more specific than the environment.
func resolveColor(mode string, out io.Writer) (bool, error) {
	switch strings.ToLower(mode) {
	case "always":
		return true, nil
	case "never":
		return false, nil
	case "", "auto":
	default:
		return false, fmt.Errorf("%w: %q", ErrBadColor, mode)
	}

	if os.Getenv("NO_COLOR") != "" {
		return false, nil
	}
	if force := os.Getenv("CLICOLOR_FORCE"); force != "" && force != "0" {
		return true, nil
	}
	return isTerminal(out), nil
}

// isTerminal reports whether out is a terminal. Anything that is not an
// *os.File — a bytes.Buffer in a test, a pipe wrapper — is not.
func isTerminal(out io.Writer) bool {
	f, ok := out.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// MetricsPath reports the path the metrics endpoint is served on.
func (o *Obs) MetricsPath() string {
	if o.cfg.MetricsPath == "" {
		return "/metrics"
	}
	return o.cfg.MetricsPath
}

// MetricsHandler serves this tool's registry in the Prometheus text format.
func (o *Obs) MetricsHandler() http.Handler {
	return promhttp.HandlerFor(o.Registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.ContinueOnError,
	})
}

// ServeMetrics runs the Prometheus endpoint until ctx is cancelled, then
// shuts it down gracefully. It returns nil immediately when Config.MetricsAddr
// is empty, so callers can invoke it unconditionally.
func (o *Obs) ServeMetrics(ctx context.Context) error {
	if o.cfg.MetricsAddr == "" {
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle(o.MetricsPath(), o.MetricsHandler())

	srv := &http.Server{
		Addr:              o.cfg.MetricsAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		o.Logger.Debug("metrics endpoint listening",
			slog.String("addr", o.cfg.MetricsAddr),
			slog.String("path", o.MetricsPath()))
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("obs: metrics endpoint: %w", err)
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			return fmt.Errorf("obs: metrics shutdown: %w", err)
		}
		return nil
	}
}
