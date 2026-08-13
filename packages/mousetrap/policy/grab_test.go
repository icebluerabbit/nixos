package policy

import (
	"testing"

	"mousetrap/types"
)

const gib = 1 << 30

func gateEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(
		types.AcceptConfig{},
		types.GrabConfig{UnsatMargin: 5, BufferMinBytes: 2 * gib},
		types.WedgeConfig{
			Enable:                true,
			Reserve:               2,
			ProactiveMediaTypes:   []string{"Audiobook"},
			ProactiveMinSizeBytes: 400_000_000,
			RescueEnable:          true,
			RescueMinSizeBytes:    100_000_000,
		},
		types.ShopConfig{},
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestGrabGate(t *testing.T) {
	// healthy base account: 5/20 unsat, ~9 GiB buffer, 10 wedges, non-VIP.
	base := types.AccountState{
		UnsatCount: 5, UnsatLimit: 20,
		UploadedBytes: 10 * gib, DownloadedBytes: 1 * gib,
		Wedges: 10, ClassName: "User",
	}
	ebook := types.Release{MediaType: "Ebook", Size: 5_000_000}

	cases := []struct {
		name     string
		rel      types.Release
		acct     types.AccountState
		inflight types.InFlight
		wantD    types.GrabDecision
		wantR    types.GrabReason
		wantPlan GrabPlan
	}{
		{
			name:  "no_account",
			rel:   ebook,
			acct:  types.AccountState{}, // UnsatLimit 0
			wantD: types.Hold, wantR: types.GrabNoAccount,
		},
		{
			name:  "unsat_ceiling",
			rel:   ebook,
			acct:  func() types.AccountState { a := base; a.UnsatCount = 15; return a }(),
			wantD: types.Hold, wantR: types.GrabUnsatCeiling,
		},
		{
			name: "unsat_ceiling_via_inflight",
			rel:  ebook, acct: func() types.AccountState { a := base; a.UnsatCount = 12; return a }(),
			inflight: types.InFlight{Count: 3}, // 12+3 >= 20-5
			wantD:    types.Hold, wantR: types.GrabUnsatCeiling,
		},
		{
			name: "freeleech_nonvip",
			rel:  func() types.Release { r := ebook; r.Freeleech = true; return r }(),
			acct: base, wantD: types.Grab, wantR: types.GrabOK,
		},
		{
			name:  "vip_release_account_vip_free",
			rel:   func() types.Release { r := ebook; r.VIP = true; return r }(),
			acct:  func() types.AccountState { a := base; a.ClassName = "Elite VIP"; return a }(),
			wantD: types.Grab, wantR: types.GrabOK,
		},
		{
			name:  "vip_release_nonvip_account_skips",
			rel:   func() types.Release { r := ebook; r.VIP = true; r.Freeleech = true; return r }(),
			acct:  base, // non-VIP account can't download a VIP torrent (MAM 406) → skip
			wantD: types.Skip, wantR: types.GrabVIPRequired,
		},
		{
			name: "normal_ratio_grab",
			rel:  ebook, acct: base,
			wantD: types.Grab, wantR: types.GrabOK, wantPlan: GrabPlan{CostsRatio: true},
		},
		{
			name:  "buffer_hold",
			rel:   func() types.Release { r := ebook; r.Size = 50_000_000; return r }(), // < rescue min
			acct:  func() types.AccountState { a := base; a.UploadedBytes = 1 * gib; a.DownloadedBytes = 0; return a }(),
			wantD: types.Hold, wantR: types.GrabBuffer,
		},
		{
			name:  "proactive_wedge",
			rel:   types.Release{MediaType: "Audiobook", Size: 500_000_000},
			acct:  base, // buffer healthy, but proactive audiobook wedges anyway
			wantD: types.Grab, wantR: types.GrabOKWedge, wantPlan: GrabPlan{UseWedge: true},
		},
		{
			name:  "rescue_wedge",
			rel:   func() types.Release { r := ebook; r.Size = 200_000_000; return r }(), // >= rescue min, would buffer-hold
			acct:  func() types.AccountState { a := base; a.UploadedBytes = 1 * gib; a.DownloadedBytes = 0; return a }(),
			wantD: types.Grab, wantR: types.GrabOKWedge, wantPlan: GrabPlan{UseWedge: true},
		},
		{
			name:  "wedge_wanted_but_none_left",
			rel:   types.Release{MediaType: "Audiobook", Size: 500_000_000},
			acct:  func() types.AccountState { a := base; a.Wedges = 2; return a }(),     // == reserve, not > → unavailable
			wantD: types.Grab, wantR: types.GrabOK, wantPlan: GrabPlan{CostsRatio: true}, // buffer healthy → ratio grab
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, r, plan := gateEngine(t).Grab(tc.rel, tc.acct, tc.inflight)
			if d != tc.wantD || r != tc.wantR {
				t.Errorf("Grab = %s/%s, want %s/%s", d, r, tc.wantD, tc.wantR)
			}
			if plan != tc.wantPlan {
				t.Errorf("plan = %+v, want %+v", plan, tc.wantPlan)
			}
		})
	}
}
