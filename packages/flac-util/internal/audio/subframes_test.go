package audio

import (
	"testing"

	"github.com/mewkiz/flac/frame"
)

// subframe returns one subframe with the given prediction method and order.
func subframe(pred frame.Pred, order int, wasted uint) *frame.Subframe {
	return &frame.Subframe{
		SubHeader: frame.SubHeader{Pred: pred, Order: order, Wasted: wasted},
	}
}

func TestSubframeStatsCountsEveryMethod(t *testing.T) {
	s := NewSubframeStats()
	s.Add(&frame.Frame{Subframes: []*frame.Subframe{
		subframe(frame.PredConstant, 0, 0),
		subframe(frame.PredVerbatim, 0, 0),
		subframe(frame.PredFixed, 2, 0),
		subframe(frame.PredFixed, 2, 0),
		subframe(frame.PredFIR, 8, 0),
		subframe(frame.PredFIR, 12, 0),
	}})

	got := *s
	switch {
	case got.Constant != 1:
		t.Errorf("Constant = %d, want 1", got.Constant)
	case got.Verbatim != 1:
		t.Errorf("Verbatim = %d, want 1", got.Verbatim)
	case got.Fixed != 2:
		t.Errorf("Fixed = %d, want 2", got.Fixed)
	case got.LPC != 2:
		t.Errorf("LPC = %d, want 2", got.LPC)
	case got.Subframes != 6:
		t.Errorf("Subframes = %d, want 6", got.Subframes)
	}

	if got.FixedOrders[2] != 2 {
		t.Errorf("FixedOrders[2] = %d, want 2", got.FixedOrders[2])
	}
	if got.LPCOrders[8] != 1 || got.LPCOrders[12] != 1 {
		t.Errorf("LPCOrders = %v, want one each of 8 and 12", got.LPCOrders)
	}
}

// TestSubframeStatsCountsWastedBits covers the signal that a stream does not
// use the depth it declares: an encoder finding every sample in a subframe to
// share trailing zeros stores them shifted and records how many.
func TestSubframeStatsCountsWastedBits(t *testing.T) {
	s := NewSubframeStats()
	s.Add(&frame.Frame{Subframes: []*frame.Subframe{
		subframe(frame.PredFIR, 8, 8),
		subframe(frame.PredFIR, 8, 0),
	}})

	if s.WastedBits != 1 {
		t.Errorf("WastedBits = %d, want 1", s.WastedBits)
	}
}

func TestSubframeStatsAccumulatesAcrossFrames(t *testing.T) {
	s := NewSubframeStats()
	for range 10 {
		s.Add(&frame.Frame{Subframes: []*frame.Subframe{
			subframe(frame.PredFIR, 8, 0),
			subframe(frame.PredFIR, 8, 0),
		}})
	}

	if s.Subframes != 20 {
		t.Errorf("Subframes = %d, want 20", s.Subframes)
	}
	if s.LPCOrders[8] != 20 {
		t.Errorf("LPCOrders[8] = %d, want 20", s.LPCOrders[8])
	}
}

func TestSubframeStatsIgnoresNothing(t *testing.T) {
	s := NewSubframeStats()
	s.Add(nil)
	s.Add(&frame.Frame{Subframes: []*frame.Subframe{nil}})

	if s.Subframes != 0 {
		t.Errorf("Subframes = %d, want none", s.Subframes)
	}
}
