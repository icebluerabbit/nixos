// Package report turns measurements into the files left beside a track.
//
// The report records what was measured and nothing more. It contains no
// scores, no thresholds and no conclusions, because working out what the
// numbers mean is the reader's job and the tool has no business pretending
// otherwise.
package report

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"homelab/flac-util/internal/audio"
	"homelab/flac-util/internal/measure"
	"homelab/flac-util/internal/spectrogram"
)

// Note is written at the top of every report, because the file will be read by
// people who have no idea where it came from.
const Note = "Automated measurements of the FLAC file beside this report. " +
	"These are observations, not conclusions: nothing here judges whether the " +
	"file is genuine. Compare 'integrity.stored_md5' against the file's own " +
	"checksum to confirm this report describes the file you have."

// Number is a floating point value that marshals as null when it is not
// finite, since JSON has no way to write infinity or not-a-number and silence
// legitimately measures as negative infinity.
type Number float64

// spectrumDecimals is how much of a decibel figure the spectrum arrays keep.
//
// A dB value arrives here from float32 power sums by way of a logarithm, and
// prints in full as something like -94.64743041992188. Everything past the
// second decimal is an artifact of the binary representation rather than
// anything the transform resolved, and 0.01 dB is already far finer than the
// quantisation floor of the files being measured. Written out in full, those
// digits are most of a report: there are thousands of them per file and only a
// few dozen other numbers.
const spectrumDecimals = 2

// roundDB rounds a decibel figure to the precision the spectrum arrays keep.
//
// Values that are not finite pass through untouched, so that a silent bin
// stays negative infinity and still marshals as null.
func roundDB(v float64) Number {
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return Number(v)
	}
	scale := math.Pow(10, spectrumDecimals)
	return Number(math.Round(v*scale) / scale)
}

// MarshalJSON implements json.Marshaler.
func (n Number) MarshalJSON() ([]byte, error) {
	v := float64(n)
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return []byte("null"), nil
	}
	return json.Marshal(v)
}

// UnmarshalJSON implements json.Unmarshaler, reading null back as negative
// infinity so a round trip preserves the distinction between silence and zero.
func (n *Number) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*n = Number(math.Inf(-1))
		return nil
	}
	var v float64
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*n = Number(v)
	return nil
}

// Tool identifies what produced a report.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// File describes the audio file on disk.
type File struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size_bytes"`
	Modified time.Time `json:"modified"`
}

// Stream is what the file's header claims about its audio.
type Stream struct {
	SampleRate    int    `json:"sample_rate_hz"`
	Channels      int    `json:"channels"`
	BitsPerSample int    `json:"bits_per_sample"`
	TotalSamples  uint64 `json:"total_samples"`
	DurationSecs  Number `json:"duration_seconds"`
	BitrateKbps   Number `json:"bitrate_kbps"`
	BlockSizeMin  int    `json:"block_size_min"`
	BlockSizeMax  int    `json:"block_size_max"`
	FrameSizeMin  int    `json:"frame_size_min"`
	FrameSizeMax  int    `json:"frame_size_max"`
	Vendor        string `json:"vendor"`
	AudioOffset   int64  `json:"audio_offset_bytes"`
}

// Tags records the file's metadata blocks verbatim, plus the tag formats found
// that FLAC does not define.
type Tags struct {
	HasVorbisComment  bool        `json:"has_vorbis_comment"`
	Comments          [][2]string `json:"comments"`
	SeekPoints        int         `json:"seek_points"`
	HasCueSheet       bool        `json:"has_cue_sheet"`
	Pictures          int         `json:"pictures"`
	ApplicationBlocks int         `json:"application_blocks"`
	PaddingBytes      int64       `json:"padding_bytes"`
	ForeignTags       []string    `json:"foreign_tags"`
}

