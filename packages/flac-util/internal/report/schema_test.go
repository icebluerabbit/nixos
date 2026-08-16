package report

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"flac-util/internal/audio"
	"flac-util/internal/measure"
	"flac-util/internal/spectrogram"
)

func TestNumberMarshal(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  string
	}{
		{"finite", -6.5, "-6.5"},
		{"zero", 0, "0"},
		{"negative infinity", math.Inf(-1), "null"},
		{"positive infinity", math.Inf(1), "null"},
		{"not a number", math.NaN(), "null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(Number(tt.value))
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("Marshal(%v) = %s, want %s", tt.value, got, tt.want)
			}
		})
	}
}

func TestNumberUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    float64
		wantErr bool
	}{
		{"finite", "-6.5", -6.5, false},
		{"null becomes silence", "null", math.Inf(-1), false},
		{"not a number at all", `"loud"`, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Number
			err := json.Unmarshal([]byte(tt.data), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if float64(got) != tt.want {
				t.Errorf("Unmarshal(%s) = %v, want %v", tt.data, float64(got), tt.want)
			}
		})
	}
}

// testInput builds a complete, realistic Input so individual tests can vary
// one thing at a time.
func testInput(t *testing.T) Input {
	t.Helper()

	format := measure.Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}
	set, err := measure.NewSet(format, measure.Options{Spectrum: measure.DefaultSpectrumOptions()})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}
	samples := make([]int32, 8192)
	for i := range samples {
		samples[i] = int32(1000 * math.Sin(float64(i)/8))
	}
	set.Write(measure.Block{Samples: [][]int32{samples, samples}})

	return Input{
		Path:     "/music/Artist/Album/01 Track.flac",
		Size:     1024 * 1024,
		Modified: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		Tool:     Tool{Name: "flac-util", Version: "0.1.0"},
		Metadata: &audio.Metadata{
			StreamInfo: audio.StreamInfo{
				SampleRate:    44100,
				Channels:      2,
				BitsPerSample: 16,
				TotalSamples:  441000,
				BlockSizeMin:  4096,
				BlockSizeMax:  4096,
				MD5:           "abc123",
				MD5Present:    true,
			},
			Vendor:           "reference libFLAC 1.4.3",
			Tags:             [][2]string{{"ARTIST", "Someone"}},
			HasVorbisComment: true,
			SeekPoints:       12,
			PaddingBytes:     8192,
			ForeignTags:      []string{audio.TagID3v2},
			AudioOffset:      20,
		},
		Decode: audio.DecodeResult{
			Results: set.Results(),
			Frames:  108,
		},
		Spectrum: measure.DefaultSpectrumOptions(),
		Image:    spectrogram.DefaultOptions(),
		Now:      time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC),
	}
}

func TestNewRejectsMissingMetadata(t *testing.T) {
	in := testInput(t)
	in.Metadata = nil
	if _, err := New(in); err == nil {
		t.Fatal("New() with nil metadata succeeded, want error")
	}
}

func TestNewCopiesTheHeader(t *testing.T) {
	r, err := New(testInput(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if r.Note != Note {
		t.Error("Note is missing from the report")
	}
	if r.Tool.Name != "flac-util" || r.Tool.Version != "0.1.0" {
		t.Errorf("Tool = %+v, want flac-util 0.1.0", r.Tool)
	}
	if r.Stream.SampleRate != 44100 || r.Stream.Channels != 2 || r.Stream.BitsPerSample != 16 {
		t.Errorf("Stream = %+v, want 44100/2/16", r.Stream)
	}
	if got := float64(r.Stream.DurationSecs); math.Abs(got-10) > 1e-9 {
		t.Errorf("DurationSecs = %v, want 10", got)
	}
	if r.Stream.Vendor != "reference libFLAC 1.4.3" {
		t.Errorf("Vendor = %q", r.Stream.Vendor)
	}
	if r.Stream.AudioOffset != 20 {
		t.Errorf("AudioOffset = %d, want 20", r.Stream.AudioOffset)
	}
	if r.File.Path != "/music/Artist/Album/01 Track.flac" {
		t.Errorf("File.Path = %q", r.File.Path)
	}
}

func TestNewRecordsIntegrity(t *testing.T) {
	tests := []struct {
		name      string
		stored    string
		present   bool
		wantMatch bool
	}{
		{"checksum agrees", "", true, true},
		{"checksum differs", "0000", true, false},
		{"no checksum stored", "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := testInput(t)
			stored := tt.stored
			if tt.name == "checksum agrees" {
				stored = in.Decode.Results.Fingerprint.MD5
			}
			in.Metadata.StreamInfo.MD5 = stored
			in.Metadata.StreamInfo.MD5Present = tt.present

			r, err := New(in)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			if r.Integrity.Match != tt.wantMatch {
				t.Errorf("Match = %v, want %v", r.Integrity.Match, tt.wantMatch)
			}
			if r.Integrity.ComputedMD5 != in.Decode.Results.Fingerprint.MD5 {
				t.Errorf("ComputedMD5 = %q", r.Integrity.ComputedMD5)
			}
			if r.Integrity.FramesDecoded != 108 {
				t.Errorf("FramesDecoded = %d, want 108", r.Integrity.FramesDecoded)
			}
		})
	}
}

func TestNewOmitsStereoForMono(t *testing.T) {
	format := measure.Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}
	set, err := measure.NewSet(format, measure.Options{SkipSpectrum: true})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}
	set.Write(measure.Block{Samples: [][]int32{{1, 2, 3}}})

	in := testInput(t)
	in.Metadata.StreamInfo.Channels = 1
	in.Decode.Results = set.Results()

	r, err := New(in)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if r.Stereo != nil {
		t.Errorf("Stereo = %+v, want it omitted for a mono stream", r.Stereo)
	}
	if r.Spectrum != nil {
		t.Error("Spectrum is present despite the spectrum being skipped")
	}
}

