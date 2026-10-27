package measure

import (
	"math"
	"testing"
)

func TestWindowNames(t *testing.T) {
	want := []string{"rectangular", "hann", "hamming", "blackman-harris"}
	got := WindowNames()
	if len(got) != len(want) {
		t.Fatalf("WindowNames() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("WindowNames()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNewWindowRejectsBadInput(t *testing.T) {
	if _, err := NewWindow(WindowHann, 0); err == nil {
		t.Error("NewWindow() with zero length succeeded, want error")
	}
	if _, err := NewWindow(WindowHann, -4); err == nil {
		t.Error("NewWindow() with negative length succeeded, want error")
	}
	if _, err := NewWindow("kaiser", 8); err == nil {
		t.Error("NewWindow() with unknown name succeeded, want error")
	}
}

func TestNewWindowLengthAndRange(t *testing.T) {
	for _, name := range WindowNames() {
		t.Run(name, func(t *testing.T) {
			const n = 64
			w, err := NewWindow(name, n)
			if err != nil {
				t.Fatalf("NewWindow() error = %v", err)
			}
			if len(w) != n {
				t.Fatalf("len = %d, want %d", len(w), n)
			}
			for i, v := range w {
				if v < -0.01 || v > 1.01 {
					t.Errorf("coefficient %d = %v, outside the expected range", i, v)
				}
			}
		})
	}
}

func TestNewWindowIsSymmetric(t *testing.T) {
	for _, name := range WindowNames() {
		t.Run(name, func(t *testing.T) {
			const n = 33
			w, err := NewWindow(name, n)
			if err != nil {
				t.Fatalf("NewWindow() error = %v", err)
			}
			for i := range n / 2 {
				if !closeTo(w[i], w[n-1-i], 1e-12) {
					t.Errorf("coefficient %d = %v but its mirror %d = %v", i, w[i], n-1-i, w[n-1-i])
				}
			}
		})
	}
}

func TestNewWindowKnownValues(t *testing.T) {
	tests := []struct {
		name      string
		window    string
		n         int
		index     int
		want      float64
		tolerance float64
	}{
		{"rectangular is flat", WindowRectangular, 8, 3, 1, 1e-12},
		{"hann starts at zero", WindowHann, 8, 0, 0, 1e-12},
		{"hann peaks at the centre", WindowHann, 9, 4, 1, 1e-12},
		{"hamming starts at its pedestal", WindowHamming, 8, 0, 0.08, 1e-12},
		{"blackman-harris starts near zero", WindowBlackmanHarris, 8, 0, 0.00006, 1e-6},
		{"blackman-harris peaks at the centre", WindowBlackmanHarris, 9, 4, 1, 1e-9},
		{"single point window is unity", WindowHann, 1, 0, 1, 1e-12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewWindow(tt.window, tt.n)
			if err != nil {
				t.Fatalf("NewWindow() error = %v", err)
			}
			if !closeTo(w[tt.index], tt.want, tt.tolerance) {
				t.Errorf("coefficient %d = %v, want %v", tt.index, w[tt.index], tt.want)
			}
		})
	}
}

func TestCoherentGain(t *testing.T) {
	tests := []struct {
		name      string
		window    []float64
		want      float64
		tolerance float64
	}{
		{"empty", nil, 0, 0},
		{"all ones", []float64{1, 1, 1, 1}, 1, 1e-12},
		{"mixed", []float64{0, 1, 0, 1}, 0.5, 1e-12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CoherentGain(tt.window); !closeTo(got, tt.want, tt.tolerance) {
				t.Errorf("CoherentGain() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCoherentGainOfNamedWindows(t *testing.T) {
	// Textbook coherent gains, which the accumulator relies on to keep bin
	// magnitudes comparable to digital full scale.
	tests := []struct {
		window string
		want   float64
	}{
		{WindowRectangular, 1},
		{WindowHann, 0.5},
		{WindowHamming, 0.54},
		{WindowBlackmanHarris, 0.35875},
	}
	for _, tt := range tests {
		t.Run(tt.window, func(t *testing.T) {
			w, err := NewWindow(tt.window, 4096)
			if err != nil {
				t.Fatalf("NewWindow() error = %v", err)
			}
			if got := CoherentGain(w); math.Abs(got-tt.want) > 1e-3 {
				t.Errorf("CoherentGain() = %v, want approximately %v", got, tt.want)
			}
		})
	}
}

func TestDefaultWindowIsBlackmanHarris(t *testing.T) {
	if DefaultWindow != WindowBlackmanHarris {
		t.Errorf("DefaultWindow = %q, want %q", DefaultWindow, WindowBlackmanHarris)
	}
}
