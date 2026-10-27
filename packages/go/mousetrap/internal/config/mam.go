package config

import (
	flag "github.com/spf13/pflag"
)

// defaultMAMAPIBase is the main site; the tracker host is derived from it.
const defaultMAMAPIBase = "https://www.myanonamouse.net"

// registerMAM declares the MyAnonaMouse source-client flags.
func registerMAM(fs *flag.FlagSet) {
	fs.String("mam-api-base", defaultMAMAPIBase, "MAM API base URL")
	fs.String("mam-seed-path", "", "path to the mam_id seed secret (path, not value)")
	fs.StringToString("mam-media-type-name", nil, "searchApiId=announceName (repeatable), e.g. 2=Ebook; unmapped ids stay their raw id")
}
