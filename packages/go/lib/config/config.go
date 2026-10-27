// Package config loads a tool's configuration from the layers this repo's
// packages already use: values baked into the destination struct, an optional
// JSON file, environment variables, and command-line flags.
//
// Precedence runs lowest to highest in that order. Every layer is optional, so
// a tool that only takes flags passes only a FlagSet, and one configured
// entirely by systemd environment passes only an EnvPrefix.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	"github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
)

// Delim separates nested keys.
//
// Nesting is spelled explicitly so that a key may itself contain an
// underscore, which is common in JSON config and would otherwise be
// indistinguishable from a level boundary:
//
//	json      env               flag
//	base_url  HOMELAB_BASE_URL  --base-url
//	log.level HOMELAB_LOG__LEVEL --log.level
//
// That is, a double underscore descends a level in the environment and a dot
// does the same in a flag name, mirroring Prometheus' --web.listen-address
// convention. A single underscore or dash is part of the key.
const Delim = "."

// Options selects which layers to load and where they come from.
type Options struct {
	// EnvPrefix is the environment variable prefix, e.g. "HOMELAB_". Empty
	// skips the environment layer. The prefix is stripped, the remainder
	// lowercased, and double underscores become key separators.
	EnvPrefix string
	// FilePath is a JSON config file. Empty skips the file layer.
	FilePath string
	// FileOptional suppresses the error when FilePath does not exist,
	// for tools whose config file is a deployment-time opt-in.
	FileOptional bool
	// Flags is a parsed flag set. Empty skips the flag layer. Only flags
	// the user actually set take precedence over env and file, so a flag
	// default never silently overrides deployed configuration.
	Flags *pflag.FlagSet
	// Tag is the struct tag Load unmarshals by. Empty means "json", which
	// keeps one set of field names across the file and the struct.
	Tag string
}

// ErrNoDestination reports a nil or non-pointer destination.
var ErrNoDestination = errors.New("config: destination must be a non-nil pointer")

// Load fills dst from the layers named in opts.
//
// dst must be a pointer to a struct. Fields left unset by every layer keep the
// value they had on entry, which is how defaults are expressed: populate dst
// before calling.
func Load(dst any, opts Options) error {
	if dst == nil {
		return ErrNoDestination
	}

	k := koanf.New(Delim)

	if opts.FilePath != "" {
		if err := loadFile(k, opts); err != nil {
			return err
		}
	}

	if opts.EnvPrefix != "" {
		provider := env.Provider(opts.EnvPrefix, Delim, func(s string) string {
			trimmed := strings.ToLower(strings.TrimPrefix(s, opts.EnvPrefix))
			return strings.ReplaceAll(trimmed, "__", Delim)
		})
		if err := k.Load(provider, nil); err != nil {
			return fmt.Errorf("config: load environment: %w", err)
		}
	}

	if opts.Flags != nil {
		// Passing k as posflag's reference set makes it merge a flag only
		// when the user actually changed it, or when no earlier layer
		// supplied the key. Without this a flag's default would clobber
		// deployed file and environment configuration on every run.
		//
		// A dash is part of the key, a dot descends a level: --base-url
		// addresses base_url and --log.level addresses log.level.
		flagKey := func(f *pflag.Flag) (string, any) {
			key := strings.ReplaceAll(f.Name, "-", "_")
			return key, posflag.FlagVal(opts.Flags, f)
		}
		if err := k.Load(posflag.ProviderWithFlag(opts.Flags, Delim, k, flagKey), nil); err != nil {
			return fmt.Errorf("config: load flags: %w", err)
		}
	}

	tag := opts.Tag
	if tag == "" {
		tag = "json"
	}
	unmarshalOpts := koanf.UnmarshalConf{Tag: tag}
	if err := k.UnmarshalWithConf("", dst, unmarshalOpts); err != nil {
		return fmt.Errorf("config: unmarshal: %w", err)
	}
	return nil
}

// loadFile applies the JSON file layer, honouring FileOptional.
func loadFile(k *koanf.Koanf, opts Options) error {
	if opts.FileOptional {
		if _, err := os.Stat(opts.FilePath); errors.Is(err, os.ErrNotExist) {
			return nil
		}
	}
	if err := k.Load(file.Provider(opts.FilePath), json.Parser()); err != nil {
		return fmt.Errorf("config: load %s: %w", opts.FilePath, err)
	}
	return nil
}
