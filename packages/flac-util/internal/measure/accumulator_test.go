package measure

import (
	"math"
	"testing"
)

func testFormat() Format {
	return Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}
}

func TestFormatValidate(t *testing.T) {
	tests := []struct {
		name    string
		format  Format
		wantErr bool
	}{
		{"cd audio", Format{44100, 2, 16}, false},
		{"high resolution", Format{96000, 2, 24}, false},
		{"mono", Format{22050, 1, 8}, false},
		{"zero sample rate", Format{0, 2, 16}, true},
		{"negative sample rate", Format{-1, 2, 16}, true},
		{"zero channels", Format{44100, 0, 16}, true},
		{"resolution too low", Format{44100, 2, 3}, true},
		{"resolution too high", Format{44100, 2, 33}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.format.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatDerivedValues(t *testing.T) {
	tests := []struct {
		name          string
		format        Format
		wantFullScale float64
		wantBytes     int
		wantNyquistHz float64
	}{
		{"16 bit", Format{44100, 2, 16}, 32768, 2, 22050},
		{"24 bit", Format{96000, 2, 24}, 8388608, 3, 48000},
		{"20 bit rounds up to three bytes", Format{48000, 2, 20}, 524288, 3, 24000},
		{"8 bit", Format{8000, 1, 8}, 128, 1, 4000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.format.FullScale(); got != tt.wantFullScale {
				t.Errorf("FullScale() = %v, want %v", got, tt.wantFullScale)
			}
			if got := tt.format.BytesPerSample(); got != tt.wantBytes {
				t.Errorf("BytesPerSample() = %v, want %v", got, tt.wantBytes)
			}
			if got := tt.format.Nyquist(); got != tt.wantNyquistHz {
				t.Errorf("Nyquist() = %v, want %v", got, tt.wantNyquistHz)
			}
		})
	}
}

func TestBlockDimensions(t *testing.T) {
	tests := []struct {
		name         string
		block        Block
		wantChannels int
		wantLen      int
	}{
		{"empty", Block{}, 0, 0},
		{"stereo", Block{Samples: [][]int32{{1, 2, 3}, {4, 5, 6}}}, 2, 3},
		{"mono", Block{Samples: [][]int32{{1}}}, 1, 1},
		{"channel present but empty", Block{Samples: [][]int32{{}, {}}}, 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.block.Channels(); got != tt.wantChannels {
				t.Errorf("Channels() = %d, want %d", got, tt.wantChannels)
			}
			if got := tt.block.Len(); got != tt.wantLen {
				t.Errorf("Len() = %d, want %d", got, tt.wantLen)
			}
		})
	}
}

func TestNewSetRejectsBadInput(t *testing.T) {
	if _, err := NewSet(Format{}, Options{}); err == nil {
		t.Fatal("NewSet() with zero format succeeded, want error")
	}

	opts := Options{Spectrum: DefaultSpectrumOptions()}
	opts.Spectrum.FFTSize = 1000
	if _, err := NewSet(testFormat(), opts); err == nil {
		t.Fatal("NewSet() with non power of two fft size succeeded, want error")
	}
}

func TestNewSetIncludesEveryAccumulator(t *testing.T) {
	set, err := NewSet(testFormat(), Options{Spectrum: DefaultSpectrumOptions()})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}

	want := []string{"fingerprint", "levels", "bits", "stereo", "silence", "spectrum"}
	got := make([]string, 0, len(set.Accumulators()))
	for _, a := range set.Accumulators() {
		got = append(got, a.Name())
	}
	if len(got) != len(want) {
		t.Fatalf("Accumulators() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Accumulators()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewSetSkipSpectrum(t *testing.T) {
	set, err := NewSet(testFormat(), Options{SkipSpectrum: true})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}
	if set.Spectrum != nil {
		t.Error("Spectrum accumulator built despite SkipSpectrum")
	}
	if got := len(set.Accumulators()); got != 5 {
		t.Errorf("len(Accumulators()) = %d, want 5", got)
	}
	if set.Results().Spectrum != nil {
		t.Error("Results().Spectrum non-nil despite SkipSpectrum")
	}
}

func TestSetWriteReachesEveryAccumulator(t *testing.T) {
	set, err := NewSet(testFormat(), Options{SkipSpectrum: true})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}

	set.Write(Block{Samples: [][]int32{{1000, -1000}, {0, 0}}})
	got := set.Results()

	if got.Fingerprint.Samples != 2 {
		t.Errorf("Fingerprint.Samples = %d, want 2", got.Fingerprint.Samples)
	}
	if got.Levels.SampleCount != 2 {
		t.Errorf("Levels.SampleCount = %d, want 2", got.Levels.SampleCount)
	}
	if got.Levels.Channels[0].PeakSample != 1000 {
		t.Errorf("Levels peak = %d, want 1000", got.Levels.Channels[0].PeakSample)
	}
	if got.Bits.DeclaredBits != 16 {
		t.Errorf("Bits.DeclaredBits = %d, want 16", got.Bits.DeclaredBits)
	}
	if !got.Stereo.Applicable {
		t.Error("Stereo.Applicable = false, want true")
	}
	if got.Silence.TotalSamples != 2 {
		t.Errorf("Silence.TotalSamples = %d, want 2", got.Silence.TotalSamples)
	}
	if got.Format != testFormat() {
		t.Errorf("Results.Format = %+v, want %+v", got.Format, testFormat())
	}
}

func TestSetWriteIgnoresEmptyBlocks(t *testing.T) {
	set, err := NewSet(testFormat(), Options{SkipSpectrum: true})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}

	set.Write(Block{})
	set.Write(Block{Samples: [][]int32{{}, {}}})

	if got := set.Results().Levels.SampleCount; got != 0 {
		t.Errorf("SampleCount = %d after empty blocks, want 0", got)
	}
}

func TestSetResultsIsRepeatable(t *testing.T) {
	set, err := NewSet(testFormat(), Options{Spectrum: DefaultSpectrumOptions()})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}
	set.Write(Block{Samples: [][]int32{{1, 2, 3}, {4, 5, 6}}})

	first, second := set.Results(), set.Results()
	if first.Fingerprint != second.Fingerprint {
		t.Errorf("Fingerprint changed between calls: %+v then %+v", first.Fingerprint, second.Fingerprint)
	}
	if first.Spectrum.Columns() != second.Spectrum.Columns() {
		t.Errorf("Spectrum columns changed between calls: %d then %d",
			first.Spectrum.Columns(), second.Spectrum.Columns())
	}
}

// closeTo reports whether got is within tolerance of want, treating matching
// infinities as equal.
func closeTo(got, want, tolerance float64) bool {
	if math.IsInf(want, 0) || math.IsInf(got, 0) {
		return math.IsInf(got, 1) == math.IsInf(want, 1) &&
			math.IsInf(got, -1) == math.IsInf(want, -1)
	}
	return math.Abs(got-want) <= tolerance
}
