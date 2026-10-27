// Package app is the bootstrap every homelab binary runs through.
//
// It owns the eight things each tool would otherwise repeat: flag parsing,
// layered configuration, logger and metrics construction, signal handling,
// the optional metrics endpoint, invoking the work, reporting a failure, and
// choosing an exit code. What remains in a cmd/<name>/main.go is a struct
// literal and a call.
//
// The same bootstrap serves one-shot tools and daemons. A one-shot Run returns
// when its work is done; a daemon's Run blocks until ctx is cancelled. Nothing
// branches on which kind a tool is.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sync/errgroup"

	"homelab/buildinfo"
	"homelab/config"
	"homelab/obs"
)

// Env is what a tool's Run receives. It stays deliberately small: the moment it
// becomes a service locator, every tool couples to it.
type Env struct {
	// Logger is the root logger, already carrying the tool's name and
	// version. Diagnostics and progress go here — never to Stdout.
	Logger *slog.Logger
	// Metrics is this tool's registry. Collectors registered on it are
	// exposed when --metrics.addr is set.
	Metrics *prometheus.Registry
	// Stdout is where the tool's product goes: rendered markdown, JSON
	// lines for Vector's exec source, nothing at all for most tools. It is
	// injected rather than taken from os.Stdout so that output is
	// capturable in tests without spawning a process.
	Stdout io.Writer
	// Style renders human-facing output with the colour profile the logger
	// already resolved.
	Style *lipgloss.Renderer
	// DryRun reports whether --dry-run was given. Tools that mutate
	// external state are expected to honour it.
	DryRun bool
}

// App describes one binary.
type App struct {
	// Name is the binary name. It also seeds the environment prefix for
	// tool-specific configuration and the service attribute on every log
	// record. Required.
	Name string
	// Short is cobra's one-line description.
	Short string
	// Long is cobra's extended help, shown by --help.
	Long string
	// Config points at the tool's configuration struct, pre-populated with
	// its defaults. Nil means the tool takes no configuration beyond the
	// standard flags.
	Config any
	// Flags registers tool-specific flags. Their names map to Config keys:
	// --base-url addresses base_url, --log.level addresses log.level.
	Flags func(*pflag.FlagSet)
	// Run is the tool's actual work. Required.
	Run func(ctx context.Context, env *Env) error
	// Stdout overrides where the tool's product goes. Nil means os.Stdout.
	Stdout io.Writer
	// Stderr overrides where logs go. Nil means os.Stderr.
	Stderr io.Writer
	// Args overrides the command line. Nil means os.Args[1:], which is
	// what a real binary wants; tests supply their own.
	Args []string
}

// Exit codes. 2 for a usage error matches both the shell scripts these tools
// replace and the convention most CLIs follow, so a wrapper can still tell
// "you called it wrong" apart from "it ran and failed".
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// usageError marks a failure caused by the command line rather than by the
// work. Cobra does not distinguish the two, and both silencing flags are on so
// that run errors are reported once through the logger — which would otherwise
// leave a mistyped flag producing no output at all.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// IsUsageError reports whether err came from parsing the command line.
func IsUsageError(err error) bool {
	var ue *usageError
	return errors.As(err, &ue)
}

// SharedEnvPrefix configures the standard settings — log level, format, colour,
// metrics address — for every tool at once, so a single systemd drop-in or
// shell export can turn up logging across the fleet.
const SharedEnvPrefix = "HOMELAB_"

// ErrNoRun reports an App with no work to do.
var ErrNoRun = errors.New("app: Run is required")

// ErrNoName reports an App with no name.
var ErrNoName = errors.New("app: Name is required")

// standard holds the settings every tool shares. It is loaded from its own
// layers under SharedEnvPrefix rather than being mixed into the tool's Config,
// so a tool cannot accidentally collide with --log.level.
type standard struct {
	Log struct {
		Level  string `json:"level"`
		Format string `json:"format"`
		Color  string `json:"color"`
	} `json:"log"`
	Metrics struct {
		Addr string `json:"addr"`
		Path string `json:"path"`
	} `json:"metrics"`
	DryRun bool `json:"dry_run"`
}

// Main runs the app and exits. It does not return.
//
// Panics are deliberately not recovered: a crash should reach the journal with
// its stack trace intact rather than be laundered into a one-line error.
func (a App) Main() {
	err := a.Execute(context.Background())
	switch {
	case err == nil:
		os.Exit(exitOK)
	case IsUsageError(err):
		os.Exit(exitUsage)
	default:
		os.Exit(exitError)
	}
}

