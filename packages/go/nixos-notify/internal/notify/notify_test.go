package notify

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeSystem is a host described entirely by the test. Every report here is
// a description of a machine's state, so this is the only way to assert one
// without being that machine.
type fakeSystem struct {
	nodename string
	release  string

	// commands maps a joined command line to its output, and records what
	// was actually run.
	commands map[string]string
	ran      [][]string

	files    map[string]string
	links    map[string]string
	realPath map[string]string
	dirs     map[string][]string

	// state is what WriteState recorded.
	state    map[string]string
	stateErr error

	// published collects each attempted message; failUntil publishes fail
	// before one succeeds.
	published  [][]byte
	failUntil  int
	slept      []time.Duration
	publishAll bool
}

func newFakeSystem() *fakeSystem {
	return &fakeSystem{
		nodename: "server",
		release:  "6.12.1",
		commands: map[string]string{},
		files:    map[string]string{},
		links:    map[string]string{},
		realPath: map[string]string{},
		dirs:     map[string][]string{},
		state:    map[string]string{},
	}
}

func (f *fakeSystem) Run(name string, args ...string) string {
	line := strings.Join(append([]string{name}, args...), " ")
	f.ran = append(f.ran, append([]string{name}, args...))
	return f.commands[line]
}

func (f *fakeSystem) Publish(_ Config, body []byte) bool {
	f.published = append(f.published, append([]byte(nil), body...))
	if f.failUntil > 0 {
		f.failUntil--
		return false
	}
	return !f.publishAll
}

func (f *fakeSystem) Nodename() string { return f.nodename }
func (f *fakeSystem) Release() string  { return f.release }

func (f *fakeSystem) ReadFile(path string) (string, error) {
	v, ok := f.files[path]
	if !ok {
		return "", os.ErrNotExist
	}
	return v, nil
}

func (f *fakeSystem) Readlink(path string) (string, error) {
	v, ok := f.links[path]
	if !ok {
		return "", os.ErrNotExist
	}
	return v, nil
}

func (f *fakeSystem) RealPath(path string) (string, error) {
	v, ok := f.realPath[path]
	if !ok {
		return "", os.ErrNotExist
	}
	return v, nil
}

