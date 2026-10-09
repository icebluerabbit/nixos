package config

import (
	flag "github.com/spf13/pflag"
)

// registerAccept declares the Layer-1 acceptance flags. List rules are
// repeatable flags; empty list / 0 / "" means the rule is off.
func registerAccept(fs *flag.FlagSet) {
	fs.StringArray("accept-media-type", nil, "allowed MAM media type, e.g. Ebook (repeatable); empty = any")
	fs.StringArray("accept-category-allow", nil, "MAM category to allow; matches if the release has ANY listed (repeatable); empty = any")
	fs.StringArray("accept-category-deny", nil, "MAM category to deny; rejects if the release has ANY listed (repeatable; takes precedence)")
	fs.StringArray("accept-filetype-allow", nil, "filetype to allow; matches if the release has ANY listed (repeatable); empty = any")
	fs.StringArray("accept-filetype-deny", nil, "filetype to deny; rejects if the release has ANY listed (repeatable; takes precedence)")
	fs.StringArray("accept-fiction-type", nil, "MAM fiction flag to allow: Fiction or Non-Fiction (repeatable); empty = any. Releases carrying no flag (torznab search results) always pass")
	fs.StringArray("accept-language", nil, "language substring, case-insensitive (repeatable); empty = any")
	fs.Int64("accept-size-min", 0, "reject below this size in bytes; 0 = no floor")
	fs.Int64("accept-size-max", 0, "reject above this size in bytes; 0 = no ceiling")
	fs.Bool("accept-freeleech-only", false, "reject non-freeleech releases")
	fs.Bool("accept-vip-only", false, "reject non-VIP releases (raw release flag)")
	fs.Bool("accept-exclude-vip", false, "reject VIP releases (raw release flag)")
	fs.String("accept-name-include", "", "require name to match this regex, case-insensitive; empty = off")
	fs.String("accept-name-exclude", "", "reject name matching this regex, case-insensitive; empty = off")
}
