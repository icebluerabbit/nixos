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
	"time"

	"flac-util/internal/audio"
	"flac-util/internal/measure"
	"flac-util/internal/spectrogram"
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
	Integrity  Integrity       `json:"integrity"`
	Levels     []ChannelLevels `json:"levels"`
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
	Spectrum measure.SpectrumOptions
	Image    spectrogram.Options
	Now      time.Time
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
		out.AverageDBFS[bin] = Number(measure.PowerDB(mean))
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
