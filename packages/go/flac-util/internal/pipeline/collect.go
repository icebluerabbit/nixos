package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"time"

	"homelab/flac-util/internal/audio"
	"homelab/flac-util/internal/measure"
	"homelab/flac-util/internal/report"
	"homelab/flac-util/internal/spectrogram"
)

// Names of the stages a collect run is made of.
//
// They are separate because they cost wildly different amounts and contend for
// different resources: reading a header is network latency, decoding is
// arithmetic, writing is latency again. Giving each its own pool is what lets
// a run be balanced against whatever hardware it finds itself on.
const (
	StageScreen   = "screen"
	StageProbe    = "probe"
	StageDecode   = "decode"
	StageHash     = "hash"
	StageVerify   = "verify"
	StageFinger   = "fingerprint"
	StageScale    = "scale"
	StageBuild    = "build"
	StageAnnotate = "annotate"
	StageEncode   = "encode"
	StageReport   = "report"
	StageMarshal  = "marshal"
	StageWrite    = "write"
)

// StageConfig is the worker count and queue depth for one stage.
type StageConfig struct {
	Workers int
	Buffer  int
}

// StageDefinition is a stage of a collect run together with the resources it
// gets unless told otherwise.
//
// The defaults live here, beside the stage they describe, rather than in the
// command that exposes them as flags: what a stage costs is a property of the
// work it does, and keeping the two together means adding a stage is a change
// to exactly one list.
type StageDefinition struct {
	Name     string
	Defaults StageConfig
	// Why records what the default is reasoning from, so that anyone moving
	// the number knows what it was chosen against.
	Why string
}

// CollectStages returns the stages of a collect run, in order, with the
// resources each starts with.
//
// These are opening guesses, not truths. Every one is exposed as a flag, and
// the interactive display shows which stage is holding work, so the numbers
// are meant to be moved once there is evidence rather than trusted.
func CollectStages() []StageDefinition {
	cores := runtime.NumCPU()

	return []StageDefinition{
		{
			Name:     StageScreen,
			Defaults: StageConfig{Workers: 16, Buffer: 64},
			Why:      "waits on the filesystem; on network storage the way to hide a round trip is to have several in flight",
		},
		{
			Name:     StageProbe,
			Defaults: StageConfig{Workers: 16, Buffer: 64},
			Why:      "reads a header and little else, so it is round trips rather than work",
		},
		{
			Name:     StageDecode,
			Defaults: StageConfig{Workers: cores, Buffer: 32},
			Why:      "the expensive stage and the only compute bound one; its queue is deep because everything upstream is cheap",
		},
		{
			Name:     StageHash,
			Defaults: StageConfig{Workers: 8, Buffer: 16},
			Why:      "reads the file again but does nothing with it; bounded by the disk rather than by the processor",
		},
		{
			Name:     StageVerify,
			Defaults: StageConfig{Workers: max(cores/2, 1), Buffer: 8},
			Why:      "decodes the whole file a second time in another process, so it competes with decode for the same cores",
		},
		{
			Name:     StageFinger,
			Defaults: StageConfig{Workers: max(cores/2, 1), Buffer: 8},
			Why:      "decodes and resamples in another process; competes with decode much as verify does",
		},
		{
			Name:     StageScale,
			Defaults: StageConfig{Workers: 2, Buffer: 4},
			Why:      "cheap, but its queue carries megabytes per item, so it stays shallow deliberately",
		},
		{
			Name:     StageBuild,
			Defaults: StageConfig{Workers: 2, Buffer: 4},
			Why:      "cheap, and holds a whole image per item",
		},
		{
			Name:     StageAnnotate,
			Defaults: StageConfig{Workers: 2, Buffer: 4},
			Why:      "draws a handful of lines; trivial next to everything around it",
		},
		{
			Name:     StageEncode,
			Defaults: StageConfig{Workers: 2, Buffer: 4},
			Why:      "compression is real work but small, and each queued item is an image",
		},
		{
			Name:     StageReport,
			Defaults: StageConfig{Workers: 2, Buffer: 8},
			Why:      "assembles a struct; the queue holds only the summary",
		},
		{
			Name:     StageMarshal,
			Defaults: StageConfig{Workers: 2, Buffer: 8},
			Why:      "renders JSON, which is bytes in and bytes out",
		},
		{
			Name:     StageWrite,
			Defaults: StageConfig{Workers: 4, Buffer: 16},
			Why:      "latency bound; writing small files over a network share is slower than it looks",
		},
	}
}

// CollectStageNames returns the stages of a collect run, in order.
func CollectStageNames() []string {
	stages := CollectStages()
	names := make([]string, 0, len(stages))
	for _, stage := range stages {
		names = append(names, stage.Name)
	}
	return names
}

