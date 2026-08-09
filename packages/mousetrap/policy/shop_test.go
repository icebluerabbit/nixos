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
		name string
		acct types.AccountState
		want types.ShopAction
	}{
		{"buy_vip", types.AccountState{ClassName: "Power User", SeedBonus: 6000}, types.ShopBuyVIP},
		{"vip_too_poor_hold", types.AccountState{ClassName: "Power User", SeedBonus: 500}, types.ShopHold},
		{"already_vip_buys_upload", types.AccountState{ClassName: "Elite VIP", SeedBonus: 40000}, types.ShopBuyUpload},
		{"class_not_buyable_buys_upload", types.AccountState{ClassName: "User", SeedBonus: 40000}, types.ShopBuyUpload},
		{"upload_too_poor_hold", types.AccountState{ClassName: "User", SeedBonus: 20000}, types.ShopHold},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if act, _ := e.Shop(tc.acct); act != tc.want {
				t.Errorf("Shop = %s, want %s", act, tc.want)
			}
		})
	}
}

func TestShopDisabled(t *testing.T) {
	e := shopEngine(t, types.ShopConfig{}) // nothing enabled
	if act, _ := e.Shop(types.AccountState{SeedBonus: 1_000_000}); act != types.ShopHold {
		t.Errorf("disabled shop = %s, want hold", act)
	}
}
