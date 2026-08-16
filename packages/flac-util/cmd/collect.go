package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"flac-util/internal/audio"
	"flac-util/internal/measure"
	"flac-util/internal/pipeline"
	"flac-util/internal/progress"
	"flac-util/internal/report"
	"flac-util/internal/spectrogram"
	"flac-util/internal/tui"
)

// summaryInterval is how often the non-interactive reporter prints a summary.
const summaryInterval = 30 * time.Second

// collectFlags holds everything the collect command accepts.
type collectFlags struct {
	// behaviour
	force           bool
	dryRun          bool
	skipSpectrogram bool
	reportSuffix    string
	specSuffix      string

	// traversal
	extensions      []string
	walkConcurrency int
	walkBuffer      int
	readBufferBytes int

	// per stage, keyed by stage name
	stageWorkers map[string]*int
	stageBuffers map[string]*int

	// analysis
	fftSize       int
	referenceRate int
	window        string
	overlap       float64
	columns       int

	// image
	width     int
	height    int
	maxHz     float64
	rangeDB   float64
	palette   string
	gridlines string
}

// newCollectCommand builds the collect subcommand.
func newCollectCommand(global *GlobalOptions) *cobra.Command {
	flags := &collectFlags{
		stageWorkers: map[string]*int{},
		stageBuffers: map[string]*int{},
	}

	cmd := &cobra.Command{
		Use:   "collect [path...]",
		Short: "Measure every FLAC file under the given paths",
		Long: "Walks the given paths, measures every FLAC file found, and writes\n" +
			"a report and a spectrogram beside each one.\n\n" +
			"Audio files are opened read-only. Point this at a library rather\n" +
			"than at a torrent download directory: the files it writes are new\n" +
			"neighbours, but a download directory is not yours to add to.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCollect(cmd, args, global, flags)
		},
	}

	registerCollectFlags(cmd, flags)
	return cmd
}

// registerCollectFlags declares every flag the command accepts.
func registerCollectFlags(cmd *cobra.Command, f *collectFlags) {
	fs := cmd.Flags()

	fs.BoolVar(&f.force, "force", false, "re-measure files that already have a current report")
	fs.BoolVar(&f.dryRun, "dry-run", false, "measure everything and write nothing")
	fs.BoolVar(&f.skipSpectrogram, "skip-spectrogram", false, "write the report but no image")
	fs.StringVar(&f.reportSuffix, "report-suffix", report.DefaultReportSuffix, "suffix of the report written beside each track")
	fs.StringVar(&f.specSuffix, "spec-suffix", report.DefaultSpecSuffix, "suffix of the image written beside each track")

	fs.StringSliceVar(&f.extensions, "extensions", []string{pipeline.DefaultExtension}, "file extensions to collect")
	fs.IntVar(&f.walkConcurrency, "walk-concurrency", pipeline.DefaultWalkConcurrency, "directories read at once")
	fs.IntVar(&f.walkBuffer, "walk-buffer", 256, "queue depth between the walker and the pipeline")
	fs.IntVar(&f.readBufferBytes, "read-buffer", audio.DefaultBufferSize, "per-file read-ahead, in bytes")

	// One pair of flags per stage, taken from the pipeline's own list, so that
	// adding a stage cannot leave it unconfigurable and the defaults can only
	// be stated in one place.
	for _, stage := range pipeline.CollectStages() {
		workers, buffer := new(int), new(int)

		fs.IntVar(workers, stage.Name+"-workers", stage.Defaults.Workers,
			fmt.Sprintf("workers running the %s stage (%s)", stage.Name, stage.Why))
		fs.IntVar(buffer, stage.Name+"-buffer", stage.Defaults.Buffer,
			fmt.Sprintf("queue depth feeding the %s stage", stage.Name))

		f.stageWorkers[stage.Name] = workers
		f.stageBuffers[stage.Name] = buffer
	}

	defaults := measure.DefaultSpectrumOptions()
	fs.IntVar(&f.fftSize, "fft-size", defaults.FFTSize,
		"transform length at the reference sample rate; scaled with the file's own rate")
	fs.IntVar(&f.referenceRate, "fft-reference-rate", defaults.ReferenceRate, "sample rate the transform length is quoted at")
	fs.StringVar(&f.window, "fft-window", defaults.Window,
		fmt.Sprintf("window function: one of %v", measure.WindowNames()))
	fs.Float64Var(&f.overlap, "fft-overlap", defaults.Overlap, "fraction of each frame shared with the next")
	fs.IntVar(&f.columns, "spectrum-columns", defaults.Columns, "time slices retained by the analysis")

	image := spectrogram.DefaultOptions()
	fs.IntVar(&f.width, "spec-width", image.Width, "image width in pixels")
	fs.IntVar(&f.height, "spec-height", image.Height, "image height in pixels")
	fs.Float64Var(&f.maxHz, "spec-freq-max", image.MaxHz,
		"frequency at the top of the image; fixed so a frequency lands on the same row in every file")
	fs.Float64Var(&f.rangeDB, "spec-range-db", image.RangeDB, "how far below full scale still shows, in decibels")
	fs.StringVar(&f.palette, "spec-palette", image.Palette,
		fmt.Sprintf("colour ramp: one of %v", spectrogram.PaletteNames()))
	fs.StringVar(&f.gridlines, "spec-gridlines", defaultGridlineSpec(),
		"comma separated frequencies in hertz to rule, each optionally followed by a colon and a label")
}

