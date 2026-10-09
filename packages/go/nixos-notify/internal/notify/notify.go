// Package notify reports a host lifecycle event to MQTT.
//
// Home Assistant consumes the message through an mqtt trigger and owns the
// formatting and routing. Publishing to the broker rather than posting to HA
// is what fixes the old webhook race: the broker stays up across an HA
// restart, so a message sent mid-restart is still delivered, where a POST was
// silently dropped.
//
// There is no logging sidecar on the host, unlike the containers, so this
// publishes with mosquitto_pub directly.
package notify

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Events this tool knows how to report.
const (
	EventBoot   = "boot"
	EventSwitch = "switch"
	EventScrub  = "scrub"
	EventGC     = "gc"
)

// Retry budget for the broker, which is routinely still starting when the
// boot report fires.
const (
	// DefaultAttempts is how many times a publish is tried.
	DefaultAttempts = 30
	// DefaultRetryDelay is how long to wait between attempts.
	DefaultRetryDelay = 2 * time.Second
)

// DefaultStatePath records the system the last switch report was made for,
// which is what stops a plain boot reporting a switch that did not happen.
const DefaultStatePath = "/var/lib/nixos-notify/last-system"

// statePerm and stateDirPerm match the tmpfiles rule that creates the
// directory.
const (
	statePerm    = 0o644
	stateDirPerm = 0o755
)

// gcMarker identifies the journal line carrying what a collection freed.
const gcMarker = "store paths deleted"

// scrubClean is what btrfs prints when a scrub found nothing wrong.
const scrubClean = "no errors found"

// bootedSystem is the closure the host is currently running, which is what a
// pending reboot is judged against.
const bootedSystem = "/run/booted-system"

// Config is what the tool takes on the command line.
type Config struct {
	// Pub is the mosquitto_pub binary.
	Pub string
	// Host and Port address the broker.
	Host string
	Port string
	// Topic is where the message is published.
	Topic string
	// Event is one of boot, switch, scrub or gc.
	Event string
	// Arg is the switch's flake revision, or the scrub's mount point.
	Arg string
	// Arg2 is the switch's incoming system path.
	Arg2 string
}

// System is everything the reports read from outside the process. It is an
// interface so tests can report on a host that is not this one: every event
// here is a description of a machine's state, and none of it can be set up
// from a test otherwise.
type System interface {
	// Run executes a command and returns its trimmed standard output,
	// ignoring failure the way the reports do — a missing tool yields an
	// empty field rather than no report at all.
	Run(name string, args ...string) string
	// Publish sends one message, reporting whether it succeeded.
	Publish(cfg Config, body []byte) bool
	// Nodename is the host's own name.
	Nodename() string
	// Release is the running kernel's version.
	Release() string
	// ReadFile reads a file.
	ReadFile(path string) (string, error)
	// Readlink resolves one symlink.
	Readlink(path string) (string, error)
	// RealPath resolves a path with its symlinks followed.
	RealPath(path string) (string, error)
	// ListDir lists a directory's entries.
	ListDir(path string) ([]string, error)
	// WriteState records the system a switch was reported for.
	WriteState(path, value string) error
	// Sleep waits between publish attempts.
	Sleep(d time.Duration)
}

// Options tune the retry loop and say where the state file lives.
type Options struct {
	Attempts   int
	RetryDelay time.Duration
	StatePath  string
}

// withDefaults fills in whatever the caller left unset.
func (o Options) withDefaults() Options {
	if o.Attempts <= 0 {
		o.Attempts = DefaultAttempts
	}
	if o.RetryDelay <= 0 {
		o.RetryDelay = DefaultRetryDelay
	}
	if o.StatePath == "" {
		o.StatePath = DefaultStatePath
	}
	return o
}

// Boot is what a host reports when it comes up.
type Boot struct {
	Event string `json:"event"`
	Host  string `json:"host"`
	// Kernel is the running release, which is how a reboot onto a new
	// kernel is visible without diffing anything.
	Kernel string `json:"kernel"`
	// Degraded counts the units that failed to start.
	Degraded int `json:"degraded"`
}

