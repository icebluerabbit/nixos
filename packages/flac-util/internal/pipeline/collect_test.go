package pipeline

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"flac-util/internal/measure"
	"flac-util/internal/progress"
	"flac-util/internal/report"
	"flac-util/internal/spectrogram"
)

var testNow = time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

// testCollectOptions returns a workable configuration with a small image, so
// that tests stay fast.
func testCollectOptions() CollectOptions {
	image := spectrogram.DefaultOptions()
	image.Width, image.Height = 64, 32

	return CollectOptions{
		Tool:    report.Tool{Name: "flac-util", Version: "test"},
		Measure: measure.Options{Spectrum: measure.DefaultSpectrumOptions()},
		Image:   image,
		Now:     func() time.Time { return testNow },
	}
}

// copyAlbum puts the fixture album into a writable temporary directory.
func copyAlbum(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	entries, err := os.ReadDir(filepath.Join("testdata", "album"))
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join("testdata", "album", entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	return dir
}

// runCollect walks a directory through a full collect pipeline.
func runCollect(t *testing.T, dir string, opts CollectOptions) *progress.Recorder {
	t.Helper()

	specs, err := NewCollectPipeline(opts)
	if err != nil {
		t.Fatalf("NewCollectPipeline() error = %v", err)
	}

	counters := progress.NewCounters(CollectStageNames(), time.Now())
	var rec progress.Recorder

	runner, err := NewRunner(specs, counters, &rec)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}
	walker, err := NewWalker(WalkOptions{Roots: []string{dir}, Buffer: 8}, counters)
	if err != nil {
		t.Fatalf("NewWalker() error = %v", err)
	}

	if err := runner.Run(t.Context(), walker.Start(t.Context())); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if err := walker.Err(); err != nil {
		t.Fatalf("walker error = %v", err)
	}
	return &rec
}

func TestCollectStageNames(t *testing.T) {
	want := []string{
		"screen", "probe", "decode",
		"scale", "build", "annotate", "encode",
		"report", "marshal", "write",
	}
	if got := CollectStageNames(); !slices.Equal(got, want) {
		t.Errorf("CollectStageNames() = %v, want %v", got, want)
	}
}

func TestCollectStagesAreUsable(t *testing.T) {
	stages := CollectStages()
	if len(stages) == 0 {
		t.Fatal("CollectStages() is empty")
	}

	seen := make(map[string]bool, len(stages))
	for _, stage := range stages {
		t.Run(stage.Name, func(t *testing.T) {
			if stage.Name == "" {
				t.Error("the stage has no name")
			}
			if seen[stage.Name] {
				t.Errorf("the stage %q appears twice", stage.Name)
			}
			seen[stage.Name] = true

			if stage.Defaults.Workers < 1 {
				t.Errorf("Workers = %d, want at least one", stage.Defaults.Workers)
			}
			if stage.Defaults.Buffer < 0 {
				t.Errorf("Buffer = %d, want a non-negative depth", stage.Defaults.Buffer)
			}
			if stage.Why == "" {
				t.Error("the default is unexplained, so nobody can tell what it was chosen against")
			}
		})
	}
}

// TestCollectStageDefaultsMatchTheirCost pins the reasoning behind the
// numbers: the stages that wait get concurrency, the stage that computes gets
// cores, and the stages carrying megabytes per item queue shallowly.
func TestCollectStageDefaultsMatchTheirCost(t *testing.T) {
	defaults := make(map[string]StageConfig)
	for _, stage := range CollectStages() {
		defaults[stage.Name] = stage.Defaults
	}

	probe, decode := defaults[StageProbe], defaults[StageDecode]
	if probe.Workers < 8 {
		t.Errorf("probe has %d workers; a latency bound stage wants more in flight", probe.Workers)
	}
	if decode.Workers != runtime.NumCPU() {
		t.Errorf("decode has %d workers, want one per core (%d)", decode.Workers, runtime.NumCPU())
	}

	for _, name := range []string{StageScale, StageBuild, StageAnnotate, StageEncode} {
		if defaults[name].Buffer >= decode.Buffer {
			t.Errorf("%s buffers %d against decode's %d; a stage holding images should queue less",
				name, defaults[name].Buffer, decode.Buffer)
		}
	}
}

