package measure

import (
	"math"
	"testing"
)

// sineBlock builds one block of a full-scale sine wave whose frequency sits
// exactly on the given bin of a transform of length fftSize.
func sineBlock(t *testing.T, format Format, fftSize, bin, samples int) Block {
	t.Helper()
	amplitude := format.FullScale() - 1
	out := make([]int32, samples)
	for i := range out {
		phase := 2 * math.Pi * float64(bin) * float64(i) / float64(fftSize)
		out[i] = int32(math.Round(amplitude * math.Sin(phase)))
	}
	channels := make([][]int32, format.Channels)
	for c := range channels {
		channels[c] = out
	}
	return Block{Samples: channels}
}

func testSpectrumOptions() SpectrumOptions {
	return SpectrumOptions{
		FFTSize:       64,
		ReferenceRate: 44100,
		Window:        WindowRectangular,
		Overlap:       0,
		Columns:       8,
	}
}

func TestSpectrumOptionsValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*SpectrumOptions)
		wantErr bool
	}{
		{"defaults", func(*SpectrumOptions) {}, false},
		{"fft size not a power of two", func(o *SpectrumOptions) { o.FFTSize = 1000 }, true},
		{"fft size too small", func(o *SpectrumOptions) { o.FFTSize = 1 }, true},
		{"reference rate zero", func(o *SpectrumOptions) { o.ReferenceRate = 0 }, true},
		{"overlap negative", func(o *SpectrumOptions) { o.Overlap = -0.1 }, true},
		{"overlap of one leaves no advance", func(o *SpectrumOptions) { o.Overlap = 1 }, true},
		{"columns zero", func(o *SpectrumOptions) { o.Columns = 0 }, true},
		{"unknown window", func(o *SpectrumOptions) { o.Window = "kaiser" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultSpectrumOptions()
			tt.mutate(&opts)
			if err := opts.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSpectrumOptionsSizeFor covers the rule that keeps a frequency bin the
// same width in Hz whatever the file's sample rate.
func TestSpectrumOptionsSizeFor(t *testing.T) {
	opts := DefaultSpectrumOptions()
	tests := []struct {
		rate int
		want int
	}{
		{44100, 2048},
		{48000, 2048},
		{88200, 4096},
		{96000, 4096},
		{176400, 8192},
		{192000, 8192},
		{22050, 1024},
		// Rounding is geometric, so 371 lands nearer to 512 than to 256.
		{8000, 512},
	}
	for _, tt := range tests {
		t.Run(t.Name(), func(t *testing.T) {
			if got := opts.SizeFor(tt.rate); got != tt.want {
				t.Errorf("SizeFor(%d) = %d, want %d", tt.rate, got, tt.want)
			}
		})
	}
}

func TestSpectrumOptionsSizeForIsClamped(t *testing.T) {
	opts := DefaultSpectrumOptions()

	if got := opts.SizeFor(1); got < 64 {
		t.Errorf("SizeFor(1) = %d, want at least 64", got)
	}
	if got := opts.SizeFor(1 << 24); got > 1<<16 {
		t.Errorf("SizeFor(huge) = %d, want at most 65536", got)
	}
}

func TestNewSpectrumRejectsBadInput(t *testing.T) {
	if _, err := NewSpectrum(Format{}, DefaultSpectrumOptions()); err == nil {
		t.Error("NewSpectrum() with zero format succeeded, want error")
	}

	opts := DefaultSpectrumOptions()
	opts.Columns = 0
	if _, err := NewSpectrum(testFormat(), opts); err == nil {
		t.Error("NewSpectrum() with zero columns succeeded, want error")
	}
}

func TestSpectrumName(t *testing.T) {
	a, err := NewSpectrum(testFormat(), DefaultSpectrumOptions())
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	if got := a.Name(); got != "spectrum" {
		t.Errorf("Name() = %q, want %q", got, "spectrum")
	}
}

// TestSpectrumLocatesAFullScaleSine is the load-bearing test for the whole
// package: a known tone must land in the right bin, at the right level, with
// the frequency axis reported correctly.
func TestSpectrumLocatesAFullScaleSine(t *testing.T) {
	const (
		fftSize = 64
		bin     = 8
	)
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}

	a, err := NewSpectrum(format, testSpectrumOptions())
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	a.Write(sineBlock(t, format, fftSize, bin, fftSize*4))
	grid := a.Result()

	if grid.Columns() == 0 {
		t.Fatal("grid has no columns")
	}
	if grid.Bins != fftSize/2+1 {
		t.Errorf("Bins = %d, want %d", grid.Bins, fftSize/2+1)
	}

	column := grid.Power[0]
	peak := 0
	for k, p := range column {
		if p > column[peak] {
			peak = k
		}
	}
	if peak != bin {
		t.Fatalf("peak at bin %d, want %d", peak, bin)
	}
	if db := PowerDB(float64(column[peak])); !closeTo(db, 0, 0.05) {
		t.Errorf("peak level = %v dB, want approximately 0", db)
	}

	wantHz := float64(bin) * float64(format.SampleRate) / float64(fftSize)
	if got := grid.BinFrequency(peak); !closeTo(got, wantHz, 1e-9) {
		t.Errorf("BinFrequency(%d) = %v, want %v", peak, got, wantHz)
	}
}

func TestSpectrumSilenceHasNoPower(t *testing.T) {
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	a, err := NewSpectrum(format, testSpectrumOptions())
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	a.Write(Block{Samples: [][]int32{make([]int32, 256)}})
	grid := a.Result()

	for c, column := range grid.Power {
		for k, p := range column {
			if p != 0 {
				t.Fatalf("column %d bin %d = %v, want 0", c, k, p)
			}
		}
	}
}

// TestSpectrumBoundsMemoryForLongStreams checks the halving strategy: however
// many frames arrive, the grid never exceeds its configured column count.
func TestSpectrumBoundsMemoryForLongStreams(t *testing.T) {
	const columns = 8
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	opts := testSpectrumOptions()
	opts.Columns = columns

	a, err := NewSpectrum(format, opts)
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	// Two hundred frames into eight columns forces repeated halving.
	a.Write(sineBlock(t, format, 64, 8, 64*200))
	grid := a.Result()

	if grid.Columns() > columns {
		t.Errorf("Columns() = %d, want at most %d", grid.Columns(), columns)
	}
	if grid.Frames != 200 {
		t.Errorf("Frames = %d, want 200", grid.Frames)
	}
	if grid.FramesPerColumn < 1 {
		t.Errorf("FramesPerColumn = %d, want at least 1", grid.FramesPerColumn)
	}
	if grid.FramesPerColumn&(grid.FramesPerColumn-1) != 0 {
		t.Errorf("FramesPerColumn = %d, want a power of two", grid.FramesPerColumn)
	}
}

// TestSpectrumHalvingPreservesContent checks that merging columns keeps the
// measured level intact rather than accumulating without dividing.
func TestSpectrumHalvingPreservesContent(t *testing.T) {
	const (
		fftSize = 64
		bin     = 8
	)
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	opts := testSpectrumOptions()
	opts.Columns = 4

	a, err := NewSpectrum(format, opts)
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	a.Write(sineBlock(t, format, fftSize, bin, fftSize*32))
	grid := a.Result()

	for c, column := range grid.Power {
		if db := PowerDB(float64(column[bin])); !closeTo(db, 0, 0.05) {
			t.Errorf("column %d peak = %v dB, want approximately 0", c, db)
		}
	}
}

func TestSpectrumShortStreamIsZeroPadded(t *testing.T) {
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	a, err := NewSpectrum(format, testSpectrumOptions())
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	// Fewer samples than one transform: without padding there would be no
	// frames at all and the grid would be empty.
	a.Write(Block{Samples: [][]int32{{1000, -1000, 2000, -2000}}})
	grid := a.Result()

	if grid.Columns() != 1 {
		t.Errorf("Columns() = %d, want 1", grid.Columns())
	}
	if grid.Frames != 1 {
		t.Errorf("Frames = %d, want 1", grid.Frames)
	}
}

func TestSpectrumIgnoresWritesAfterResult(t *testing.T) {
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	a, err := NewSpectrum(format, testSpectrumOptions())
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	a.Write(sineBlock(t, format, 64, 8, 64))

	before := a.Result().Frames
	a.Write(sineBlock(t, format, 64, 8, 640))
	if after := a.Result().Frames; after != before {
		t.Errorf("Frames = %d after a late write, want %d", after, before)
	}
}

func TestSpectrumOverlapAdvancesByHop(t *testing.T) {
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	opts := testSpectrumOptions()
	opts.Overlap = 0.5
	opts.Columns = 64

	a, err := NewSpectrum(format, opts)
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	if got := a.Hop(); got != 32 {
		t.Fatalf("Hop() = %d, want 32", got)
	}
	if got := a.FFTSize(); got != 64 {
		t.Fatalf("FFTSize() = %d, want 64", got)
	}

	// With a hop of 32, 320 samples yield frames starting at 0,32,...,256.
	a.Write(sineBlock(t, format, 64, 8, 320))
	if got := a.Result().Frames; got != 9 {
		t.Errorf("Frames = %d, want 9", got)
	}
}

func TestSpectrumIsIndependentOfBlocking(t *testing.T) {
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	block := sineBlock(t, format, 64, 8, 256)

	whole, err := NewSpectrum(format, testSpectrumOptions())
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	whole.Write(block)

	split, err := NewSpectrum(format, testSpectrumOptions())
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	for _, chunk := range [][]int32{
		block.Samples[0][:17],
		block.Samples[0][17:100],
		block.Samples[0][100:],
	} {
		split.Write(Block{Samples: [][]int32{chunk}})
	}

	a, b := whole.Result(), split.Result()
	if a.Frames != b.Frames {
		t.Fatalf("Frames = %d then %d", a.Frames, b.Frames)
	}
	for c := range a.Power {
		for k := range a.Power[c] {
			if !closeTo(float64(a.Power[c][k]), float64(b.Power[c][k]), 1e-6) {
				t.Fatalf("column %d bin %d = %v then %v", c, k, a.Power[c][k], b.Power[c][k])
			}
		}
	}
}

func TestSpectrumMixesChannelsToMono(t *testing.T) {
	format := Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}
	opts := testSpectrumOptions()

	// Two identical channels must measure the same as one, since mixing takes
	// the mean rather than the sum.
	stereo, err := NewSpectrum(format, opts)
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	stereo.Write(sineBlock(t, format, 64, 8, 256))

	mono, err := NewSpectrum(Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}, opts)
	if err != nil {
		t.Fatalf("NewSpectrum() error = %v", err)
	}
	mono.Write(sineBlock(t, Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}, 64, 8, 256))

	got := PowerDB(float64(stereo.Result().Power[0][8]))
	want := PowerDB(float64(mono.Result().Power[0][8]))
	if !closeTo(got, want, 1e-6) {
		t.Errorf("stereo peak = %v dB, mono peak = %v dB, want them equal", got, want)
	}
}