// Integrity compares the checksum the file carries against the audio that came
// out of it.
type Integrity struct {
	// StoredMD5 is the checksum recorded in the file's header, empty when the
	// encoder left it blank.
	StoredMD5 string `json:"stored_md5"`
	// ComputedMD5 is the checksum of the audio as decoded here.
	ComputedMD5 string `json:"computed_md5"`
	// StoredMD5Present is false when the header carried no checksum, in which
	// case there is nothing to compare against.
	StoredMD5Present bool `json:"stored_md5_present"`
	// Match is true when the two agree. It is false when they differ and when
	// there was nothing stored to compare.
	Match bool `json:"match"`
	// SamplesDecoded is how many inter-channel samples came out of the file.
	SamplesDecoded uint64 `json:"samples_decoded"`
	// FramesDecoded is how many audio frames were read.
	FramesDecoded int64 `json:"frames_decoded"`
}

// Decode records how reading the audio went.
//
// A file whose audio stops short still gets a report. Everything measured up
// to the fault is real, and the fault itself is one of the more telling things
// a file can tell you - a stream that ends in the wrong place has been through
// something. Discarding all of it because the last frame was bad would throw
// away the evidence along with the error.
//
// Read this before anything else in the report: measurements taken over part
// of a file describe that part, not the file.
type Decode struct {
	// Complete is true when every frame was read and the stream ended where
	// it said it would.
	Complete bool `json:"complete"`
	// Error is what went wrong, empty when nothing did.
	Error string `json:"error,omitempty"`
	// Stage names where it went wrong, empty when nothing did. A fault in the
	// header means no audio was read at all; one in the frames means the
	// measurements cover whatever came before it.
	Stage string `json:"stage,omitempty"`
}

// Loudness reports programme loudness to ITU-R BS.1770-5 and loudness range to
// EBU Tech 3342.
//
// Both are standard measures rather than this tool's own, so a figure here is
// comparable with one from any other meter. A CD master mixed for loudness and
// a hi-res master of the same music differ here by more than they differ
// anywhere else in this report.
type Loudness struct {
	// IntegratedLUFS is the loudness of the whole track, gated so that silence
	// between passages does not drag it down. Digital silence marshals as null.
	IntegratedLUFS Number `json:"integrated_lufs"`
	// RangeLU is the spread between its quiet and loud parts.
	RangeLU Number `json:"range_lu"`
	// Blocks is how many windows were measured, and GatedBlocks how many
	// counted towards the figure. A wide gap between them means a track with a
	// lot of near-silence in it.
	Blocks      int `json:"blocks"`
	GatedBlocks int `json:"gated_blocks"`
}

// Provenance is what the file says about itself from outside the audio.
//
// The checksum in integrity covers the decoded samples, so two files with the
// same music and different tags share it. FileSHA256 covers the bytes on disk,
// which is what finds the same file twice rather than the same recording.
// Fingerprint is the opposite: it identifies the recording however it was
// encoded or tagged.
type Provenance struct {
	// FileSHA256 is the digest of the whole file.
	FileSHA256 string `json:"file_sha256,omitempty"`
	// ReferenceDecoder is what the reference implementation made of the file.
	ReferenceDecoder ReferenceDecoder `json:"reference_decoder"`
	// AcoustID is the acoustic fingerprint, empty when none was taken.
	AcoustID string `json:"acoustid_fingerprint,omitempty"`
	// AcoustIDDuration is the length the fingerprinter measured, which need
	// not agree with the header.
	AcoustIDDuration string `json:"acoustid_duration,omitempty"`
}

// ReferenceDecoder records a second opinion on the file.
//
// This program has its own decoder, and everything measured here came out of
// it. Recording what the reference implementation says about the same file is
// what makes a disagreement visible rather than invisible.
type ReferenceDecoder struct {
	// Ran is false when the tool was not installed, which is not a fault in
	// the file.
	Ran bool `json:"ran"`
	// OK is true when it read the file without complaint.
	OK bool `json:"ok"`
	// Output is what it said.
	Output string `json:"output,omitempty"`
}

