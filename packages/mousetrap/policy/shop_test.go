package policy

import (
	"testing"

	"mousetrap/types"
)

func shopEngine(t *testing.T, cfg types.ShopConfig) *Engine {
	t.Helper()
	e, err := New(types.AcceptConfig{}, types.GrabConfig{}, types.WedgeConfig{}, cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return e
}

func TestShop(t *testing.T) {
	// reserve 1000; VIP cost 5000, reserve×2 = 10000; upload cost 50×500 = 25000.
	cfg := types.ShopConfig{
		Reserved:           1000,
		VIPEnable:          true,
		VIPCost:            5000,
		VIPReserveMultiple: 2,
		VIPBuyableClasses:  []string{"Power User"},
		UploadEnable:       true,
		UploadChunkGiB:     50,
		UploadPointsPerGiB: 500,
	}
	e := shopEngine(t, cfg)

	cases := []struct {
		name     string
		acct     types.AccountState
		inflight types.InFlight
		want     types.ShopAction
	}{
		{"buy_vip", types.AccountState{ClassName: "Power User", SeedBonus: 6000}, types.InFlight{}, types.ShopBuyVIP},
		{"vip_too_poor_hold", types.AccountState{ClassName: "Power User", SeedBonus: 500}, types.InFlight{}, types.ShopHold},
		{"already_vip_buys_upload", types.AccountState{ClassName: "Elite VIP", SeedBonus: 40000}, types.InFlight{}, types.ShopBuyUpload},
		{"class_not_buyable_buys_upload", types.AccountState{ClassName: "User", SeedBonus: 40000}, types.InFlight{}, types.ShopBuyUpload},
		{"upload_too_poor_hold", types.AccountState{ClassName: "User", SeedBonus: 20000}, types.InFlight{}, types.ShopHold},

		// In-flight points: MAM's seedbonus has not caught up to a spend we already
		// made, so the same balance must no longer fund a second buy.
		{
			"vip_inflight_points_hold",
			types.AccountState{ClassName: "Power User", SeedBonus: 6000},
			types.InFlight{Points: 5000},
			types.ShopHold,
		},
		{
			"upload_inflight_points_hold",
			types.AccountState{ClassName: "User", SeedBonus: 40000},
			types.InFlight{Points: 25000},
			types.ShopHold,
		},
		// Points in flight must not suppress a buy that is still affordable without them.
		{
			"upload_inflight_points_still_affordable",
			types.AccountState{ClassName: "User", SeedBonus: 40000},
			types.InFlight{Points: 3000},
			types.ShopBuyUpload,
		},
		// Non-points in-flight tallies are the grab gate's business, not the shop's.
		{
			"wedges_and_grabs_inflight_ignored",
			types.AccountState{ClassName: "Power User", SeedBonus: 6000},
			types.InFlight{Count: 9, RatioBytes: 1 << 40, Wedges: 4},
			types.ShopBuyVIP,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if act, _ := e.Shop(tc.acct, tc.inflight); act != tc.want {
				t.Errorf("Shop = %s, want %s", act, tc.want)
			}
		})
	}
}

// TestShopNeverInfersVIP pins the asymmetry: in-flight state is a usage tally only.
// A VIP purchase we believe succeeded must not make the account look VIP — that is
// read from MAM's ClassName on refresh. Were it inferred, a silently-failed buy
// would turn every VIP-flagged release freeleech-for-us in the grab gate.
func TestShopNeverInfersVIP(t *testing.T) {
	cfg := types.ShopConfig{
		Reserved: 1000, VIPEnable: true, VIPCost: 5000,
		VIPReserveMultiple: 2, VIPBuyableClasses: []string{"Power User"},
	}
	e := shopEngine(t, cfg)

	// Points spent on a VIP buy are in flight, but the class has not changed. With
	// enough balance left the account is still a non-VIP Power User, so the decision
	// is driven by ClassName alone.
	acct := types.AccountState{ClassName: "Power User", SeedBonus: 20000}
	if act, _ := e.Shop(acct, types.InFlight{Points: 5000}); act != types.ShopBuyVIP {
		t.Errorf("Shop = %s, want %s (VIP must come from ClassName, not from our own spend)", act, types.ShopBuyVIP)
	}
	if acct.VIP() {
		t.Error("AccountState.VIP() true for a Power User — VIP must never be inferred")
	}
}

func TestShopDisabled(t *testing.T) {
	e := shopEngine(t, types.ShopConfig{}) // nothing enabled
	if act, _ := e.Shop(types.AccountState{SeedBonus: 1_000_000}, types.InFlight{}); act != types.ShopHold {
		t.Errorf("disabled shop = %s, want hold", act)
	}
}
