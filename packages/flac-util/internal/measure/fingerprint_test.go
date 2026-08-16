package measure

import (
	"crypto/md5"
	"encoding/hex"
	"testing"
)

// expectMD5 hashes an explicit byte sequence written out by hand, so that the
// expected value never travels through the code under test.
func expectMD5(t *testing.T, raw []byte) string {
	t.Helper()
	sum := md5.Sum(raw)
	return hex.EncodeToString(sum[:])
}

func TestFingerprintName(t *testing.T) {
	if got := NewFingerprint(testFormat()).Name(); got != "fingerprint" {
		t.Errorf("Name() = %q, want %q", got, "fingerprint")
	}
}

func TestFingerprintEmptyStream(t *testing.T) {
	a := NewFingerprint(testFormat())
	got := a.Result()

	if want := expectMD5(t, nil); got.MD5 != want {
		t.Errorf("MD5 = %q, want %q", got.MD5, want)
	}
	if got.Samples != 0 {
		t.Errorf("Samples = %d, want 0", got.Samples)
	}
}

// TestFingerprintSixteenBitLayout pins the interleaving and byte order FLAC
// defines for its stored checksum: samples in stream order, channels
// interleaved, little-endian two's complement.
func TestFingerprintSixteenBitLayout(t *testing.T) {
	a := NewFingerprint(Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16})
	a.Write(Block{Samples: [][]int32{
		{1, -1},
		{0, 256},
	}})

	want := expectMD5(t, []byte{
		0x01, 0x00, // sample 0, left  = 1
		0x00, 0x00, // sample 0, right = 0
		0xff, 0xff, // sample 1, left  = -1
		0x00, 0x01, // sample 1, right = 256
	})

	got := a.Result()
	if got.MD5 != want {
		t.Errorf("MD5 = %q, want %q", got.MD5, want)
	}
	if got.Samples != 2 {
		t.Errorf("Samples = %d, want 2", got.Samples)
	}
}

func TestFingerprintTwentyFourBitLayout(t *testing.T) {
	a := NewFingerprint(Format{SampleRate: 96000, Channels: 1, BitsPerSample: 24})
	a.Write(Block{Samples: [][]int32{{1, -1, 0x7fffff}}})

	want := expectMD5(t, []byte{
		0x01, 0x00, 0x00,
		0xff, 0xff, 0xff,
		0xff, 0xff, 0x7f,
	})

	if got := a.Result().MD5; got != want {
		t.Errorf("MD5 = %q, want %q", got, want)
	}
}

// TestFingerprintTwentyBitUsesThreeBytes covers a resolution that is not a
// whole number of bytes, which FLAC rounds up.
func TestFingerprintTwentyBitUsesThreeBytes(t *testing.T) {
	a := NewFingerprint(Format{SampleRate: 48000, Channels: 1, BitsPerSample: 20})
	a.Write(Block{Samples: [][]int32{{1}}})

	want := expectMD5(t, []byte{0x01, 0x00, 0x00})
	if got := a.Result().MD5; got != want {
		t.Errorf("MD5 = %q, want %q", got, want)
	}
}

// TestFingerprintIsIndependentOfBlocking checks that the checksum depends on
// the audio alone, not on how the decoder happened to chop it up.
func TestFingerprintIsIndependentOfBlocking(t *testing.T) {
	format := Format{SampleRate: 44100, Channels: 2, BitsPerSample: 16}

	whole := NewFingerprint(format)
	whole.Write(Block{Samples: [][]int32{
		{1, 2, 3, 4},
		{5, 6, 7, 8},
	}})

	split := NewFingerprint(format)
	split.Write(Block{Samples: [][]int32{{1, 2}, {5, 6}}})
	split.Write(Block{Samples: [][]int32{{3}, {7}}})
	split.Write(Block{Samples: [][]int32{{4}, {8}}})

	if whole.Result() != split.Result() {
		t.Errorf("blocking changed the result: %+v then %+v", whole.Result(), split.Result())
	}
}

func TestFingerprintIgnoresEmptyBlocks(t *testing.T) {
	a := NewFingerprint(testFormat())
	a.Write(Block{})
	a.Write(Block{Samples: [][]int32{{}, {}}})

	if want := expectMD5(t, nil); a.Result().MD5 != want {
		t.Errorf("MD5 = %q, want the empty-input checksum %q", a.Result().MD5, want)
	}
}

func TestFingerprintReusesScratchAcrossBlocks(t *testing.T) {
	// Feeding a large block after a small one exercises the growth path of the
	// scratch buffer; the result must still depend only on the audio.
	format := Format{SampleRate: 44100, Channels: 1, BitsPerSample: 16}

	grown := NewFingerprint(format)
	grown.Write(Block{Samples: [][]int32{{1}}})
	grown.Write(Block{Samples: [][]int32{{2, 3, 4, 5, 6, 7, 8, 9}}})

	direct := NewFingerprint(format)
	direct.Write(Block{Samples: [][]int32{{1, 2, 3, 4, 5, 6, 7, 8, 9}}})

	if grown.Result() != direct.Result() {
		t.Errorf("scratch growth changed the result: %+v then %+v", grown.Result(), direct.Result())
	}
}
