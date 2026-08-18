package report

import (
	"encoding/json"
	"errors"
	"fmt"
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
	// One column at full scale and one silent averages to half power, which
	// the spectrum arrays keep to two decimals.
	if want := roundDB(measure.PowerDB(0.5)); got.AverageDBFS[0] != want {
		t.Errorf("AverageDBFS[0] = %v, want %v", float64(got.AverageDBFS[0]), float64(want))
	}
	if !math.IsInf(float64(got.AverageDBFS[1]), -1) {
		t.Errorf("AverageDBFS[1] = %v, want -Inf", float64(got.AverageDBFS[1]))
	}
}

func TestSummariseSpectrumRecordsEachChannel(t *testing.T) {
	grid := &measure.Grid{
		Power:      [][]float32{{1, 0}},
		Bins:       2,
		SampleRate: 4000,
		FFTSize:    4,
		Channels: []measure.ChannelSpectrum{
			{Mean: []float64{0.5, 0}, Peak: []float64{1, 0}},
			{Mean: []float64{0.25, 0.25}, Peak: []float64{0.25, 1}},
		},
	}

	got := summariseSpectrum(grid)
	if len(got.Channels) != 2 {
		t.Fatalf("len(Channels) = %d, want 2", len(got.Channels))
	}
	for c, ch := range got.Channels {
		if len(ch.MeanDBFS) != grid.Bins || len(ch.PeakDBFS) != grid.Bins {
			t.Errorf("channel %d has %d mean and %d peak bins, want %d each",
				c, len(ch.MeanDBFS), len(ch.PeakDBFS), grid.Bins)
		}
	}

	if want := roundDB(measure.PowerDB(0.5)); got.Channels[0].MeanDBFS[0] != want {
		t.Errorf("Channels[0].MeanDBFS[0] = %v, want %v",
			float64(got.Channels[0].MeanDBFS[0]), float64(want))
	}
	if want := roundDB(measure.PowerDB(1)); got.Channels[0].PeakDBFS[0] != want {
		t.Errorf("Channels[0].PeakDBFS[0] = %v, want %v",
			float64(got.Channels[0].PeakDBFS[0]), float64(want))
	}
	// A bin that is silent on average and silent at its peak stays silent.
	if !math.IsInf(float64(got.Channels[0].PeakDBFS[1]), -1) {
		t.Errorf("Channels[0].PeakDBFS[1] = %v, want -Inf", float64(got.Channels[0].PeakDBFS[1]))
	}
	// The second channel occupies a band the first never reaches, which is
	// exactly what the mixed-down average would hide.
	if got.Channels[1].PeakDBFS[1] <= got.Channels[0].PeakDBFS[1] {
		t.Errorf("Channels[1].PeakDBFS[1] = %v, want above channel 0's %v",
			float64(got.Channels[1].PeakDBFS[1]), float64(got.Channels[0].PeakDBFS[1]))
	}
}

func TestSummariseSpectrumPeakNeverBelowMean(t *testing.T) {
	grid := &measure.Grid{
		Power:      [][]float32{{1, 0.5}},
		Bins:       2,
		SampleRate: 4000,
		FFTSize:    4,
		Channels: []measure.ChannelSpectrum{
			{Mean: []float64{0.1, 0.5}, Peak: []float64{0.9, 0.5}},
		},
	}

	for bin, ch := 0, summariseSpectrum(grid).Channels[0]; bin < grid.Bins; bin++ {
		if ch.PeakDBFS[bin] < ch.MeanDBFS[bin] {
			t.Errorf("bin %d peak %v is below mean %v",
				bin, float64(ch.PeakDBFS[bin]), float64(ch.MeanDBFS[bin]))
		}
	}
}

func TestSummariseSpectrumPadsShortChannels(t *testing.T) {
	// A channel summary shorter than the grid's bin count must still line up
	// with AverageDBFS, so that the two can be indexed together.
	grid := &measure.Grid{
		Power:      [][]float32{{1, 0, 0}},
		Bins:       3,
		SampleRate: 6000,
		FFTSize:    4,
		Channels:   []measure.ChannelSpectrum{{Mean: []float64{1}, Peak: []float64{1}}},
	}

	got := summariseSpectrum(grid)
	ch := got.Channels[0]
	if len(ch.MeanDBFS) != 3 || len(ch.PeakDBFS) != 3 {
		t.Fatalf("got %d mean and %d peak bins, want 3 each", len(ch.MeanDBFS), len(ch.PeakDBFS))
	}
	if !math.IsInf(float64(ch.MeanDBFS[2]), -1) {
		t.Errorf("MeanDBFS[2] = %v, want -Inf", float64(ch.MeanDBFS[2]))
	}
}

func TestSummariseSpectrumWithoutChannels(t *testing.T) {
	grid := &measure.Grid{
		Power:      [][]float32{{1}},
		Bins:       1,
		SampleRate: 4000,
		FFTSize:    2,
	}

	if got := summariseSpectrum(grid); len(got.Channels) != 0 {
		t.Errorf("len(Channels) = %d, want 0", len(got.Channels))
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
	for _, want := range []string{`"comments":[]`, `"foreign_tags":[]`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("report does not contain %s", want)
		}
	}
}