// Encoding counts how the stream was compressed.
//
// FLAC lets an encoder choose how to predict each subframe, and encoders differ
// in what they reach for and how hard they look. The distribution is therefore
// a fingerprint of the program that wrote the file, which is worth having when
// the vendor string has been stripped by a tagger - as it has on most files
// that have been through anything.
//
// Nothing here names an encoder. The counts are recorded; which encoder they
// resemble is the reader's business.
type Encoding struct {
	// Constant, Verbatim, Fixed and LPC count subframes by prediction method.
	// Verbatim means the encoder stored samples untouched, which a well-run
	// encoder rarely needs to do.
	Constant int64 `json:"constant"`
	Verbatim int64 `json:"verbatim"`
	Fixed    int64 `json:"fixed"`
	LPC      int64 `json:"lpc"`
	// FixedOrders and LPCOrders count how often each prediction order was
	// chosen, keyed by order.
	FixedOrders map[string]int64 `json:"fixed_orders"`
	LPCOrders   map[string]int64 `json:"lpc_orders"`
	// WastedBits counts subframes whose samples shared trailing zero bits.
	WastedBits int64 `json:"wasted_bits_subframes"`
	// Subframes is the total counted.
	Subframes int64 `json:"subframes"`
}

// TruePeak reports the level the waveform reaches between samples.
//
// The sample peak in levels is the highest value stored; this is the highest
// the reconstructed waveform passes through, which is what a converter sees.
// A signal can sit below full scale at every sample and still exceed it in
// between, so this figure can be above zero and on a loud master usually is.
type TruePeak struct {
	// ChannelsDBTP is the true peak of each channel, in dBTP.
	ChannelsDBTP []Number `json:"channels_dbtp"`
	// Oversample is the factor the waveform was reconstructed at.
	Oversample int `json:"oversample"`
}

// ChannelLevels holds the amplitude measurements for one channel.
type ChannelLevels struct {
	PeakSample     int32  `json:"peak_sample"`
	PeakDBFS       Number `json:"peak_dbfs"`
	RMSDBFS        Number `json:"rms_dbfs"`
	DCOffset       Number `json:"dc_offset"`
	ClippedSamples uint64 `json:"clipped_samples"`
}

// ChannelResolution holds how much of the declared resolution one channel uses.
type ChannelResolution struct {
	UnusedLowBits int  `json:"unused_low_bits"`
	EffectiveBits int  `json:"effective_bits"`
	AllZero       bool `json:"all_zero"`
}

// Resolution reports the declared resolution against the one actually carried.
type Resolution struct {
	DeclaredBits  int                 `json:"declared_bits"`
	EffectiveBits int                 `json:"effective_bits"`
	Channels      []ChannelResolution `json:"channels"`
}

// Stereo reports how alike the first two channels are.
type Stereo struct {
	Correlation      Number `json:"correlation"`
	IdenticalSamples uint64 `json:"identical_samples"`
	TotalSamples     uint64 `json:"total_samples"`
}

// Silence reports runs of digital silence at each end of the stream.
type Silence struct {
	LeadingSamples  uint64 `json:"leading_samples"`
	TrailingSamples uint64 `json:"trailing_samples"`
	AllSilent       bool   `json:"all_silent"`
}

// Spectrum describes the analysis behind the image written alongside, and
// carries the whole-track average so the frequency content can be queried
// without opening the picture.
type Spectrum struct {
	FFTSize         int    `json:"fft_size"`
	Window          string `json:"window"`
	HopSamples      int    `json:"hop_samples"`
	Frames          int64  `json:"frames"`
	FramesPerColumn int64  `json:"frames_per_column"`
	Columns         int    `json:"columns"`
	Bins            int    `json:"bins"`
	BinHz           Number `json:"bin_hz"`
	// AverageDBFS is the mean level of each frequency bin across the whole
	// track, in decibels relative to full scale, from lowest bin to highest.
	AverageDBFS []Number `json:"average_dbfs"`
	// Channels summarises each channel separately, unmixed. AverageDBFS
	// collapses the channels together, which hides content present in only
	// one of them.
	Channels []ChannelSpectrum `json:"channels,omitempty"`
}