func (f *fakeSystem) ListDir(path string) ([]string, error) {
	v, ok := f.dirs[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return v, nil
}

func (f *fakeSystem) WriteState(path, value string) error {
	if f.stateErr != nil {
		return f.stateErr
	}
	f.state[path] = value
	return nil
}

func (f *fakeSystem) Sleep(d time.Duration) { f.slept = append(f.slept, d) }

// fast keeps the retry behaviour without its wall clock.
func fast() Options {
	return Options{Attempts: 3, RetryDelay: time.Millisecond, StatePath: "/state"}
}

// onlyMessage decodes the single message that was published.
func onlyMessage(t *testing.T, f *fakeSystem) map[string]any {
	t.Helper()
	if len(f.published) != 1 {
		t.Fatalf("published %d messages, want 1", len(f.published))
	}
	var payload map[string]any
	if err := json.Unmarshal(f.published[0], &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return payload
}

func TestBootReportsFailedUnits(t *testing.T) {
	f := newFakeSystem()
	f.commands["systemctl --failed --no-legend --plain"] = "a.service failed\nb.service failed"

	if err := Run(f, io.Discard, Config{Event: EventBoot}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := onlyMessage(t, f)
	if got["event"] != EventBoot || got["host"] != "server" || got["kernel"] != "6.12.1" {
		t.Errorf("payload = %v", got)
	}
	if got["degraded"] != float64(2) {
		t.Errorf("degraded = %v, want 2", got["degraded"])
	}
}

// A healthy host reports zero, not an empty string turned into one line.
func TestBootReportsZeroWhenHealthy(t *testing.T) {
	f := newFakeSystem()
	f.commands["systemctl --failed --no-legend --plain"] = ""

	if err := Run(f, io.Discard, Config{Event: EventBoot}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := onlyMessage(t, f)["degraded"]; got != float64(0) {
		t.Errorf("degraded = %v, want 0", got)
	}
}

// switchSystem sets up a host mid-activation: an old closure recorded, a new
// one passed in.
func switchSystem() *fakeSystem {
	f := newFakeSystem()
	f.files["/state"] = "/nix/store/old-system"
	f.files["/nix/store/new-system/nixos-version"] = "26.11.20260919.20b1ddd\n"
	f.dirs["/nix/store/new-system/kernel-modules/lib/modules"] = []string{"6.12.2"}
	f.links["/nix/var/nix/profiles/system"] = "system-431-link"
	f.links["/run/booted-system/initrd"] = "/nix/store/a-initrd"
	f.links["/run/booted-system/kernel"] = "/nix/store/a-kernel"
	f.links["/run/booted-system/kernel-modules"] = "/nix/store/a-modules"
	f.links["/nix/store/new-system/initrd"] = "/nix/store/a-initrd"
	f.links["/nix/store/new-system/kernel"] = "/nix/store/a-kernel"
	f.links["/nix/store/new-system/kernel-modules"] = "/nix/store/a-modules"
	f.commands["nvd --color never diff /nix/store/old-system /nix/store/new-system"] = "<<< upgraded foo"
	return f
}

func TestSwitchReportsTheClosureChange(t *testing.T) {
	f := switchSystem()
	cfg := Config{Event: EventSwitch, Arg: "abc123", Arg2: "/nix/store/new-system"}

	if err := Run(f, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := onlyMessage(t, f)
	want := map[string]any{
		"event":  EventSwitch,
		"host":   "server",
		"rev":    "abc123",
		"gen":    "431",
		"nixos":  "26.11.20260919.20b1ddd",
		"kernel": "6.12.2",
		"reboot": false,
		"diff":   "<<< upgraded foo",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %v, want %v", key, got[key], value)
		}
	}

	// The state advances, so the next activation diffs against this one.
	if f.state["/state"] != "/nix/store/new-system" {
		t.Errorf("state = %q", f.state["/state"])
	}
}

// A changed kernel is the one thing a reader has to act on.
func TestSwitchFlagsRebootWhenBootPartsChange(t *testing.T) {
	for _, part := range []string{"initrd", "kernel", "kernel-modules"} {
		t.Run(part, func(t *testing.T) {
			f := switchSystem()
			f.links["/nix/store/new-system/"+part] = "/nix/store/b-" + part

			cfg := Config{Event: EventSwitch, Arg: "abc123", Arg2: "/nix/store/new-system"}
			if err := Run(f, io.Discard, cfg, fast()); err != nil {
				t.Fatalf("Run: %v", err)
			}

			if got := onlyMessage(t, f)["reboot"]; got != true {
				t.Errorf("reboot = %v, want true", got)
			}
		})
	}
}

// A plain boot activates the same closure it is already running, and must
// not announce a switch that did not happen.
func TestSwitchSaysNothingWhenTheClosureIsUnchanged(t *testing.T) {
	f := switchSystem()
	f.files["/state"] = "/nix/store/new-system"

	cfg := Config{Event: EventSwitch, Arg: "abc123", Arg2: "/nix/store/new-system"}
	if err := Run(f, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(f.published) != 0 {
		t.Errorf("published %d messages, want none", len(f.published))
	}
	if f.state["/state"] != "/nix/store/new-system" {
		t.Errorf("state = %q, want it seeded", f.state["/state"])
	}
}

// The first run has nothing to diff against, so it seeds and stays quiet.
func TestSwitchSeedsOnFirstRun(t *testing.T) {
	f := switchSystem()
	delete(f.files, "/state")

	cfg := Config{Event: EventSwitch, Arg: "abc123", Arg2: "/nix/store/new-system"}
	if err := Run(f, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(f.published) != 0 {
		t.Errorf("published %d messages on a first run, want none", len(f.published))
	}
	if f.state["/state"] != "/nix/store/new-system" {
		t.Errorf("state = %q, want it seeded", f.state["/state"])
	}
}

// Without the explicit system path the current one is resolved, which is
// only correct outside activation — /run/current-system is not swapped until
// later.
func TestSwitchFallsBackToCurrentSystem(t *testing.T) {
	f := switchSystem()
	f.realPath["/run/current-system"] = "/nix/store/new-system"

	cfg := Config{Event: EventSwitch, Arg: "abc123"}
	if err := Run(f, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := onlyMessage(t, f)["nixos"]; got != "26.11.20260919.20b1ddd" {
		t.Errorf("nixos = %v", got)
	}
}

// An unknown revision is reported as dirty rather than as an empty field.
func TestSwitchReportsDirtyRevision(t *testing.T) {
	f := switchSystem()

	cfg := Config{Event: EventSwitch, Arg2: "/nix/store/new-system"}
	if err := Run(f, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := onlyMessage(t, f)["rev"]; got != "dirty" {
		t.Errorf("rev = %v, want dirty", got)
	}
}

// Missing pieces leave blank fields rather than suppressing the report: a
// switch that happened is worth announcing even if one detail is unreadable.
func TestSwitchToleratesMissingDetails(t *testing.T) {
	f := switchSystem()
	delete(f.files, "/nix/store/new-system/nixos-version")
	delete(f.dirs, "/nix/store/new-system/kernel-modules/lib/modules")
	delete(f.links, "/nix/var/nix/profiles/system")

	cfg := Config{Event: EventSwitch, Arg: "abc", Arg2: "/nix/store/new-system"}
	if err := Run(f, io.Discard, cfg, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := onlyMessage(t, f)
	for _, key := range []string{"nixos", "kernel", "gen"} {
		if got[key] != "" {
			t.Errorf("%s = %v, want empty", key, got[key])
		}
	}
	if got["event"] != EventSwitch {
		t.Errorf("no switch report was sent: %v", got)
	}
}

func TestScrubReportsCleanRun(t *testing.T) {
	f := newFakeSystem()
	f.commands["btrfs scrub status /data"] = "Status: finished\nError summary: no errors found"

	if err := Run(f, io.Discard, Config{Event: EventScrub, Arg: "/data"}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := onlyMessage(t, f)
	if got["mount"] != "/data" || got["errors"] != false {
		t.Errorf("payload = %v", got)
	}
	if !strings.Contains(got["text"].(string), "no errors found") {
		t.Errorf("text = %v", got["text"])
	}
}

// Anything that is not the clean line counts as trouble, including a status
// that could not be read at all.
func TestScrubReportsErrors(t *testing.T) {
	cases := map[string]string{
		"corruption": "Error summary: csum=3",
		"unreadable": "",
	}
	for name, status := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFakeSystem()
			f.commands["btrfs scrub status /"] = status

			if err := Run(f, io.Discard, Config{Event: EventScrub}, fast()); err != nil {
				t.Fatalf("Run: %v", err)
			}
			if got := onlyMessage(t, f)["errors"]; got != true {
				t.Errorf("errors = %v, want true", got)
			}
		})
	}
}

// The mount defaults to the root filesystem, which is what the unit relies
// on when it passes nothing.
func TestScrubDefaultsToRoot(t *testing.T) {
	f := newFakeSystem()
	f.commands["btrfs scrub status /"] = "no errors found"

	if err := Run(f, io.Discard, Config{Event: EventScrub}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := onlyMessage(t, f)["mount"]; got != "/" {
		t.Errorf("mount = %v, want /", got)
	}
}

// The freed line is the last one in the journal that reports deletions; a
// collection logs several and only the final total matters.
func TestGCReportsTheLastFreedLine(t *testing.T) {
	f := newFakeSystem()
	f.commands["journalctl -u nix-gc.service -o cat -n 50"] = strings.Join([]string{
		"starting",
		"1 store paths deleted, 1.00 MiB freed",
		"finding garbage",
		"42 store paths deleted, 3.50 GiB freed",
		"done",
	}, "\n")

	if err := Run(f, io.Discard, Config{Event: EventGC}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := onlyMessage(t, f)["freed"]; got != "42 store paths deleted, 3.50 GiB freed" {
		t.Errorf("freed = %v", got)
	}
}

// A collection that freed nothing still reports, with an empty field.
func TestGCReportsEmptyWhenNothingWasFreed(t *testing.T) {
	f := newFakeSystem()
	f.commands["journalctl -u nix-gc.service -o cat -n 50"] = "nothing to do"

	if err := Run(f, io.Discard, Config{Event: EventGC}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := onlyMessage(t, f)["freed"]; got != "" {
		t.Errorf("freed = %v, want empty", got)
	}
}

// The broker is routinely still starting when the boot report fires, which
// is the whole reason for the retry.
func TestRunRetriesThePublish(t *testing.T) {
	f := newFakeSystem()
	f.failUntil = 2

	if err := Run(f, io.Discard, Config{Event: EventBoot}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(f.published) != 3 {
		t.Errorf("published %d times, want 3", len(f.published))
	}
	if len(f.slept) != 2 {
		t.Errorf("slept %d times, want 2", len(f.slept))
	}
}

// A broker that never answers is reported and tolerated: a missed
// notification must not fail an activation or a boot.
func TestRunGivesUpOpen(t *testing.T) {
	f := newFakeSystem()
	f.publishAll = true

	var stderr bytes.Buffer
	if err := Run(f, &stderr, Config{Event: EventBoot}, fast()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(f.published) != 3 {
		t.Errorf("published %d times, want the full budget of 3", len(f.published))
	}
	if !strings.Contains(stderr.String(), "broker unreachable") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestRunRejectsUnknownEvent(t *testing.T) {
	f := newFakeSystem()

	err := Run(f, io.Discard, Config{Event: "explode"}, fast())

	var unknown *ErrUnknownEvent
	if !errors.As(err, &unknown) {
		t.Fatalf("error = %v, want an unknown event", err)
	}
	if unknown.Event != "explode" {
		t.Errorf("event = %q", unknown.Event)
	}
	if len(f.published) != 0 {
		t.Error("an unknown event published something")
	}
}

// A state file that cannot be written means the next activation would report
// the same switch again, so it is a real failure.
func TestSwitchReportsStateFailure(t *testing.T) {
	f := switchSystem()
	f.stateErr = errors.New("read-only file system")

	cfg := Config{Event: EventSwitch, Arg: "abc", Arg2: "/nix/store/new-system"}
	if err := Run(f, io.Discard, cfg, fast()); err == nil {
		t.Fatal("Run tolerated an unwritable state file")
	}
}

func TestOptionDefaults(t *testing.T) {
	got := Options{}.withDefaults()

	if got.Attempts != DefaultAttempts || got.RetryDelay != DefaultRetryDelay {
		t.Errorf("retries = %d x %v, want %d x %v",
			got.Attempts, got.RetryDelay, DefaultAttempts, DefaultRetryDelay)
	}
	if got.StatePath != DefaultStatePath {
		t.Errorf("state path = %q, want %q", got.StatePath, DefaultStatePath)
	}
}

// The real host implementation is exercised where it can be without being a
// particular machine.
func TestOSSystem(t *testing.T) {
	sys := OSSystem{}

	if sys.Nodename() == "" {
		t.Error("Nodename is empty")
	}
	if out := sys.Run("echo", "hello"); out != "hello" {
		t.Errorf("Run(echo) = %q, want hello", out)
	}
	if out := sys.Run("definitely-not-a-command-here"); out != "" {
		t.Errorf("a missing command returned %q, want empty", out)
	}

	dir := t.TempDir()
	statePath := filepath.Join(dir, "nested", "last-system")
	if err := sys.WriteState(statePath, "/nix/store/x"); err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	got, err := sys.ReadFile(statePath)
	if err != nil || got != "/nix/store/x" {
		t.Errorf("ReadFile = %q, %v", got, err)
	}

	if _, err := sys.ListDir(dir); err != nil {
		t.Errorf("ListDir: %v", err)
	}
	if _, err := sys.RealPath(dir); err != nil {
		t.Errorf("RealPath: %v", err)
	}
	if _, err := sys.Readlink(filepath.Join(dir, "absent")); err == nil {
		t.Error("Readlink on a non-link succeeded")
	}
	sys.Sleep(time.Millisecond)
}
