package measure

import (
	"crypto/md5"
	"encoding/hex"
	"hash"
)

// Fingerprint recomputes the MD5 checksum that a FLAC stream stores in its
// StreamInfo block.
//
// The checksum is defined over the unencoded audio: samples in stream order,
// interleaved by channel, each written little-endian two's complement in the
// declared resolution rounded up to a whole number of bytes. Comparing the
// recomputed value against the stored one is what proves a file decodes to the
// audio it claims to contain.
type Fingerprint struct {
	format  Format
	hash    hash.Hash
	scratch []byte
	samples uint64
}

// FingerprintResult is the final value of a Fingerprint accumulator.
type FingerprintResult struct {
	// MD5 is the lowercase hex checksum of the decoded audio.
	MD5 string
	// Samples is the number of inter-channel samples hashed.
	Samples uint64
}

// NewFingerprint returns a Fingerprint accumulator for the given format.
func NewFingerprint(f Format) *Fingerprint {
	return &Fingerprint{format: f, hash: md5.New()}
}

// Name implements Accumulator.
func (a *Fingerprint) Name() string { return "fingerprint" }

// Write implements Accumulator.
func (a *Fingerprint) Write(b Block) {
	n, channels := b.Len(), b.Channels()
	if n == 0 || channels == 0 {
		return
	}

	width := a.format.BytesPerSample()
	need := n * channels * width
	if cap(a.scratch) < need {
		a.scratch = make([]byte, need)
	}
	buf := a.scratch[:need]

	pos := 0
	for i := range n {
		for c := range channels {
			v := uint32(b.Samples[c][i])
			for w := range width {
				buf[pos+w] = byte(v >> (8 * w))
			}
			pos += width
		}
	}

	a.hash.Write(buf)
	a.samples += uint64(n)
}

// Result implements the finaliser for the accumulator.
func (a *Fingerprint) Result() FingerprintResult {
	return FingerprintResult{
		MD5:     hex.EncodeToString(a.hash.Sum(nil)),
		Samples: a.samples,
	}
}