// ChannelSpectrum is one channel's frequency content over a whole track.
//
// Mean and peak answer different questions. Content present for the whole
// track shows in both. Content present only in bursts is averaged away in the
// mean and survives only in the peak, so the two together say whether a band
// is continuously occupied or occasionally reached.
type ChannelSpectrum struct {
	// MeanDBFS is the average level of each bin across every frame.
	MeanDBFS []Number `json:"mean_dbfs"`
	// PeakDBFS is the highest level each bin ever reached in any one frame.
	PeakDBFS []Number `json:"peak_dbfs"`
}

// Settings records the configuration that produced the numbers, so that a
// report read later can be understood without guessing.
type Settings struct {
	Spectrum measure.SpectrumOptions `json:"spectrum"`
	Image    ImageSettings           `json:"image"`
}

// ImageSettings mirrors the rendering options.
type ImageSettings struct {
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	MaxHz     float64   `json:"max_hz"`
	RangeDB   float64   `json:"range_db"`
	Palette   string    `json:"palette"`
	Gridlines []float64 `json:"gridlines_hz"`
}

// Report is the whole document written beside a track.
type Report struct {
	Note       string          `json:"_note"`
	Tool       Tool            `json:"tool"`
	Generated  time.Time       `json:"generated"`
	File       File            `json:"file"`
	Stream     Stream          `json:"stream"`
	Tags       Tags            `json:"tags"`
	Decode     Decode          `json:"decode"`
	Integrity  Integrity       `json:"integrity"`
	Levels     []ChannelLevels `json:"levels"`
	Loudness   Loudness        `json:"loudness"`
	TruePeak   TruePeak        `json:"true_peak"`
	Encoding   Encoding        `json:"encoding"`
	Provenance Provenance      `json:"provenance"`
	Resolution Resolution      `json:"resolution"`
	// Stereo is absent for streams with fewer than two channels.
	Stereo  *Stereo `json:"stereo,omitempty"`
	Silence Silence `json:"silence"`
	// Spectrum is absent when the spectral analysis was skipped.
	Spectrum *Spectrum `json:"spectrum,omitempty"`
	Settings Settings  `json:"settings"`
}

// Input is everything needed to assemble a report.
type Input struct {
	Path     string
	Size     int64
	Modified time.Time
	Tool     Tool
	Metadata *audio.Metadata
	Decode   audio.DecodeResult
	// DecodeErr is why the decode stopped early, nil when it did not.
	DecodeErr error
	// DecodeStage names where it stopped, empty when it did not.
	DecodeStage string
	// FileSHA256, Verify and Fingerprint come from the stages that read the
	// file rather than decode it.
	FileSHA256  string
	Verify      audio.VerifyResult
	Fingerprint audio.FingerprintResult
	Spectrum    measure.SpectrumOptions
	Image       spectrogram.Options
	Now         time.Time
}