func TestGridHelpers(t *testing.T) {
	g := Grid{SampleRate: 44100, FFTSize: 1024, Power: make([][]float32, 3)}

	if got := g.Columns(); got != 3 {
		t.Errorf("Columns() = %d, want 3", got)
	}
	if want := 44100.0 / 1024; !closeTo(g.BinHz(), want, 1e-9) {
		t.Errorf("BinHz() = %v, want %v", g.BinHz(), want)
	}
	if want := 10 * 44100.0 / 1024; !closeTo(g.BinFrequency(10), want, 1e-9) {
		t.Errorf("BinFrequency(10) = %v, want %v", g.BinFrequency(10), want)
	}

	empty := Grid{}
	if got := empty.BinHz(); got != 0 {
		t.Errorf("BinHz() on a zero grid = %v, want 0", got)
	}
}

func TestPowerDB(t *testing.T) {
	tests := []struct {
		name  string
		power float64
		want  float64
	}{
		{"full scale", 1, 0},
		{"ten decibels down", 0.1, -10},
		{"twenty decibels down", 0.01, -20},
		{"zero maps to negative infinity", 0, math.Inf(-1)},
		{"negative maps to negative infinity", -1, math.Inf(-1)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PowerDB(tt.power); !closeTo(got, tt.want, 1e-9) {
				t.Errorf("PowerDB(%v) = %v, want %v", tt.power, got, tt.want)
			}
		})
	}
}