// Switch is what a host reports when its system closure changes.
type Switch struct {
	Event string `json:"event"`
	Host  string `json:"host"`
	// Rev is the flake revision, or "dirty" when the tree was not clean.
	Rev    string `json:"rev"`
	Gen    string `json:"gen"`
	NixOS  string `json:"nixos"`
	Kernel string `json:"kernel"`
	// Reboot reports whether the boot-critical parts changed, which is the
	// one thing a reader has to act on.
	Reboot bool   `json:"reboot"`
	Diff   string `json:"diff"`
}

// Scrub is what a host reports when a Btrfs scrub finishes.
type Scrub struct {
	Event  string `json:"event"`
	Host   string `json:"host"`
	Mount  string `json:"mount"`
	Errors bool   `json:"errors"`
	Text   string `json:"text"`
}

// GC is what a host reports when a collection finishes.
type GC struct {
	Event string `json:"event"`
	Host  string `json:"host"`
	Freed string `json:"freed"`
}

// ErrUnknownEvent reports an event nobody knows how to describe.
type ErrUnknownEvent struct{ Event string }

func (e *ErrUnknownEvent) Error() string {
	return fmt.Sprintf("unknown event %q", e.Event)
}

// Run builds the report for the configured event and publishes it.
//
// A switch that changed nothing produces no message at all, which is what
// keeps a plain boot from announcing a switch.
func Run(sys System, stderr io.Writer, cfg Config, opts Options) error {
	opts = opts.withDefaults()

	payload, err := build(sys, cfg, opts)
	if err != nil {
		return err
	}
	if payload == nil {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode payload: %w", err)
	}

	for attempt := range opts.Attempts {
		if sys.Publish(cfg, body) {
			return nil
		}
		if attempt == opts.Attempts-1 {
			break
		}
		sys.Sleep(opts.RetryDelay)
	}

	//nolint:errcheck // a report that cannot be written has nowhere to go
	fmt.Fprintln(stderr, "nixos-notify: broker unreachable, giving up")
	return nil
}

// build assembles the payload for one event, or nil when there is nothing
// worth reporting.
func build(sys System, cfg Config, opts Options) (any, error) {
	switch cfg.Event {
	case EventBoot:
		return boot(sys), nil
	case EventSwitch:
		return switchReport(sys, cfg, opts)
	case EventScrub:
		return scrub(sys, cfg), nil
	case EventGC:
		return gc(sys), nil
	default:
		return nil, &ErrUnknownEvent{Event: cfg.Event}
	}
}

// boot describes the host that just came up.
func boot(sys System) Boot {
	failed := sys.Run("systemctl", "--failed", "--no-legend", "--plain")
	degraded := 0
	if failed != "" {
		degraded = len(strings.Split(failed, "\n"))
	}
	return Boot{
		Event:    EventBoot,
		Host:     sys.Nodename(),
		Kernel:   sys.Release(),
		Degraded: degraded,
	}
}

// switchReport describes a closure change, or reports nothing when the
// closure did not actually change.
func switchReport(sys System, cfg Config, opts Options) (any, error) {
	// The activation script passes the incoming system path explicitly,
	// because /run/current-system is not swapped until later and cannot be
	// trusted here.
	newSystem := cfg.Arg2
	if newSystem == "" {
		resolved, err := sys.RealPath("/run/current-system")
		if err != nil {
			return nil, fmt.Errorf("resolve /run/current-system: %w", err)
		}
		newSystem = resolved
	}

	old, err := sys.ReadFile(opts.StatePath)
	if err != nil {
		old = "" // no state yet; this is a first run
	}
	old = strings.TrimSpace(old)

	// First run, or a plain boot onto the same closure: seed the state and
	// say nothing.
	if old == "" || old == newSystem {
		return nil, sys.WriteState(opts.StatePath, newSystem)
	}

	report := Switch{
		Event:  EventSwitch,
		Host:   sys.Nodename(),
		Rev:    cfg.Arg,
		Gen:    generation(sys),
		NixOS:  readTrimmed(sys, filepath.Join(newSystem, "nixos-version")),
		Kernel: firstEntry(sys, filepath.Join(newSystem, "kernel-modules", "lib", "modules")),
		Reboot: needsReboot(sys, newSystem),
		Diff:   sys.Run("nvd", "--color", "never", "diff", old, newSystem),
	}
	if report.Rev == "" {
		report.Rev = "dirty"
	}

	if err := sys.WriteState(opts.StatePath, newSystem); err != nil {
		return nil, err
	}
	return report, nil
}