// New assembles a report from one file's measurements.
func New(in Input) (*Report, error) {
	if in.Metadata == nil {
		return nil, fmt.Errorf("report: metadata is nil")
	}

	info := in.Metadata.StreamInfo
	results := in.Decode.Results

	r := &Report{
		Note:      Note,
		Tool:      in.Tool,
		Generated: in.Now,
		File: File{
			Path:     in.Path,
			Size:     in.Size,
			Modified: in.Modified,
		},
		Stream: Stream{
			SampleRate:    info.SampleRate,
			Channels:      info.Channels,
			BitsPerSample: info.BitsPerSample,
			TotalSamples:  info.TotalSamples,
			DurationSecs:  Number(info.Duration()),
			BitrateKbps:   Number(bitrateKbps(in.Size, info.Duration())),
			BlockSizeMin:  info.BlockSizeMin,
			BlockSizeMax:  info.BlockSizeMax,
			FrameSizeMin:  info.FrameSizeMin,
			FrameSizeMax:  info.FrameSizeMax,
			Vendor:        in.Metadata.Vendor,
			AudioOffset:   in.Metadata.AudioOffset,
		},
		Tags: Tags{
			HasVorbisComment:  in.Metadata.HasVorbisComment,
			Comments:          orEmptyPairs(in.Metadata.Tags),
			SeekPoints:        in.Metadata.SeekPoints,
			HasCueSheet:       in.Metadata.HasCueSheet,
			Pictures:          in.Metadata.Pictures,
			ApplicationBlocks: in.Metadata.ApplicationBlocks,
			PaddingBytes:      in.Metadata.PaddingBytes,
			ForeignTags:       orEmptyStrings(in.Metadata.ForeignTags),
		},
		Decode: Decode{
			Complete: in.DecodeErr == nil,
			Error:    errorText(in.DecodeErr, in.Path),
			Stage:    in.DecodeStage,
		},
		Integrity: Integrity{
			StoredMD5:        info.MD5,
			ComputedMD5:      results.Fingerprint.MD5,
			StoredMD5Present: info.MD5Present,
			Match:            info.MD5Present && info.MD5 == results.Fingerprint.MD5,
			SamplesDecoded:   results.Fingerprint.Samples,
			FramesDecoded:    in.Decode.Frames,
		},
		Resolution: Resolution{
			DeclaredBits:  results.Bits.DeclaredBits,
			EffectiveBits: results.Bits.EffectiveBits,
		},
		Silence: Silence{
			LeadingSamples:  results.Silence.LeadingSamples,
			TrailingSamples: results.Silence.TrailingSamples,
			AllSilent:       results.Silence.AllSilent,
		},
		TruePeak: truePeak(results.TruePeak),
		Encoding: encoding(in.Decode.Subframes),
		Provenance: Provenance{
			FileSHA256: in.FileSHA256,
			ReferenceDecoder: ReferenceDecoder{
				Ran:    in.Verify.Ran,
				OK:     in.Verify.OK,
				Output: in.Verify.Output,
			},
			AcoustID:         in.Fingerprint.Fingerprint,
			AcoustIDDuration: in.Fingerprint.Duration,
		},
		Loudness: Loudness{
			IntegratedLUFS: Number(results.Loudness.IntegratedLUFS),
			RangeLU:        Number(results.Loudness.RangeLU),
			Blocks:         results.Loudness.Blocks,
			GatedBlocks:    results.Loudness.GatedBlocks,
		},
		Settings: Settings{
			Spectrum: in.Spectrum,
			Image:    imageSettings(in.Image),
		},
	}

	for _, ch := range results.Levels.Channels {
		r.Levels = append(r.Levels, ChannelLevels{
			PeakSample:     ch.PeakSample,
			PeakDBFS:       Number(ch.PeakDBFS),
			RMSDBFS:        Number(ch.RMSDBFS),
			DCOffset:       Number(ch.DCOffset),
			ClippedSamples: ch.ClippedSamples,
		})
	}
	for _, ch := range results.Bits.Channels {
		r.Resolution.Channels = append(r.Resolution.Channels, ChannelResolution{
			UnusedLowBits: ch.UnusedLowBits,
			EffectiveBits: ch.EffectiveBits,
			AllZero:       ch.AllZero,
		})
	}
	if results.Stereo.Applicable {
		r.Stereo = &Stereo{
			Correlation:      Number(results.Stereo.Correlation),
			IdenticalSamples: results.Stereo.IdenticalSamples,
			TotalSamples:     results.Stereo.TotalSamples,
		}
	}
	if results.Spectrum != nil {
		r.Spectrum = summariseSpectrum(results.Spectrum)
	}
	return r, nil
}

