package config

import (
	flag "github.com/spf13/pflag"
)

// Shop defaults. Cost and reserve multiple are MAM's own pricing; the upload
// figures are the chunk size and rate the bonus shop sells at.
const (
	defaultShopVIPCost            = 5000
	defaultShopVIPReserveMultiple = 2
	defaultShopUploadChunkGiB     = 50
	defaultShopUploadPointsPerGiB = 500
)

// registerShop declares the bonus-point shop flags. dry-run defaults on:
// nothing is bought until explicitly armed.
func registerShop(fs *flag.FlagSet) {
	fs.Bool("shop-dry-run", true, "decide and log only; buy nothing (arm by setting false)")
	fs.Int64("shop-reserved", 0, "absolute bonus-point floor, never spent")
	fs.Bool("shop-vip-enable", false, "allow buying/renewing VIP")
	fs.Int64("shop-vip-cost", defaultShopVIPCost, "bonus points per VIP purchase")
	fs.Int64("shop-vip-reserve-multiple", defaultShopVIPReserveMultiple, "hold this × VIP cost for renewals before buying upload")
	fs.StringArray("shop-vip-buyable-class", nil, "account class permitted to buy VIP (repeatable); empty = never")
	fs.Bool("shop-upload-enable", false, "allow buying upload credit")
	fs.Int64("shop-upload-chunk-gib", defaultShopUploadChunkGiB, "GiB of upload per purchase")
	fs.Int64("shop-upload-points-per-gib", defaultShopUploadPointsPerGiB, "bonus points per GiB")
}
