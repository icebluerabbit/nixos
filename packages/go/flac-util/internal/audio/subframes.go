package audio

import "github.com/mewkiz/flac/frame"

// SubframeStats counts how a stream's frames were encoded.
//
// FLAC lets an encoder choose, for every subframe, how to predict the next
// sample: not at all, from a fixed formula of a given order, or from
// coefficients it worked out itself. Encoders differ in which they reach for
// and how hard they look, so the distribution is a fingerprint of the encoder
// and of the effort it was run at.
//
// That matters when the vendor string has been stripped, which happens as soon
// as a file passes through a tagger. A stream whose vendor says nothing may
// still say which program wrote it.
//
// Nothing here identifies an encoder by name. The counts are recorded; which
// encoder they resemble is the reader's business.
type SubframeStats struct {
	// Constant, Verbatim, Fixed and LPC count subframes by prediction method.
	// Verbatim means the encoder gave up and stored samples untouched, which
	// a well-run encoder rarely needs to do.
	Constant int64
	Verbatim int64
	Fixed    int64
	LPC      int64

	// FixedOrders and LPCOrders count how often each prediction order was
	// chosen. A high LPC order costs the encoder time to find, so the shape of
	// this distribution says how hard it looked.
	FixedOrders map[int]int64
	LPCOrders   map[int]int64

	// WastedBits counts subframes where the encoder found every sample to
	// share trailing zero bits and stored them shifted. A stream where this is
	// common is one whose audio does not use its declared depth.
	WastedBits int64

	// Subframes is the total counted.
	Subframes int64
}

// NewSubframeStats returns empty statistics.
func NewSubframeStats() *SubframeStats {
	return &SubframeStats{
		FixedOrders: map[int]int64{},
		LPCOrders:   map[int]int64{},
	}
}

// Add folds one frame's subframes into the statistics.
func (s *SubframeStats) Add(f *frame.Frame) {
	if f == nil {
		return
	}

	for _, sub := range f.Subframes {
		if sub == nil {
			continue
		}
		s.Subframes++
		if sub.Wasted > 0 {
			s.WastedBits++
		}

		switch sub.Pred {
		case frame.PredConstant:
			s.Constant++
		case frame.PredVerbatim:
			s.Verbatim++
		case frame.PredFixed:
			s.Fixed++
			s.FixedOrders[sub.Order]++
		case frame.PredFIR:
			s.LPC++
			s.LPCOrders[sub.Order]++
		}
	}
}
