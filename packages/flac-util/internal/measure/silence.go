package measure

// Silence measures runs of digital silence at the start and end of a stream,
// where digital silence means every channel holding exactly zero.
//
// Unusual amounts of it can mark padding added when material was assembled
// from more than one source.
type Silence struct {
	format      Format
	leading     uint64
	leadingOpen bool
	trailing    uint64
	total       uint64
}

// SilenceResult is the final value of a Silence accumulator.
type SilenceResult struct {
	// LeadingSamples counts silent inter-channel samples before the first
	// non-zero one.
	LeadingSamples uint64
	// TrailingSamples counts silent inter-channel samples after the last
	// non-zero one.
	TrailingSamples uint64
	// TotalSamples is the number of inter-channel samples examined.
	TotalSamples uint64
	// AllSilent reports that the stream contains no non-zero sample at all.
	AllSilent bool
}

// NewSilence returns a Silence accumulator for the given format.
func NewSilence(f Format) *Silence {
	return &Silence{format: f, leadingOpen: true}
}

// Name implements Accumulator.
func (a *Silence) Name() string { return "silence" }

// Write implements Accumulator.
func (a *Silence) Write(b Block) {
	n, channels := b.Len(), b.Channels()
	if n == 0 || channels == 0 {
		return
	}

	for i := range n {
		silent := true
		for c := range channels {
			if b.Samples[c][i] != 0 {
				silent = false
				break
			}
		}
		switch {
		case silent && a.leadingOpen:
			a.leading++
			a.trailing++
		case silent:
			a.trailing++
		default:
			a.leadingOpen = false
			a.trailing = 0
		}
	}
	a.total += uint64(n)
}

// Result implements the finaliser for the accumulator.
func (a *Silence) Result() SilenceResult {
	return SilenceResult{
		LeadingSamples:  a.leading,
		TrailingSamples: a.trailing,
		TotalSamples:    a.total,
		AllSilent:       a.total > 0 && a.leadingOpen,
	}
}