// CollectOptions configures a collect run.
type CollectOptions struct {
	// Tool identifies this program in the reports it writes.
	Tool report.Tool
	// Measure selects which measurements run during decoding.
	Measure measure.Options
	// Image configures the rendered spectrogram.
	Image spectrogram.Options
	// ReportSuffix and SpecSuffix name the files written beside each track.
	ReportSuffix string
	SpecSuffix   string
	// SkipSpectrogram writes the report but no image.
	SkipSpectrogram bool
	// DryRun measures everything and writes nothing.
	DryRun bool
	// Force reprocesses files that already have a current report.
	Force bool
	// ReadBufferSize is the read-ahead applied to each file.
	ReadBufferSize int
	// SkipExternal omits the stages that shell out, which are the ones that
	// roughly double a run.
	SkipExternal bool
	// FlacTool and FpcalcTool name the external programs, for a caller that
	// has them somewhere unusual. Empty selects the names on PATH.
	FlacTool   string
	FpcalcTool string
	// Stages overrides the per-stage worker counts, keyed by stage name.
	// Stages absent from the map keep the defaults declared by CollectStages.
	Stages map[string]StageConfig
	// Now supplies the timestamp written into reports. Nil uses time.Now.
	Now func() time.Time
}

// withDefaults fills in the fields a caller may reasonably leave blank.
func (o CollectOptions) withDefaults() CollectOptions {
	if o.ReportSuffix == "" {
		o.ReportSuffix = report.DefaultReportSuffix
	}
	if o.SpecSuffix == "" {
		o.SpecSuffix = report.DefaultSpecSuffix
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.SkipSpectrogram {
		o.Measure.SkipSpectrum = true
	}
	return o
}

// Validate reports whether a collect run can be configured from the options.
func (o CollectOptions) Validate() error {
	if o.Tool.Name == "" || o.Tool.Version == "" {
		return fmt.Errorf("pipeline: the tool name and version are required")
	}
	if !o.Measure.SkipSpectrum {
		if err := o.Measure.Spectrum.Validate(); err != nil {
			return err
		}
	}
	if !o.SkipSpectrogram {
		if err := o.Image.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// NewCollectPipeline builds the stages of a collect run.
func NewCollectPipeline(opts CollectOptions) ([]StageSpec, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	opts = opts.withDefaults()

	stages := map[string]Stage{
		StageScreen:   NewStage(StageScreen, opts.screen),
		StageProbe:    NewStage(StageProbe, opts.probe),
		StageDecode:   NewStage(StageDecode, opts.decode),
		StageHash:     NewStage(StageHash, opts.hash),
		StageVerify:   NewStage(StageVerify, opts.verify),
		StageFinger:   NewStage(StageFinger, opts.fingerprint),
		StageScale:    NewStage(StageScale, opts.scale),
		StageBuild:    NewStage(StageBuild, opts.build),
		StageAnnotate: NewStage(StageAnnotate, opts.annotate),
		StageEncode:   NewStage(StageEncode, opts.encode),
		StageReport:   NewStage(StageReport, opts.report),
		StageMarshal:  NewStage(StageMarshal, opts.marshal),
		StageWrite:    NewStage(StageWrite, opts.write),
	}

	definitions := CollectStages()
	specs := make([]StageSpec, 0, len(definitions))

	for _, definition := range definitions {
		stage, ok := stages[definition.Name]
		if !ok {
			// A stage listed but not built would otherwise become a nil entry
			// that only fails much later, in the runner.
			return nil, fmt.Errorf("pipeline: stage %q has no implementation", definition.Name)
		}

		config, ok := opts.Stages[definition.Name]
		if !ok {
			config = definition.Defaults
		}
		specs = append(specs, StageSpec{
			Stage:   stage,
			Workers: max(config.Workers, 1),
			Buffer:  max(config.Buffer, 0),
		})
	}

	if len(specs) != len(stages) {
		return nil, fmt.Errorf("pipeline: %d stages built but %d listed", len(stages), len(specs))
	}
	return specs, nil
}

// screen decides whether the file already has a current report.
func (o CollectOptions) screen(_ context.Context, j *Job) error {
	reason, err := report.Check(j.Path, j.Info, report.CheckOptions{
		ReportSuffix: o.ReportSuffix,
		ToolVersion:  o.Tool.Version,
		Force:        o.Force,
	})
	if err != nil {
		return err
	}
	j.Skip = reason
	return nil
}

// probe reads the file's metadata without decoding any audio.
func (o CollectOptions) probe(_ context.Context, j *Job) error {
	metadata, err := audio.Probe(j.Path)
	if err != nil {
		return err
	}
	j.Metadata = metadata
	return nil
}

// decode reads every sample through the measurements. This is where nearly all
// of a run's time goes.
func (o CollectOptions) decode(ctx context.Context, j *Job) error {
	result, err := audio.Decode(ctx, j.Path, audio.DecodeOptions{
		Measure:    o.Measure,
		BufferSize: o.ReadBufferSize,
	})
	j.Decode = result

	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		// The run is winding down rather than the file being bad, and a report
		// covering however much was read before the interruption would say
		// something about this run rather than about the file.
		return err
	case result.Frames == 0:
		// Nothing came out of the file, so there is nothing to describe. A
		// header that will not parse is the whole finding.
		return err
	}

	// The audio stopped short, but what was read is real and the fault is
	// worth recording. Carrying on writes a report that says both.
	j.DecodeErr = err
	return nil
}

// scale resamples the measured grid onto the image's pixel raster.
func (o CollectOptions) scale(_ context.Context, j *Job) error {
	if o.SkipSpectrogram || j.Decode.Results.Spectrum == nil {
		return nil
	}

	levels, err := spectrogram.Scale(
		j.Decode.Results.Spectrum,
		o.Image.Width, o.Image.Height, o.Image.MaxHz,
	)
	if err != nil {
		return err
	}
	j.Levels = levels
	return nil
}

// build colours the levels into an indexed image.
func (o CollectOptions) build(_ context.Context, j *Job) error {
	if j.Levels == nil {
		return nil
	}

	img, err := spectrogram.Build(j.Levels, o.Image)
	if err != nil {
		return err
	}
	j.Image = img
	// The level table is several megabytes and nothing downstream reads it.
	j.Levels = nil
	return nil
}

// annotate draws the frequency ruler.
func (o CollectOptions) annotate(_ context.Context, j *Job) error {
	if j.Image == nil {
		return nil
	}
	return spectrogram.Annotate(j.Image, o.Image)
}

// encode compresses the image to PNG bytes.
func (o CollectOptions) encode(_ context.Context, j *Job) error {
	if j.Image == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := spectrogram.Encode(&buf, j.Image); err != nil {
		return err
	}
	j.PNG = buf.Bytes()
	// The image is the last large thing held; from here on the job carries
	// only the bytes to be written.
	j.Image = nil
	return nil
}

// report assembles the document written beside the track.
func (o CollectOptions) report(_ context.Context, j *Job) error {
	r, err := report.New(report.Input{
		Path:        j.Path,
		Size:        j.Size(),
		Modified:    modTime(j),
		Tool:        o.Tool,
		Metadata:    j.Metadata,
		Decode:      j.Decode,
		DecodeErr:   j.DecodeErr,
		DecodeStage: decodeStageOf(j),
		FileSHA256:  j.FileHash,
		Verify:      j.Verify,
		Fingerprint: j.Fingerprint,
		Spectrum:    o.Measure.Spectrum,
		Image:       o.Image,
		Now:         o.Now(),
	})
	if err != nil {
		return err
	}
	j.Report = r
	// The measured grid is the largest thing the decode stage produced, and
	// the report has taken the summary it needs.
	j.Decode.Results.Spectrum = nil
	return nil
}

// marshal renders the report as JSON.
func (o CollectOptions) marshal(_ context.Context, j *Job) error {
	data, err := report.Marshal(j.Report)
	if err != nil {
		return err
	}
	j.JSON = data
	return nil
}

// write puts both files beside the track.
func (o CollectOptions) write(_ context.Context, j *Job) error {
	if o.DryRun {
		return nil
	}

	if len(j.JSON) > 0 {
		if err := report.Write(report.SidecarPath(j.Path, o.ReportSuffix), j.JSON); err != nil {
			return err
		}
	}
	if len(j.PNG) > 0 {
		if err := report.Write(report.SidecarPath(j.Path, o.SpecSuffix), j.PNG); err != nil {
			return err
		}
	}
	return nil
}

// modTime returns the file's modification time, or the zero time when it is
// unknown.
func modTime(j *Job) time.Time {
	if j.Info == nil {
		return time.Time{}
	}
	return j.Info.ModTime()
}

// decodeStageOf names where a decode fault happened, empty when there was
// none. There is only one stage that reads audio, so this is a constant in all
// but name - it exists so the report says where the fault came from rather
// than leaving a reader to infer it.
func decodeStageOf(j *Job) string {
	if j.DecodeErr == nil {
		return ""
	}
	return StageDecode
}

// hash digests the file as it sits on disk.
func (o CollectOptions) hash(_ context.Context, j *Job) error {
	if o.SkipExternal {
		return nil
	}

	sum, err := audio.FileHash(j.Path, o.ReadBufferSize)
	if err != nil {
		return err
	}
	j.FileHash = sum
	return nil
}

// verify asks the reference decoder for its opinion of the file.
//
// A tool that is not installed is not a fault in the file: the result records
// that nothing was asked, and the run carries on.
func (o CollectOptions) verify(ctx context.Context, j *Job) error {
	if o.SkipExternal {
		return nil
	}

	result, err := audio.Verify(ctx, o.FlacTool, j.Path)
	if err != nil {
		return err
	}
	j.Verify = result
	return nil
}

// fingerprint computes an acoustic fingerprint of the audio.
func (o CollectOptions) fingerprint(ctx context.Context, j *Job) error {
	if o.SkipExternal {
		return nil
	}

	result, err := audio.Fingerprint(ctx, o.FpcalcTool, j.Path)
	if err != nil {
		// A fingerprint that cannot be taken is worth noting and not worth
		// abandoning a file over: everything else about it still stands.
		j.Fingerprint = audio.FingerprintResult{Ran: true}
		return nil
	}
	j.Fingerprint = result
	return nil
}