func TestNewSummarisesTheSpectrum(t *testing.T) {
	r, err := New(testInput(t))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	s := r.Spectrum
	if s == nil {
		t.Fatal("Spectrum is nil, want a summary")
	}
	if s.Bins != s.FFTSize/2+1 {
		t.Errorf("Bins = %d, want %d", s.Bins, s.FFTSize/2+1)
	}
	if len(s.AverageDBFS) != s.Bins {
		t.Errorf("len(AverageDBFS) = %d, want %d", len(s.AverageDBFS), s.Bins)
	}
	if s.Window != measure.DefaultWindow {
		t.Errorf("Window = %q, want %q", s.Window, measure.DefaultWindow)
	}
	if s.Frames == 0 {
		t.Error("Frames = 0, want the analysed frame count")
	}
}

func TestSummariseSpectrumAveragesAcrossColumns(t *testing.T) {
	grid := &measure.Grid{
		Power:      [][]float32{{1, 0}, {0, 0}},
		Bins:       2,
		SampleRate: 4000,
		FFTSize:    4,
	}

	got := summariseSpectrum(grid)
	if len(got.AverageDBFS) != 2 {
		t.Fatalf("len(AverageDBFS) = %d, want 2", len(got.AverageDBFS))
	}
	// One column at full scale and one silent averages to half power.
	if want := measure.PowerDB(0.5); math.Abs(float64(got.AverageDBFS[0])-want) > 1e-9 {
		t.Errorf("AverageDBFS[0] = %v, want %v", float64(got.AverageDBFS[0]), want)
	}
	if !math.IsInf(float64(got.AverageDBFS[1]), -1) {
		t.Errorf("AverageDBFS[1] = %v, want -Inf", float64(got.AverageDBFS[1]))
	}
}

func TestBitrateKbps(t *testing.T) {
	tests := []struct {
		name    string
		size    int64
		seconds float64
		want    float64
	}{
		{"one megabit per second", 125000, 1, 1000},
		{"unknown duration", 1000, 0, 0},
		{"negative duration", 1000, -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bitrateKbps(tt.size, tt.seconds); math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("bitrateKbps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageSettingsFlattensGridlines(t *testing.T) {
	opts := spectrogram.DefaultOptions()
	got := imageSettings(opts)

	if len(got.Gridlines) != len(opts.Gridlines) {
		t.Fatalf("len(Gridlines) = %d, want %d", len(got.Gridlines), len(opts.Gridlines))
	}
	for i, line := range opts.Gridlines {
		if got.Gridlines[i] != line.Hz {
			t.Errorf("Gridlines[%d] = %v, want %v", i, got.Gridlines[i], line.Hz)
		}
	}
	if got.Palette != opts.Palette {
		t.Errorf("Palette = %q, want %q", got.Palette, opts.Palette)
	}
}

func TestImageSettingsWithoutGridlinesMarshalsAsEmptyList(t *testing.T) {
	opts := spectrogram.DefaultOptions()
	opts.Gridlines = nil

	data, err := json.Marshal(imageSettings(opts))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(data), `"gridlines_hz":[]`) {
		t.Errorf("marshalled as %s, want an empty list rather than null", data)
	}
}

// TestReportMarshalsCleanly is the guard against a measurement that JSON
// cannot represent slipping into the schema unnoticed.
func TestReportMarshalsCleanly(t *testing.T) {
	in := testInput(t)
	// Digital silence produces negative infinity throughout, which is the case
	// that would break a naive schema.
	format := measure.Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}
	set, err := measure.NewSet(format, measure.Options{Spectrum: measure.DefaultSpectrumOptions()})
	if err != nil {
		t.Fatalf("NewSet() error = %v", err)
	}
	silent := make([]int32, 4096)
	set.Write(measure.Block{Samples: [][]int32{silent, silent}})
	in.Decode.Results = set.Results()

	r, err := New(in)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	data, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var round Report
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if round.Note != Note {
		t.Error("the note did not survive a round trip")
	}
	if round.Integrity.ComputedMD5 != r.Integrity.ComputedMD5 {
		t.Error("the checksum did not survive a round trip")
	}
}

func TestNewFillsEmptySlicesRatherThanNull(t *testing.T) {
	in := testInput(t)
	in.Metadata.Tags = nil
	in.Metadata.ForeignTags = nil

	r, err := New(in)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	data, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, want := range []string{`"comments": []`, `"foreign_tags": []`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("report does not contain %s", want)
		}
	}
}
