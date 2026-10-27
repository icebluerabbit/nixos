package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"homelab/flac-util/internal/measure"
	"homelab/flac-util/internal/pipeline"
	"homelab/flac-util/internal/report"
	"homelab/flac-util/internal/spectrogram"
)

// newTestFlags returns flags populated with the command's own defaults, which
// is what registering them against a throwaway command achieves.
func newTestFlags(t *testing.T) *collectFlags {
	t.Helper()

	// Registering against a throwaway command is what applies the defaults.
	cmd := &cobra.Command{Use: "collect"}
	f := &collectFlags{stageWorkers: map[string]*int{}, stageBuffers: map[string]*int{}}

	registerCollectFlags(cmd, f)
	if err := cmd.Flags().Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return f
}

// TestStageFlagsTakeTheirDefaultsFromThePipeline checks the command states no
// worker counts of its own: the numbers must come from where the stages are
// defined, so that there is only one place to change them.
func TestStageFlagsTakeTheirDefaultsFromThePipeline(t *testing.T) {
	var global GlobalOptions
	cmd := newCollectCommand(&global)

	for _, stage := range pipeline.CollectStages() {
		t.Run(stage.Name, func(t *testing.T) {
			workers := cmd.Flags().Lookup(stage.Name + "-workers")
			if workers == nil {
				t.Fatalf("the %s-workers flag is missing", stage.Name)
			}
			if got, want := workers.DefValue, strconv.Itoa(stage.Defaults.Workers); got != want {
				t.Errorf("default = %s, want %s", got, want)
			}

			buffer := cmd.Flags().Lookup(stage.Name + "-buffer")
			if buffer == nil {
				t.Fatalf("the %s-buffer flag is missing", stage.Name)
			}
			if got, want := buffer.DefValue, strconv.Itoa(stage.Defaults.Buffer); got != want {
				t.Errorf("default = %s, want %s", got, want)
			}
			if !strings.Contains(workers.Usage, stage.Why) {
				t.Errorf("the help does not explain the default:\n%s", workers.Usage)
			}
		})
	}
}

func TestRegisterCollectFlagsCoversEveryStage(t *testing.T) {
	var global GlobalOptions
	cmd := newCollectCommand(&global)

	for _, name := range pipeline.CollectStageNames() {
		for _, suffix := range []string{"-workers", "-buffer"} {
			if cmd.Flags().Lookup(name+suffix) == nil {
				t.Errorf("the %q flag is missing", name+suffix)
			}
		}
	}
}

func TestRegisterCollectFlagsCoversTheSettings(t *testing.T) {
	var global GlobalOptions
	cmd := newCollectCommand(&global)

	want := []string{
		"force", "dry-run", "skip-spectrogram", "report-suffix", "spec-suffix",
		"extensions", "walk-concurrency", "walk-buffer", "read-buffer",
		"fft-size", "fft-reference-rate", "fft-window", "fft-overlap", "spectrum-columns",
		"spec-width", "spec-height", "spec-freq-max", "spec-range-db", "spec-palette", "spec-gridlines",
	}
	for _, name := range want {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("the %q flag is missing", name)
		}
	}
}

func TestDefaultGridlineSpecRoundTrips(t *testing.T) {
	parsed, err := spectrogram.ParseGridlines(defaultGridlineSpec())
	if err != nil {
		t.Fatalf("ParseGridlines() error = %v", err)
	}

	want := spectrogram.DefaultGridlines()
	if len(parsed) != len(want) {
		t.Fatalf("parsed %d gridlines, want %d", len(parsed), len(want))
	}
	for i := range want {
		if parsed[i].Hz != want[i].Hz {
			t.Errorf("gridline %d = %v, want %v", i, parsed[i].Hz, want[i].Hz)
		}
	}
}

func TestCollectOptionsFromDefaults(t *testing.T) {
	f := newTestFlags(t)
	global := &GlobalOptions{}

	opts, err := f.collectOptions(global)
	if err != nil {
		t.Fatalf("collectOptions() error = %v", err)
	}

	if opts.Tool.Name != Name || opts.Tool.Version != Version {
		t.Errorf("Tool = %+v, want %s %s", opts.Tool, Name, Version)
	}
	if opts.ReportSuffix != report.DefaultReportSuffix {
		t.Errorf("ReportSuffix = %q, want %q", opts.ReportSuffix, report.DefaultReportSuffix)
	}
	if opts.Measure.Spectrum.Window != measure.DefaultWindow {
		t.Errorf("Window = %q, want %q", opts.Measure.Spectrum.Window, measure.DefaultWindow)
	}
	if opts.Image.MaxHz != spectrogram.DefaultMaxHz {
		t.Errorf("MaxHz = %v, want %v", opts.Image.MaxHz, spectrogram.DefaultMaxHz)
	}
	if len(opts.Stages) != len(pipeline.CollectStageNames()) {
		t.Errorf("configured %d stages, want %d", len(opts.Stages), len(pipeline.CollectStageNames()))
	}
	if err := opts.Validate(); err != nil {
		t.Errorf("the defaults do not validate: %v", err)
	}
}

