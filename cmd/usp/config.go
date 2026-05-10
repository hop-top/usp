package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	kitcliconfig "hop.top/kit/go/console/cli/config"
	kitconfig "hop.top/kit/go/core/config"
)

// Config is the persisted shape of usp's config file.
//
// Layers (lowest precedence to highest):
//
//  1. defaults
//  2. /etc/usp/config.yaml
//  3. $XDG_CONFIG_HOME/usp/config.yaml
//  4. ./.usp.yaml
//  5. -c/--config extra file layers
//  6. env (USP_*)
//  7. -c/--config key=value overrides
//  8. CLI flags
type Config struct {
	DefaultCLI   string `yaml:"default_cli"`
	DefaultLimit int    `yaml:"default_limit"`
	CacheTTL     string `yaml:"cache_ttl"`
}

var uspConfigMarkers = []string{".usp.yaml", ".usp/config.yaml"}

// defaultConfig returns the baseline values for a fresh install.
func defaultConfig() Config {
	return Config{DefaultCLI: "", DefaultLimit: 20, CacheTTL: "10m"}
}

// loadConfig resolves the layered config and merges it into rootViper
// as default values for any keys that aren't already set by flags or
// env. Errors during file parsing are returned; missing files are
// silently skipped per kit/config semantics.
func loadConfig(v *viper.Viper) (Config, error) {
	return loadConfigWithLayers(v, nil, nil)
}

func loadConfigWithLayers(
	v *viper.Viper,
	extraPaths []string,
	overrides map[string]any,
) (Config, error) {
	cfg := Config{}
	opts := kitconfig.OptionsForToolWithMarkers("usp", uspConfigMarkers)
	opts.Defaults = defaultConfig()
	opts.ExtraConfigPaths = extraPaths
	opts.Overrides = overrides
	opts.Viper = v
	opts.EnvPrefix = "USP"
	if err := kitconfig.Load(&cfg, opts); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	v.SetDefault("default_cli", cfg.DefaultCLI)
	v.SetDefault("default_limit", cfg.DefaultLimit)
	v.SetDefault("cache_ttl", cfg.CacheTTL)
	return cfg, nil
}

// mergeConfig copies non-zero fields from src to dst.
func mergeConfig(dst, src *Config) {
	if src.DefaultCLI != "" {
		dst.DefaultCLI = src.DefaultCLI
	}
	if src.DefaultLimit != 0 {
		dst.DefaultLimit = src.DefaultLimit
	}
	if src.CacheTTL != "" {
		dst.CacheTTL = src.CacheTTL
	}
}

// configCmd returns the `config` parent under MANAGEMENT. Hosts kit's
// shared `path` and `paths` introspection subcommands so usp matches
// `git config --list --show-origin`-style discovery.
func configCmd(_ *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect usp configuration",
		Args:  cobra.NoArgs,
	}
	kitcliconfig.RegisterPathSubcommands(cmd, "usp",
		kitcliconfig.WithResolver(uspConfigPathsResolver))
	return cmd
}

// uspConfigPathsResolver mirrors loadConfig's precedence chain (project
// → user → system → defaults) for the kit-shared `config paths` cmd.
// Highest precedence first.
func uspConfigPathsResolver(cwd string) []kitcliconfig.ResolvedPath {
	raw := kitconfig.PathsForToolWithMarkers(cwd, "usp", uspConfigMarkers)
	out := make([]kitcliconfig.ResolvedPath, 0, len(raw))
	for _, r := range raw {
		out = append(out, kitcliconfig.ResolvedPath{
			Path:   r.Path,
			Source: r.Source,
			Scope:  r.Scope,
			Exists: r.Exists,
		})
	}
	return out
}