// TestNewCollectPipelineUsesTheDeclaredDefaults covers the path a caller takes
// when it configures nothing.
func TestNewCollectPipelineUsesTheDeclaredDefaults(t *testing.T) {
	opts := testCollectOptions()
	opts.Stages = nil

	specs, err := NewCollectPipeline(opts)
	if err != nil {
		t.Fatalf("NewCollectPipeline() error = %v", err)
	}

	declared := CollectStages()
	for i, spec := range specs {
		if spec.Workers != declared[i].Defaults.Workers {
			t.Errorf("stage %q has %d workers, want the declared %d",
				spec.Stage.Name(), spec.Workers, declared[i].Defaults.Workers)
		}
		if spec.Buffer != declared[i].Defaults.Buffer {
			t.Errorf("stage %q buffers %d, want the declared %d",
				spec.Stage.Name(), spec.Buffer, declared[i].Defaults.Buffer)
		}
	}
}

func TestCollectOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CollectOptions)
		wantErr bool
	}{
		{"defaults", func(*CollectOptions) {}, false},
		{"no tool name", func(o *CollectOptions) { o.Tool.Name = "" }, true},
		{"no tool version", func(o *CollectOptions) { o.Tool.Version = "" }, true},
		{"bad spectrum settings", func(o *CollectOptions) { o.Measure.Spectrum.FFTSize = 3 }, true},
		{"bad image settings", func(o *CollectOptions) { o.Image.Width = 0 }, true},
		{
			name:    "bad image settings are irrelevant when no image is drawn",
			mutate:  func(o *CollectOptions) { o.Image.Width = 0; o.SkipSpectrogram = true },
			wantErr: false,
		},
		{
			name:    "bad spectrum settings are irrelevant when the spectrum is skipped",
			mutate:  func(o *CollectOptions) { o.Measure.Spectrum = measure.SpectrumOptions{}; o.Measure.SkipSpectrum = true },
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := testCollectOptions()
			tt.mutate(&opts)
			if err := opts.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCollectPipelineBuildsEveryStageInOrder(t *testing.T) {
	opts := testCollectOptions()
	opts.Stages = map[string]StageConfig{
		StageDecode: {Workers: 4, Buffer: 8},
	}

	specs, err := NewCollectPipeline(opts)
	if err != nil {
		t.Fatalf("NewCollectPipeline() error = %v", err)
	}
	if len(specs) != len(CollectStageNames()) {
		t.Fatalf("built %d stages, want %d", len(specs), len(CollectStageNames()))
	}

	for i, name := range CollectStageNames() {
		if got := specs[i].Stage.Name(); got != name {
			t.Errorf("stage %d = %q, want %q", i, got, name)
		}
	}
	for _, spec := range specs {
		if spec.Workers < 1 {
			t.Errorf("stage %q has %d workers, want at least one", spec.Stage.Name(), spec.Workers)
		}
		if spec.Stage.Name() == StageDecode {
			if spec.Workers != 4 || spec.Buffer != 8 {
				t.Errorf("decode = %d workers / %d buffer, want 4/8", spec.Workers, spec.Buffer)
			}
		}
	}
}

func TestNewCollectPipelineRejectsBadOptions(t *testing.T) {
	opts := testCollectOptions()
	opts.Tool.Version = ""
	if _, err := NewCollectPipeline(opts); err == nil {
		t.Fatal("NewCollectPipeline() succeeded with no tool version, want error")
	}
}

// TestCollectWritesBothSidecars is the end-to-end test: real files in, a
// report and an image out, beside each track and named after it.
func TestCollectWritesBothSidecars(t *testing.T) {
	dir := copyAlbum(t)
	rec := runCollect(t, dir, testCollectOptions())

	if got := rec.Count(progress.OutcomeCompleted); got != 2 {
		t.Fatalf("completed = %d, want 2; events: %+v", got, rec.Events())
	}
	if got := rec.Count(progress.OutcomeFailed); got != 0 {
		t.Errorf("failed = %d, want 0; events: %+v", got, rec.Events())
	}

	track := filepath.Join(dir, "01 Track.flac")

	data, err := os.ReadFile(report.SidecarPath(track, report.DefaultReportSuffix))
	if err != nil {
		t.Fatalf("the report was not written: %v", err)
	}
	var r report.Report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("the report is not valid JSON: %v", err)
	}
	if r.Note == "" {
		t.Error("the report carries no explanatory note")
	}
	if !r.Integrity.Match {
		t.Errorf("Integrity.Match = false, want the checksum to agree: %+v", r.Integrity)
	}
	if r.Stream.SampleRate != 44100 {
		t.Errorf("SampleRate = %d, want 44100", r.Stream.SampleRate)
	}
	if r.Tool.Version != "test" {
		t.Errorf("Tool.Version = %q, want test", r.Tool.Version)
	}
	if !r.Generated.Equal(testNow) {
		t.Errorf("Generated = %v, want %v", r.Generated, testNow)
	}
	if r.Spectrum == nil || len(r.Spectrum.AverageDBFS) == 0 {
		t.Error("the report carries no spectrum summary")
	}

	image, err := os.Open(report.SidecarPath(track, report.DefaultSpecSuffix))
	if err != nil {
		t.Fatalf("the image was not written: %v", err)
	}
	defer image.Close()

	config, err := png.DecodeConfig(image)
	if err != nil {
		t.Fatalf("the image is not a valid PNG: %v", err)
	}
	if config.Width != 64 || config.Height != 32 {
		t.Errorf("image is %dx%d, want 64x32", config.Width, config.Height)
	}
}

func TestCollectLeavesTheAudioAlone(t *testing.T) {
	dir := copyAlbum(t)
	track := filepath.Join(dir, "01 Track.flac")

	before, err := os.ReadFile(track)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	runCollect(t, dir, testCollectOptions())

	after, err := os.ReadFile(track)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(before) != len(after) || string(before) != string(after) {
		t.Error("the audio file was modified, which it must never be")
	}
}

func TestCollectDryRunWritesNothing(t *testing.T) {
	dir := copyAlbum(t)
	opts := testCollectOptions()
	opts.DryRun = true

	rec := runCollect(t, dir, opts)
	if got := rec.Count(progress.OutcomeCompleted); got != 2 {
		t.Fatalf("completed = %d, want 2", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("directory holds %d files, want the original three", len(entries))
	}
}

func TestCollectSkipSpectrogramWritesOnlyTheReport(t *testing.T) {
	dir := copyAlbum(t)
	opts := testCollectOptions()
	opts.SkipSpectrogram = true

	runCollect(t, dir, opts)
	track := filepath.Join(dir, "01 Track.flac")

	if _, err := os.Stat(report.SidecarPath(track, report.DefaultReportSuffix)); err != nil {
		t.Errorf("the report was not written: %v", err)
	}
	if _, err := os.Stat(report.SidecarPath(track, report.DefaultSpecSuffix)); err == nil {
		t.Error("an image was written despite the spectrogram being skipped")
	}

	data, err := os.ReadFile(report.SidecarPath(track, report.DefaultReportSuffix))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var r report.Report
	if err := json.Unmarshal(data, &r); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if r.Spectrum != nil {
		t.Error("the report carries a spectrum summary despite the spectrum being skipped")
	}
	if !r.Integrity.Match {
		t.Error("Integrity.Match = false, want the checksum to agree")
	}
}

// TestCollectSecondRunSkips is what makes a library-wide run resumable.
func TestCollectSecondRunSkips(t *testing.T) {
	dir := copyAlbum(t)
	opts := testCollectOptions()

	runCollect(t, dir, opts)
	second := runCollect(t, dir, opts)

	if got := second.Count(progress.OutcomeSkipped); got != 2 {
		t.Errorf("skipped = %d on the second run, want 2; events: %+v", got, second.Events())
	}
	if got := second.Count(progress.OutcomeCompleted); got != 0 {
		t.Errorf("completed = %d on the second run, want 0", got)
	}
}

func TestCollectForceRedoesEverything(t *testing.T) {
	dir := copyAlbum(t)
	opts := testCollectOptions()

	runCollect(t, dir, opts)
	opts.Force = true
	second := runCollect(t, dir, opts)

	if got := second.Count(progress.OutcomeCompleted); got != 2 {
		t.Errorf("completed = %d with force, want 2", got)
	}
}

func TestCollectANewToolVersionRedoesEverything(t *testing.T) {
	dir := copyAlbum(t)
	opts := testCollectOptions()
	runCollect(t, dir, opts)

	opts.Tool.Version = "next"
	second := runCollect(t, dir, opts)

	if got := second.Count(progress.OutcomeCompleted); got != 2 {
		t.Errorf("completed = %d after a version change, want 2", got)
	}
}

func TestCollectCustomSuffixes(t *testing.T) {
	dir := copyAlbum(t)
	opts := testCollectOptions()
	opts.ReportSuffix = ".measurements.json"
	opts.SpecSuffix = ".spectral.png"

	runCollect(t, dir, opts)
	track := filepath.Join(dir, "01 Track.flac")

	for _, suffix := range []string{".measurements.json", ".spectral.png"} {
		if _, err := os.Stat(report.SidecarPath(track, suffix)); err != nil {
			t.Errorf("nothing written for suffix %q: %v", suffix, err)
		}
	}
}

// TestCollectRecordsAFailureWithoutStoppingTheRun covers a corrupt file
// sitting in the middle of a library.
func TestCollectRecordsAFailureWithoutStoppingTheRun(t *testing.T) {
	dir := copyAlbum(t)
	broken := filepath.Join(dir, "broken.flac")
	if err := os.WriteFile(broken, []byte("this is not a FLAC stream"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	rec := runCollect(t, dir, testCollectOptions())

	if got := rec.Count(progress.OutcomeFailed); got != 1 {
		t.Errorf("failed = %d, want 1", got)
	}
	if got := rec.Count(progress.OutcomeCompleted); got != 2 {
		t.Errorf("completed = %d, want the two good tracks", got)
	}

	for _, e := range rec.Events() {
		if e.Outcome != progress.OutcomeFailed {
			continue
		}
		if e.Stage != StageProbe {
			t.Errorf("failure attributed to stage %q, want %q", e.Stage, StageProbe)
		}
		if _, err := os.Stat(report.SidecarPath(broken, report.DefaultReportSuffix)); err == nil {
			t.Error("a report was written for a file that could not be read")
		}
	}
}

// TestCollectReleasesLargeIntermediates guards the memory discipline the
// pipeline depends on: nothing bulky may survive the stage that consumed it.
func TestCollectReleasesLargeIntermediates(t *testing.T) {
	opts := testCollectOptions().withDefaults()
	job := &Job{
		Path: filepath.Join("testdata", "album", "01 Track.flac"),
	}

	ctx := t.Context()
	if err := opts.probe(ctx, job); err != nil {
		t.Fatalf("probe() error = %v", err)
	}
	if err := opts.decode(ctx, job); err != nil {
		t.Fatalf("decode() error = %v", err)
	}
	if err := opts.scale(ctx, job); err != nil {
		t.Fatalf("scale() error = %v", err)
	}
	if job.Levels == nil {
		t.Fatal("scale() produced no levels")
	}

	if err := opts.build(ctx, job); err != nil {
		t.Fatalf("build() error = %v", err)
	}
	if job.Levels != nil {
		t.Error("build() left the level table behind")
	}
	if job.Image == nil {
		t.Fatal("build() produced no image")
	}

	if err := opts.annotate(ctx, job); err != nil {
		t.Fatalf("annotate() error = %v", err)
	}
	if err := opts.encode(ctx, job); err != nil {
		t.Fatalf("encode() error = %v", err)
	}
	if job.Image != nil {
		t.Error("encode() left the image behind")
	}
	if len(job.PNG) == 0 {
		t.Fatal("encode() produced no bytes")
	}

	if err := opts.report(ctx, job); err != nil {
		t.Fatalf("report() error = %v", err)
	}
	if job.Decode.Results.Spectrum != nil {
		t.Error("report() left the measured grid behind")
	}
}
