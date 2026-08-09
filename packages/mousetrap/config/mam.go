package config

import (
	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerMAM binds the MyAnonaMouse source-client flags.
func registerMAM(fs *flag.FlagSet, c *types.MAMConfig) {
	fs.StringVar(&c.APIBase, "mam-api-base", "https://www.myanonamouse.net", "MAM API base URL")
	fs.StringVar(&c.SeedPath, "mam-seed-path", "", "path to the mam_id seed secret (path, not value)")
}
