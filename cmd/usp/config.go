package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	kitcliconfig "hop.top/kit/go/console/cli/config"
	kitconfig "hop.top/kit/go/core/config"
	"hop.top/kit/go/core/xdg"
)

// Config is the persisted shape of usp's config file.
//
// Layers (lowest precedence to highest):
//
//  1. defaults
//  2. /etc/usp/config.yaml
//  3. $XDG_CONFIG_HOME/usp/config.yaml
//  4. ./.usp.yaml
//  5. --config <path> override
//  6. env (USP_*)
//  7. CLI flags
type Config struct {
	DefaultTool  string `yaml:"default_tool"`
	DefaultLimit int    `yaml:"default_limit"`
}

// defaultConfig returns the baseline values for a fresh install.
func defaultConfig() Config {
	return Config{DefaultTool: "", DefaultLimit: 20}
}

// registerConfigGlobals adds --config and --offline persistent flags to
// root and binds them to viper. Call after cli.New.
func registerConfigGlobals(cmd *cobra.Command, v *viper.Viper) {
	cmd.PersistentFlags().String("config", "",
		"Path to YAML config file (overrides standard search)")
	cmd.PersistentFlags().Bool("offline", false,
		"Disable network operations")
	_ = v.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))
	_ = v.BindPFlag("offline", cmd.PersistentFlags().Lookup("offline"))
}

// loadConfig resolves the layered config and merges it into rootViper
// as default values for any keys that aren't already set by flags or
// env. Errors during file parsing are returned; missing files are
// silently skipped per kit/config semantics.
func loadConfig(v *viper.Viper) (Config, error) {
	cfg := defaultConfig()

	userPath := ""
	if dir, err := xdg.ConfigDir("usp"); err == nil {
		userPath = filepath.Join(dir, "config.yaml")
	}

	projectPath := ""
	if cwd, err := os.Getwd(); err == nil {
		projectPath = filepath.Join(cwd, ".usp.yaml")
	}

	opts := kitconfig.Options{
		SystemConfigPath:  "/etc/usp/config.yaml",
		UserConfigPath:    userPath,
		ProjectConfigPath: projectPath,
	}
	if err := kitconfig.Load(&cfg, opts); err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}

	// Explicit --config override wins over the search path. Re-run with
	// only that path as the project layer so its values overwrite the
	// merged result.
	if explicit := v.GetString("config"); explicit != "" {
		var override Config
		if err := kitconfig.Load(&override, kitconfig.Options{
			ProjectConfigPath: explicit,
		}); err != nil {
			return cfg, fmt.Errorf("load --config %s: %w", explicit, err)
		}
		mergeConfig(&cfg, &override)
	}

	// Env vars override config files. USP_DEFAULT_TOOL, USP_DEFAULT_LIMIT.
	v.SetEnvPrefix("USP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Surface defaults via viper so command flags pick them up when unset.
	if !v.IsSet("default_tool") {
		v.SetDefault("default_tool", cfg.DefaultTool)
	}
	if !v.IsSet("default_limit") {
		v.SetDefault("default_limit", cfg.DefaultLimit)
	}
	return cfg, nil
}

// mergeConfig copies non-zero fields from src to dst.
func mergeConfig(dst, src *Config) {
	if src.DefaultTool != "" {
		dst.DefaultTool = src.DefaultTool
	}
	if src.DefaultLimit != 0 {
		dst.DefaultLimit = src.DefaultLimit
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
	abs, err := filepath.Abs(cwd)
	if err != nil {
		abs = cwd
	}
	out := make([]kitcliconfig.ResolvedPath, 0, 4)

	projectPath := filepath.Join(abs, ".usp.yaml")
	out = append(out, kitcliconfig.ResolvedPath{
		Path:   projectPath,
		Source: "project",
		Scope:  "project",
		Exists: regularFileExists(projectPath),
	})

	if dir, err := xdg.ConfigDir("usp"); err == nil {
		userPath := filepath.Join(dir, "config.yaml")
		out = append(out, kitcliconfig.ResolvedPath{
			Path:   userPath,
			Source: "user",
			Scope:  "user",
			Exists: regularFileExists(userPath),
		})
	}

	const sysPath = "/etc/usp/config.yaml"
	out = append(out, kitcliconfig.ResolvedPath{
		Path:   sysPath,
		Source: "system",
		Scope:  "system",
		Exists: regularFileExists(sysPath),
	})

	out = append(out, kitcliconfig.ResolvedPath{
		Path:   "<defaults>",
		Source: "default",
		Scope:  "default",
		Exists: true,
	})
	return out
}

// regularFileExists reports whether path resolves to a regular file.
func regularFileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular()
}
