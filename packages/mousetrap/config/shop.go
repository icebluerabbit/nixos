package config

import (
	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerShop binds the bonus-point shop flags. dry-run defaults on: nothing is
// bought until explicitly armed.
func registerShop(fs *flag.FlagSet, c *types.ShopConfig) {
	fs.BoolVar(&c.DryRun, "shop-dry-run", true, "decide and log only; buy nothing (arm by setting false)")
	fs.Int64Var(&c.Reserved, "shop-reserved", 0, "absolute bonus-point floor, never spent")
	fs.BoolVar(&c.VIPEnable, "shop-vip-enable", false, "allow buying/renewing VIP")
	fs.Int64Var(&c.VIPCost, "shop-vip-cost", 5000, "bonus points per VIP purchase")
	fs.Int64Var(&c.VIPReserveMultiple, "shop-vip-reserve-multiple", 2, "hold this × VIP cost for renewals before buying upload")
	fs.StringArrayVar(&c.VIPBuyableClasses, "shop-vip-buyable-class", nil, "account class permitted to buy VIP (repeatable); empty = never")
	fs.BoolVar(&c.UploadEnable, "shop-upload-enable", false, "allow buying upload credit")
	fs.Int64Var(&c.UploadChunkGiB, "shop-upload-chunk-gib", 50, "GiB of upload per purchase")
	fs.Int64Var(&c.UploadPointsPerGiB, "shop-upload-points-per-gib", 500, "bonus points per GiB")
}