// generation is the system profile's number.
func generation(sys System) string {
	link, err := sys.Readlink("/nix/var/nix/profiles/system")
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(link, "system-"), "-link")
}

// needsReboot reports whether the boot-critical parts of the new system
// differ from what is running.
func needsReboot(sys System, newSystem string) bool {
	for _, part := range []string{"initrd", "kernel", "kernel-modules"} {
		booted, bootedErr := sys.Readlink(filepath.Join(bootedSystem, part))
		incoming, incomingErr := sys.Readlink(filepath.Join(newSystem, part))
		if bootedErr != nil || incomingErr != nil {
			continue
		}
		if booted != incoming {
			return true
		}
	}
	return false
}

// scrub describes a finished Btrfs scrub.
func scrub(sys System, cfg Config) Scrub {
	mount := cfg.Arg
	if mount == "" {
		mount = "/"
	}
	text := sys.Run("btrfs", "scrub", "status", mount)
	return Scrub{
		Event: EventScrub,
		Host:  sys.Nodename(),
		Mount: mount,
		// Absence of the clean line is what counts as trouble: a scrub that
		// could not be read about is not a scrub that went well.
		Errors: !strings.Contains(text, scrubClean),
		Text:   text,
	}
}

// gc describes a finished collection.
func gc(sys System) GC {
	log := sys.Run("journalctl", "-u", "nix-gc.service", "-o", "cat", "-n", "50")

	freed := ""
	lines := strings.Split(log, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], gcMarker) {
			freed = lines[i]
			break
		}
	}
	return GC{Event: EventGC, Host: sys.Nodename(), Freed: freed}
}

// readTrimmed reads a file, yielding "" when it cannot be read: a missing
// field is better than no report.
func readTrimmed(sys System, path string) string {
	value, err := sys.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// firstEntry names the first entry in a directory, which for
// kernel-modules/lib/modules is the kernel the closure carries.
func firstEntry(sys System, path string) string {
	entries, err := sys.ListDir(path)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return entries[0]
}

// OSSystem is the real host.
type OSSystem struct{}

// Run executes a command and returns its trimmed standard output. Failure is
// ignored deliberately: every caller wants a field filled in if it can be,
// and a report with one blank field beats no report.
func (OSSystem) Run(name string, args ...string) string {
	cmd := exec.Command(name, args...) //nolint:gosec // the commands are this package's own
	// NO_COLOR keeps escape sequences out of a payload that ends up in a
	// chat message.
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Publish sends one message with mosquitto_pub at QoS 1, so the broker
// acknowledges it rather than dropping it on a busy socket.
func (OSSystem) Publish(cfg Config, body []byte) bool {
	//nolint:gosec // the publisher and broker come from the unit's own configuration
	cmd := exec.Command(cfg.Pub,
		"-h", cfg.Host,
		"-p", cfg.Port,
		"-q", "1",
		"-t", cfg.Topic,
		"-m", string(body),
	)
	return cmd.Run() == nil
}

// Nodename is the host's own name.
func (OSSystem) Nodename() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}

// Release is the running kernel's version.
func (OSSystem) Release() string {
	return kernelRelease()
}

// ReadFile reads a file.
func (OSSystem) ReadFile(path string) (string, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the paths are system locations this package names
	return string(raw), err
}

// Readlink resolves one symlink.
func (OSSystem) Readlink(path string) (string, error) { return os.Readlink(path) }

// RealPath resolves a path with its symlinks followed.
func (OSSystem) RealPath(path string) (string, error) { return filepath.EvalSymlinks(path) }

// ListDir lists a directory's entries.
func (OSSystem) ListDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

// WriteState records the system a switch was reported for.
func (OSSystem) WriteState(path, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), stateDirPerm); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(value), statePerm); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Sleep waits between publish attempts.
func (OSSystem) Sleep(d time.Duration) { time.Sleep(d) }
