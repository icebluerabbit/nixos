package measure

import "math/bits"

// Bits measures how much of the declared sample resolution a stream actually
// uses.
//
// Scaling audio from one resolution to a larger one multiplies every sample by
// a power of two, which leaves a fixed number of low bits permanently zero.
// Counting those bits therefore distinguishes a stream carrying genuine 24-bit
// detail from one carrying 16-bit audio in a 24-bit container.
type Bits struct {
	format Format
	or     []uint32
}

// ChannelBits holds the resolution measurements for one channel.
type ChannelBits struct {
	// UnusedLowBits is the number of least significant bits that are zero in
	// every sample of the channel.
	UnusedLowBits int
	// EffectiveBits is the declared resolution less UnusedLowBits.
	EffectiveBits int
	// AllZero reports that the channel contains nothing but digital silence,
	// in which case the other fields carry no information.
	AllZero bool
}

// BitsResult is the final value of a Bits accumulator.
type BitsResult struct {
	// DeclaredBits is the resolution the stream claims in its header.
	DeclaredBits int
	// Channels holds the per-channel measurements.
	Channels []ChannelBits
	// EffectiveBits is the largest per-channel effective resolution, which is
	// the resolution the stream as a whole genuinely carries.
	EffectiveBits int
}

// NewBits returns a Bits accumulator for the given format.
func NewBits(f Format) *Bits {
	return &Bits{format: f, or: make([]uint32, f.Channels)}
}

// Name implements Accumulator.
func (a *Bits) Name() string { return "bits" }

// Write implements Accumulator.
func (a *Bits) Write(b Block) {
	for c := range min(b.Channels(), len(a.or)) {
		acc := a.or[c]
		for _, s := range b.Samples[c] {
			acc |= uint32(s)
		}
		a.or[c] = acc
	}
}

// Result implements the finaliser for the accumulator.
func (a *Bits) Result() BitsResult {
	declared := a.format.BitsPerSample
	out := BitsResult{
		DeclaredBits: declared,
		Channels:     make([]ChannelBits, len(a.or)),
	}
	for c, acc := range a.or {
		if acc == 0 {
			out.Channels[c] = ChannelBits{AllZero: true}
			continue
		}
		unused := bits.TrailingZeros32(acc)
		// A stream cannot carry more resolution than it declares, however many
		// low bits happen to be set.
		effective := max(declared-unused, 0)
		out.Channels[c] = ChannelBits{
			UnusedLowBits: unused,
			EffectiveBits: effective,
		}
		out.EffectiveBits = max(out.EffectiveBits, effective)
	}
	return out
}