// Execute runs the app under ctx and returns its error instead of exiting,
// which is what tests drive.
//
// A failure is reported once — through the logger if one was built, on stderr
// if the failure happened before that — and then returned. Callers should not
// print it again.
func (a App) Execute(ctx context.Context) error {
	if a.Name == "" {
		return ErrNoName
	}
	if a.Run == nil {
		return ErrNoRun
	}

	stderr := a.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	stdout := a.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	cmd := a.command(ctx, stdout, stderr)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	if a.Args != nil {
		cmd.SetArgs(a.Args)
	}

	if err := cmd.ExecuteContext(ctx); err != nil {
		// Run errors were already logged in serve. Usage errors have no
		// logger behind them — cobra's own reporting is silenced so that
		// a run error is not printed twice — so they are reported here.
		if IsUsageError(err) {
			_, _ = fmt.Fprintf(stderr, "%s: %v\n", a.Name, err)
			_, _ = fmt.Fprintf(stderr, "Run '%s --help' for usage.\n", a.Name)
		}
		return err
	}
	return nil
}

// command builds the cobra command. Standard flags are registered before the
// tool's own so that a tool cannot silently shadow one.
func (a App) command(_ context.Context, stdout, stderr io.Writer) *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:           a.Name,
		Short:         a.Short,
		Long:          a.Long,
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		// Arguments are rejected by default: every tool here is
		// configured by flags and environment, so a stray positional is
		// far more likely to be a mistake than an intent.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &usageError{fmt.Errorf("unexpected argument %q", args[0])}
			}
			return nil
		},
	}

	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err}
	})

	flags := cmd.Flags()
	flags.StringVar(&configPath, "config", "", "JSON configuration file")
	flags.String("log.level", "", "log level: debug, info, warn, error")
	flags.String("log.format", "", "log format: auto, json, text")
	flags.String("log.color", "", "colour: auto, always, never")
	flags.String("metrics.addr", "", "listen address for the Prometheus endpoint; empty disables it")
	flags.String("metrics.path", "", "path the Prometheus endpoint is served on")
	flags.Bool("dry-run", false, "report intended changes without making them")

	if a.Flags != nil {
		a.Flags(flags)
	}

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		return a.run(cmd, flags, configPath, stdout, stderr)
	}
	return cmd
}

// run is the body of the command: load configuration, build observability,
// then hand off to the tool.
func (a App) run(
	cmd *cobra.Command,
	flags *pflag.FlagSet,
	configPath string,
	stdout, stderr io.Writer,
) error {
	std, err := loadStandard(flags, configPath)
	if err != nil {
		// No logger yet — this is the one place a bare write is correct.
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", a.Name, err)
		return err
	}

	o, err := obs.New(obs.Config{
		Service:     a.Name,
		Version:     buildinfo.Version,
		Level:       std.Log.Level,
		Format:      std.Log.Format,
		Color:       std.Log.Color,
		Out:         stderr,
		MetricsAddr: std.Metrics.Addr,
		MetricsPath: std.Metrics.Path,
	})
	if err != nil {
		// No logger yet — this is the one place a bare write is correct.
		_, _ = fmt.Fprintf(stderr, "%s: %v\n", a.Name, err)
		return err
	}

	if a.Config != nil {
		toolOpts := config.Options{
			EnvPrefix:    toolEnvPrefix(a.Name),
			FilePath:     configPath,
			FileOptional: configPath == "",
			Flags:        flags,
		}
		if err := config.Load(a.Config, toolOpts); err != nil {
			o.Logger.Error("configuration", slog.Any("error", err))
			return err
		}
	}

	env := &Env{
		Logger:  o.Logger,
		Metrics: o.Registry,
		Stdout:  stdout,
		Style:   o.Renderer,
		DryRun:  std.DryRun,
	}

	return a.serve(cmd.Context(), o, env)
}

// serve runs the tool's work alongside the metrics endpoint, under a context
// cancelled by SIGINT or SIGTERM.
//
// The signal handler is released as soon as the first signal arrives, so a
// second one terminates the process by the default action. A shutdown that
// wedges must still be interruptible.
func (a App) serve(ctx context.Context, o *obs.Obs, env *Env) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		<-ctx.Done()
		stop()
		return nil
	})

	// ServeMetrics returns immediately when no address is configured, so a
	// one-shot tool pays nothing for this.
	group.Go(func() error { return o.ServeMetrics(ctx) })

	group.Go(func() error {
		defer stop()
		return a.Run(ctx, env)
	})

	err := group.Wait()
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		// A tool that stops because it was asked to stop succeeded.
		o.Logger.Info("interrupted")
		return nil
	default:
		o.Logger.Error("failed", slog.Any("error", err))
		return err
	}
}

// loadStandard reads the shared settings. Its file layer is the same --config
// file the tool uses, so one file can carry both.
func loadStandard(flags *pflag.FlagSet, configPath string) (standard, error) {
	var std standard
	opts := config.Options{
		EnvPrefix:    SharedEnvPrefix,
		FilePath:     configPath,
		FileOptional: configPath == "",
		Flags:        flags,
	}
	if err := config.Load(&std, opts); err != nil {
		return std, err
	}
	return std, nil
}

// toolEnvPrefix derives a tool's own environment prefix from its name:
// komga-setup reads HOMELAB_KOMGA_SETUP_*.
func toolEnvPrefix(name string) string {
	upper := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	return SharedEnvPrefix + upper + "_"
}