func TestNewRecordsACleanDecode(t *testing.T) {
	r := newTestReport(t, Input{})
	if !r.Decode.Complete {
		t.Error("decode.complete = false with no error")
	}
	if r.Decode.Error != "" || r.Decode.Stage != "" {
		t.Errorf("decode = %+v, want it empty", r.Decode)
	}
}

// TestNewRecordsADecodeFault covers what makes a partial report usable later:
// the reason the audio stopped has to be in the file, or a reader mining a
// library of reports cannot tell a short file from an odd one.
func TestNewRecordsADecodeFault(t *testing.T) {
	want := errors.New("frame 1696: invalid sync-code")
	r := newTestReport(t, Input{DecodeErr: want, DecodeStage: "decode"})

	if r.Decode.Complete {
		t.Error("decode.complete = true despite an error")
	}
	if r.Decode.Error != want.Error() {
		t.Errorf("decode.error = %q, want %q", r.Decode.Error, want.Error())
	}
	if r.Decode.Stage != "decode" {
		t.Errorf("decode.stage = %q, want %q", r.Decode.Stage, "decode")
	}
}

func TestDecodeSectionRoundTrips(t *testing.T) {
	r := newTestReport(t, Input{DecodeErr: errors.New("unexpected EOF"), DecodeStage: "decode"})

	data, err := Marshal(r)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got Report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Decode != r.Decode {
		t.Errorf("decode = %+v, want %+v", got.Decode, r.Decode)
	}
}

func TestErrorText(t *testing.T) {
	if got := errorText(nil, "/a/t.flac"); got != "" {
		t.Errorf("errorText(nil) = %q, want empty", got)
	}
	if got := errorText(errors.New("broken"), "/a/t.flac"); got != "broken" {
		t.Errorf("errorText() = %q, want %q", got, "broken")
	}
}

// newTestReport builds a report from the standard input with the given fields
// overridden, so a test can state only what it is about.
func newTestReport(t *testing.T, overrides Input) *Report {
	t.Helper()

	in := testInput(t)
	if overrides.DecodeErr != nil {
		in.DecodeErr = overrides.DecodeErr
	}
	if overrides.DecodeStage != "" {
		in.DecodeStage = overrides.DecodeStage
	}

	r, err := New(in)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return r
}

// TestErrorTextDropsTheFilesOwnPath keeps the error strings comparable across
// a library: with the path left in, two copies of the same fault would look
// like two different faults.
func TestErrorTextDropsTheFilesOwnPath(t *testing.T) {
	path := "/music/Artist/Album/01 Track.flac"
	err := fmt.Errorf("decode %s: frame 1696: invalid sync-code", path)

	if got, want := errorText(err, path), "frame 1696: invalid sync-code"; got != want {
		t.Errorf("errorText() = %q, want %q", got, want)
	}
}

func TestErrorTextWithoutAPath(t *testing.T) {
	err := errors.New("frame 1: unexpected EOF")
	if got := errorText(err, ""); got != err.Error() {
		t.Errorf("errorText() = %q, want it unchanged", got)
	}
}

func TestRoundDB(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{-94.64743041992188, -94.65},
		{-3.010299956639812, -3.01},
		{0, 0},
		{-0.004, -0},
		{-120.005, -120.01},
		{12.344, 12.34},
		{12.345, 12.35},
	}
	for _, c := range cases {
		if got := roundDB(c.in); float64(got) != c.want {
			t.Errorf("roundDB(%v) = %v, want %v", c.in, float64(got), c.want)
		}
	}
}

func TestRoundDBKeepsSilence(t *testing.T) {
	// A silent bin measures as negative infinity and has to stay that way, so
	// that it still marshals as null rather than becoming a real level.
	if got := roundDB(math.Inf(-1)); !math.IsInf(float64(got), -1) {
		t.Errorf("roundDB(-Inf) = %v, want -Inf", float64(got))
	}
	if got := roundDB(math.NaN()); !math.IsNaN(float64(got)) {
		t.Errorf("roundDB(NaN) = %v, want NaN", float64(got))
	}
}

func TestRoundedSpectrumMarshalsShort(t *testing.T) {
	// The point of rounding is the size of the written file, so check the text
	// rather than the value.
	grid := &measure.Grid{
		Power:      [][]float32{{0.34567891, 0}},
		Bins:       2,
		SampleRate: 4000,
		FFTSize:    4,
		Channels:   []measure.ChannelSpectrum{{Mean: []float64{0.34567891, 0}, Peak: []float64{1, 0}}},
	}

	data, err := json.Marshal(summariseSpectrum(grid))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	for _, want := range []string{`"average_dbfs":[-4.61,null]`, `"mean_dbfs":[-4.61,null]`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("marshalled spectrum missing %s\ngot %s", want, data)
		}
	}
}
