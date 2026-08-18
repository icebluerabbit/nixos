// Package measure computes numeric properties of decoded audio.
//
// Everything here answers questions of the form "what is the value of X".
// Nothing in this package knows about thresholds, reference tables, formats or
// verdicts. Interpreting the numbers is deliberately left to whoever reads the
// report.
package measure

import (
	"fmt"
	"math"
)

// Format describes the stream properties an Accumulator needs in order to
// interpret raw sample values.
type Format struct {
	// SampleRate in Hz.
	SampleRate int
	// Channels is the number of channels carried by each Block.
	Channels int
	// BitsPerSample is the declared sample resolution.
	BitsPerSample int
}

// Validate reports whether f describes a stream that can be measured.
func (f Format) Validate() error {
	switch {
	case f.SampleRate <= 0:
		return fmt.Errorf("measure: sample rate must be positive, got %d", f.SampleRate)
	case f.Channels <= 0:
		return fmt.Errorf("measure: channel count must be positive, got %d", f.Channels)
	case f.BitsPerSample < 4 || f.BitsPerSample > 32:
		return fmt.Errorf("measure: bits per sample must be within [4,32], got %d", f.BitsPerSample)
	}
	return nil
}

// FullScale returns the sample magnitude corresponding to 0 dBFS.
func (f Format) FullScale() float64 {
	return math.Ldexp(1, f.BitsPerSample-1)
}

// BytesPerSample returns the number of bytes FLAC uses to represent one sample
// when hashing unencoded audio: the declared resolution rounded up to a whole
// number of bytes.
func (f Format) BytesPerSample() int { return (f.BitsPerSample + 7) / 8 }

// Nyquist returns the highest frequency the stream can represent, in Hz.
func (f Format) Nyquist() float64 { return float64(f.SampleRate) / 2 }

// Block is a chunk of decoded audio in stream order: one slice of samples per
// channel, every slice holding the same number of samples. Sample values are
// sign-extended into int32 regardless of the declared resolution.
type Block struct {
	Samples [][]int32
}

// Channels returns the number of channels in the block.
func (b Block) Channels() int { return len(b.Samples) }

// Len returns the number of samples per channel, or zero for an empty block.
func (b Block) Len() int {
	if len(b.Samples) == 0 {
		return 0
	}
	return len(b.Samples[0])
}

// Accumulator consumes decoded audio in stream order and maintains a running
// measurement.
//
// Write is called once per block, in order, and must not retain the block: the
// decoder reuses its backing arrays.
type Accumulator interface {
	// Name identifies the measurement in diagnostics.
	Name() string
	// Write folds one block of samples into the running measurement.
	Write(Block)
}

// Options configures the accumulators built by NewSet.
type Options struct {
	// Spectrum configures the spectrum accumulator.
	Spectrum SpectrumOptions
	// SkipSpectrum omits the spectrum accumulator, which costs more than all
	// the others combined.
	SkipSpectrum bool
}

// Set is the collection of accumulators run over a single stream. It exists so
// that a decoder can hand every block to one place, and so that adding a new
// measurement is a change in exactly one function.
type Set struct {
	format Format
	all    []Accumulator

	Fingerprint *Fingerprint
	Levels      *Levels
	Bits        *Bits
	Stereo      *Stereo
	Silence     *Silence
	Loudness    *Loudness
	TruePeak    *TruePeak
	// Spectrum is nil when Options.SkipSpectrum was set.
	Spectrum *Spectrum
}

// NewSet builds the full set of accumulators for a stream of the given format.
func NewSet(f Format, opts Options) (*Set, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	s := &Set{
		format:      f,
		Fingerprint: NewFingerprint(f),
		Levels:      NewLevels(f),
		Bits:        NewBits(f),
		Stereo:      NewStereo(f),
		Silence:     NewSilence(f),
		Loudness:    NewLoudness(f),
		TruePeak:    NewTruePeak(f),
	}
	s.all = []Accumulator{
		s.Fingerprint, s.Levels, s.Bits, s.Stereo, s.Silence,
		s.Loudness, s.TruePeak,
	}

	if !opts.SkipSpectrum {
		spectrum, err := NewSpectrum(f, opts.Spectrum)
		if err != nil {
			return nil, err
		}
		s.Spectrum = spectrum
		s.all = append(s.all, spectrum)
	}
	return s, nil
}

// Format returns the stream format the set was built for.
func (s *Set) Format() Format { return s.format }

// Accumulators returns the members of the set in a stable order.
func (s *Set) Accumulators() []Accumulator { return s.all }

// Write folds one block into every accumulator in the set.
func (s *Set) Write(b Block) {
	if b.Len() == 0 {
		return
	}
	for _, a := range s.all {
		a.Write(b)
	}
}

// Results collects the final value of every accumulator in the set.
type Results struct {
	Format      Format
	Fingerprint FingerprintResult
	Levels      LevelsResult
	Bits        BitsResult
	Stereo      StereoResult
	Silence     SilenceResult
	Loudness    LoudnessResult
	TruePeak    TruePeakResult
	// Spectrum is nil when the spectrum accumulator was skipped.
	Spectrum *Grid
}

// Results finalises every accumulator and returns their values. It is safe to
// call more than once; accumulators are not consumed.
func (s *Set) Results() Results {
	r := Results{
		Format:      s.format,
		Fingerprint: s.Fingerprint.Result(),
		Levels:      s.Levels.Result(),
		Bits:        s.Bits.Result(),
		Stereo:      s.Stereo.Result(),
		Silence:     s.Silence.Result(),
		Loudness:    s.Loudness.Result(),
		TruePeak:    s.TruePeak.Result(),
	}
	if s.Spectrum != nil {
		grid := s.Spectrum.Result()
		r.Spectrum = &grid
	}
	return r
}
