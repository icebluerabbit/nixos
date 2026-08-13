package config

import (
	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerAccept binds the Layer-1 acceptance flags to the struct. List rules are
// repeatable flags; empty list / 0 / "" means the rule is off.
func registerAccept(fs *flag.FlagSet, c *types.AcceptConfig) {
	fs.StringArrayVar(&c.MediaTypes, "accept-media-type", nil, "allowed MAM media type, e.g. Ebook (repeatable); empty = any")
	fs.StringArrayVar(&c.CategoriesAllow, "accept-category-allow", nil, "MAM category to allow; matches if the release has ANY listed (repeatable); empty = any")
	fs.StringArrayVar(&c.CategoriesDeny, "accept-category-deny", nil, "MAM category to deny; rejects if the release has ANY listed (repeatable; takes precedence)")
	fs.StringArrayVar(&c.FiletypesAllow, "accept-filetype-allow", nil, "filetype to allow; matches if the release has ANY listed (repeatable); empty = any")
	fs.StringArrayVar(&c.FiletypesDeny, "accept-filetype-deny", nil, "filetype to deny; rejects if the release has ANY listed (repeatable; takes precedence)")
	fs.StringArrayVar(&c.FictionTypes, "accept-fiction-type", nil, "MAM fiction flag to allow: Fiction or Non-Fiction (repeatable); empty = any. Releases carrying no flag (torznab search results) always pass")
	fs.StringArrayVar(&c.LanguagesAllow, "accept-language", nil, "language substring, case-insensitive (repeatable); empty = any")
	fs.Int64Var(&c.SizeMinBytes, "accept-size-min", 0, "reject below this size in bytes; 0 = no floor")
	fs.Int64Var(&c.SizeMaxBytes, "accept-size-max", 0, "reject above this size in bytes; 0 = no ceiling")
	fs.BoolVar(&c.FreeleechOnly, "accept-freeleech-only", false, "reject non-freeleech releases")
	fs.BoolVar(&c.VIPOnly, "accept-vip-only", false, "reject non-VIP releases (raw release flag)")
	fs.BoolVar(&c.ExcludeVIP, "accept-exclude-vip", false, "reject VIP releases (raw release flag)")
	fs.StringVar(&c.NameIncludeRegex, "accept-name-include", "", "require name to match this regex, case-insensitive; empty = off")
	fs.StringVar(&c.NameExcludeRegex, "accept-name-exclude", "", "reject name matching this regex, case-insensitive; empty = off")
}