// summariseSpectrum records the analysis parameters and the whole-track
// average of every frequency bin.
func summariseSpectrum(g *measure.Grid) *Spectrum {
	out := &Spectrum{
		FFTSize:         g.FFTSize,
		Window:          g.Window,
		HopSamples:      g.Hop,
		Frames:          g.Frames,
		FramesPerColumn: g.FramesPerColumn,
		Columns:         g.Columns(),
		Bins:            g.Bins,
		BinHz:           Number(g.BinHz()),
		AverageDBFS:     make([]Number, g.Bins),
	}

	columns := g.Columns()
	for bin := range g.Bins {
		var sum float64
		for _, column := range g.Power {
			if bin < len(column) {
				sum += float64(column[bin])
			}
		}
		mean := 0.0
		if columns > 0 {
			mean = sum / float64(columns)
		}
		out.AverageDBFS[bin] = roundDB(measure.PowerDB(mean))
	}

	out.Channels = make([]ChannelSpectrum, len(g.Channels))
	for c, ch := range g.Channels {
		out.Channels[c] = ChannelSpectrum{
			MeanDBFS: powerColumnDB(ch.Mean, g.Bins),
			PeakDBFS: powerColumnDB(ch.Peak, g.Bins),
		}
	}
	return out
}

// powerColumnDB converts a run of linear bin powers into decibels, padded or
// truncated to the grid's bin count so every channel is the same length as
// AverageDBFS and can be indexed alongside it.
func powerColumnDB(power []float64, bins int) []Number {
	out := make([]Number, bins)
	for bin := range bins {
		v := 0.0
		if bin < len(power) {
			v = power[bin]
		}
		out[bin] = roundDB(measure.PowerDB(v))
	}
	return out
}

// imageSettings flattens the rendering options into their recorded form.
func imageSettings(opts spectrogram.Options) ImageSettings {
	out := ImageSettings{
		Width:     opts.Width,
		Height:    opts.Height,
		MaxHz:     opts.MaxHz,
		RangeDB:   opts.RangeDB,
		Palette:   opts.Palette,
		Gridlines: []float64{},
	}
	for _, line := range opts.Gridlines {
		out.Gridlines = append(out.Gridlines, line.Hz)
	}
	return out
}

// bitrateKbps returns the file's average bitrate, or zero when the duration is
// unknown.
func bitrateKbps(size int64, seconds float64) float64 {
	if seconds <= 0 {
		return 0
	}
	return float64(size) * 8 / seconds / 1000
}

// orEmptyPairs replaces a nil slice with an empty one, so that the field
// marshals as [] rather than null.
func orEmptyPairs(in [][2]string) [][2]string {
	if in == nil {
		return [][2]string{}
	}
	return in
}

// orEmptyStrings replaces a nil slice with an empty one.
func orEmptyStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// errorText renders an error for the report, empty when there is none.
//
// The file's own path is taken back out of the message. The decoder puts it in
// front of everything it returns, and a report already says which file it is
// about - leaving it in would make every error string unique to its file, so
// grouping a library's reports by what went wrong would find nothing in
// common between two copies of the same fault.
func errorText(err error, path string) string {
	if err == nil {
		return ""
	}

	text := err.Error()
	if path != "" {
		if _, after, found := strings.Cut(text, path+": "); found {
			return after
		}
	}
	return text
}

// truePeak converts the measured true peak for the report.
func truePeak(r measure.TruePeakResult) TruePeak {
	out := TruePeak{Oversample: r.Oversample}
	for _, dbtp := range r.ChannelsDBTP {
		out.ChannelsDBTP = append(out.ChannelsDBTP, Number(dbtp))
	}
	return out
}

// encoding converts the subframe statistics for the report.
func encoding(s *audio.SubframeStats) Encoding {
	if s == nil {
		return Encoding{FixedOrders: map[string]int64{}, LPCOrders: map[string]int64{}}
	}

	return Encoding{
		Constant:    s.Constant,
		Verbatim:    s.Verbatim,
		Fixed:       s.Fixed,
		LPC:         s.LPC,
		FixedOrders: orders(s.FixedOrders),
		LPCOrders:   orders(s.LPCOrders),
		WastedBits:  s.WastedBits,
		Subframes:   s.Subframes,
	}
}

// orders renders an order histogram with string keys, since JSON objects have
// no others.
func orders(counts map[int]int64) map[string]int64 {
	out := make(map[string]int64, len(counts))
	for order, n := range counts {
		out[strconv.Itoa(order)] = n
	}
	return out
}