// defaultGridlineSpec renders the default gridlines as a flag value.
func defaultGridlineSpec() string {
	lines := spectrogram.DefaultGridlines()
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		parts = append(parts, fmt.Sprintf("%g", line.Hz))
	}
	return strings.Join(parts, ",")
}

// collectOptions turns the flags into the options the pipeline takes.
func (f *collectFlags) collectOptions(global *GlobalOptions) (pipeline.CollectOptions, error) {
	gridlines, err := spectrogram.ParseGridlines(f.gridlines)
	if err != nil {
		return pipeline.CollectOptions{}, err
	}

	stages := make(map[string]pipeline.StageConfig, len(f.stageWorkers))
	for name, workers := range f.stageWorkers {
		if *workers < 1 {
			return pipeline.CollectOptions{}, fmt.Errorf("stage %q needs at least one worker, got %d", name, *workers)
		}
		buffer := 0
		if b, ok := f.stageBuffers[name]; ok {
			buffer = max(*b, 0)
		}
		stages[name] = pipeline.StageConfig{Workers: *workers, Buffer: buffer}
	}

	opts := pipeline.CollectOptions{
		Tool: global.Tool(),
		Measure: measure.Options{
			Spectrum: measure.SpectrumOptions{
				FFTSize:       f.fftSize,
				ReferenceRate: f.referenceRate,
				Window:        f.window,
				Overlap:       f.overlap,
				Columns:       f.columns,
			},
			SkipSpectrum: f.skipSpectrogram,
		},
		Image: spectrogram.Options{
			Width:     f.width,
			Height:    f.height,
			MaxHz:     f.maxHz,
			RangeDB:   f.rangeDB,
			Palette:   f.palette,
			Gridlines: gridlines,
		},
		ReportSuffix:    f.reportSuffix,
		SpecSuffix:      f.specSuffix,
		SkipSpectrogram: f.skipSpectrogram,
		DryRun:          f.dryRun,
		Force:           f.force,
		ReadBufferSize:  f.readBufferBytes,
		Stages:          stages,
	}
	if err := opts.Validate(); err != nil {
		return pipeline.CollectOptions{}, err
	}
	return opts, nil
}

// walkOptions turns the flags into the walker's options.
func (f *collectFlags) walkOptions(roots []string) pipeline.WalkOptions {
	return pipeline.WalkOptions{
		Roots:       roots,
		Extensions:  f.extensions,
		Concurrency: f.walkConcurrency,
		Buffer:      f.walkBuffer,
	}
}

// runCollect wires the walker, the pipeline and a display together.
func runCollect(cmd *cobra.Command, roots []string, global *GlobalOptions, flags *collectFlags) error {
	opts, err := flags.collectOptions(global)
	if err != nil {
		return err
	}

	interactive, err := global.UseInteractive(stdoutIsTerminal())
	if err != nil {
		return err
	}

	specs, err := pipeline.NewCollectPipeline(opts)
	if err != nil {
		return err
	}

	counters := progress.NewCounters(pipeline.CollectStageNames(), time.Now())
	walker, err := pipeline.NewWalker(flags.walkOptions(roots), counters)
	if err != nil {
		return err
	}

	// A signal winds the run down rather than killing it, so that no report is
	// left half written.
	ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if interactive {
		return runInteractive(ctx, stop, specs, counters, walker)
	}
	return runPlain(ctx, cmd, specs, counters, walker, global.Verbose)
}

// runPlain drives the pipeline with line-based output.
func runPlain(
	ctx context.Context,
	cmd *cobra.Command,
	specs []pipeline.StageSpec,
	counters *progress.Counters,
	walker *pipeline.Walker,
	verbose bool,
) error {
	reporter := progress.NewPlain(cmd.OutOrStdout(), counters, progress.PlainOptions{
		Verbose:  verbose,
		Interval: summaryInterval,
	})

	runner, err := pipeline.NewRunner(specs, counters, reporter)
	if err != nil {
		return err
	}

	runErr := runner.Run(ctx, walker.Start(ctx))
	closeErr := reporter.Close()
	return firstError(runErr, walker.Err(), closeErr)
}

// runInteractive drives the pipeline behind the terminal display.
func runInteractive(
	ctx context.Context,
	stop context.CancelFunc,
	specs []pipeline.StageSpec,
	counters *progress.Counters,
	walker *pipeline.Walker,
) error {
	body := tui.NewCollectModel(counters)
	shell := tui.NewShell(body, stop)
	program := tea.NewProgram(shell)

	reporter := tui.NewReporter(program, 0)
	runner, err := pipeline.NewRunner(specs, counters, reporter)
	if err != nil {
		return err
	}

	var runErr error
	go func() {
		runErr = runner.Run(ctx, walker.Start(ctx))
		// Closing first flushes whatever is still queued, so the display shows
		// the last few files before it takes the news that the run is over.
		_ = reporter.Close()
		program.Send(tui.FinishedMsg{Err: runErr})
	}()

	if _, err := program.Run(); err != nil {
		return err
	}
	return firstError(runErr, walker.Err())
}

// firstError returns the first error that is not nil.
func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