func TestCollectOptionsRejectsBadFlags(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*collectFlags)
	}{
		{"unparseable gridlines", func(f *collectFlags) { f.gridlines = "sixteen thousand" }},
		{"gridline above the ceiling", func(f *collectFlags) { f.maxHz = 20000 }},
		{"unknown window", func(f *collectFlags) { f.window = "kaiser" }},
		{"unknown palette", func(f *collectFlags) { f.palette = "viridis" }},
		{"transform not a power of two", func(f *collectFlags) { f.fftSize = 3000 }},
		{"zero width", func(f *collectFlags) { f.width = 0 }},
		{
			name:   "a stage with no workers",
			mutate: func(f *collectFlags) { *f.stageWorkers[pipeline.StageDecode] = 0 },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestFlags(t)
			tt.mutate(f)
			if _, err := f.collectOptions(&GlobalOptions{}); err == nil {
				t.Fatal("collectOptions() succeeded, want error")
			}
		})
	}
}

func TestCollectOptionsClampsNegativeBuffers(t *testing.T) {
	f := newTestFlags(t)
	*f.stageBuffers[pipeline.StageDecode] = -10

	opts, err := f.collectOptions(&GlobalOptions{})
	if err != nil {
		t.Fatalf("collectOptions() error = %v", err)
	}
	if got := opts.Stages[pipeline.StageDecode].Buffer; got != 0 {
		t.Errorf("Buffer = %d, want 0", got)
	}
}

func TestCollectOptionsSkipSpectrogramSkipsTheAnalysisToo(t *testing.T) {
	f := newTestFlags(t)
	f.skipSpectrogram = true

	opts, err := f.collectOptions(&GlobalOptions{})
	if err != nil {
		t.Fatalf("collectOptions() error = %v", err)
	}
	if !opts.Measure.SkipSpectrum {
		t.Error("SkipSpectrum = false; there is no reason to analyse a spectrum nobody will draw")
	}
}

func TestWalkOptions(t *testing.T) {
	f := newTestFlags(t)

	opts := f.walkOptions([]string{"/music"})
	if len(opts.Roots) != 1 || opts.Roots[0] != "/music" {
		t.Errorf("Roots = %v, want [/music]", opts.Roots)
	}
	if opts.Concurrency != pipeline.DefaultWalkConcurrency {
		t.Errorf("Concurrency = %d, want %d", opts.Concurrency, pipeline.DefaultWalkConcurrency)
	}
	if len(opts.Extensions) != 1 || opts.Extensions[0] != pipeline.DefaultExtension {
		t.Errorf("Extensions = %v, want [%s]", opts.Extensions, pipeline.DefaultExtension)
	}
}

func TestFirstError(t *testing.T) {
	if got := firstError(nil, nil); got != nil {
		t.Errorf("firstError() = %v, want nil", got)
	}

	first := errStub("first")
	second := errStub("second")
	if got := firstError(nil, first, second); !errors.Is(got, first) {
		t.Errorf("firstError() = %v, want %v", got, first)
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }

// TestCollectEndToEnd runs the real command over a real file and checks the
// two sidecars land beside it.
func TestCollectEndToEnd(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join("..", "audio", "testdata", "sine16.flac")

	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	track := filepath.Join(dir, "01 Track.flac")
	if err := os.WriteFile(track, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"collect", dir,
		"--display", "never",
		"--verbose",
		"--spec-width", "64",
		"--spec-height", "32",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, out.String())
	}

	reportPath := report.SidecarPath(track, report.DefaultReportSuffix)
	raw, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("the report was not written: %v\n%s", err, out.String())
	}

	var r report.Report
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatalf("the report is not valid JSON: %v", err)
	}
	if !r.Integrity.Match {
		t.Errorf("Integrity.Match = false, want the checksum to agree")
	}
	if r.Tool.Version != Version {
		t.Errorf("Tool.Version = %q, want %q", r.Tool.Version, Version)
	}

	if _, err := os.Stat(report.SidecarPath(track, report.DefaultSpecSuffix)); err != nil {
		t.Errorf("the image was not written: %v", err)
	}
	if !strings.Contains(out.String(), "1 done") {
		t.Errorf("the summary does not report the file:\n%s", out.String())
	}
}

func TestCollectEndToEndDryRun(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join("..", "audio", "testdata", "sine16.flac")

	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "track.flac"), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"collect", dir, "--display", "never", "--dry-run", "--skip-spectrogram"})

	if err := root.Execute(); err != nil {
		t.Fatalf("Execute() error = %v\n%s", err, out.String())
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("directory holds %d files, want only the original track", len(entries))
	}
}

func TestCollectRejectsBadDisplayMode(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"collect", t.TempDir(), "--display", "fancy"})

	if err := root.Execute(); err == nil {
		t.Fatal("Execute() with an unknown display mode succeeded, want error")
	}
}
