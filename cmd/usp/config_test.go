package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	kitconfig "hop.top/kit/go/core/config"
)

func TestDefaultConfig(t *testing.T) {
	c := defaultConfig()
	if c.DefaultLimit != 20 {
		t.Errorf("DefaultLimit = %d, want 20", c.DefaultLimit)
	}
	if c.DefaultCLI != "" {
		t.Errorf("DefaultCLI = %q, want empty", c.DefaultCLI)
	}
	if c.CacheTTL != "10m" {
		t.Errorf("CacheTTL = %q, want 10m", c.CacheTTL)
	}
}

func TestMergeConfigSkipsZero(t *testing.T) {
	dst := Config{DefaultCLI: "claude", DefaultLimit: 50, CacheTTL: "1m"}
	src := Config{}
	mergeConfig(&dst, &src)
	if dst.DefaultCLI != "claude" || dst.DefaultLimit != 50 || dst.CacheTTL != "1m" {
		t.Errorf("merge zero src clobbered dst: %+v", dst)
	}
}

func TestMergeConfigOverwritesNonZero(t *testing.T) {
	dst := Config{DefaultCLI: "claude", DefaultLimit: 20, CacheTTL: "10m"}
	src := Config{DefaultCLI: "codex", DefaultLimit: 50, CacheTTL: "1m"}
	mergeConfig(&dst, &src)
	if dst.DefaultCLI != "codex" {
		t.Errorf("DefaultCLI = %q, want codex", dst.DefaultCLI)
	}
	if dst.DefaultLimit != 50 {
		t.Errorf("DefaultLimit = %d, want 50", dst.DefaultLimit)
	}
	if dst.CacheTTL != "1m" {
		t.Errorf("CacheTTL = %q, want 1m", dst.CacheTTL)
	}
}

func TestLoadConfigFromProjectFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".usp.yaml")
	body := []byte("default_limit: 5\ndefault_cli: codex\ncache_ttl: 1m\n")
	if err := os.WriteFile(cfgPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	cfg, err := loadConfig(v)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.DefaultLimit != 5 {
		t.Errorf("DefaultLimit = %d, want 5", cfg.DefaultLimit)
	}
	if cfg.DefaultCLI != "codex" {
		t.Errorf("DefaultCLI = %q, want codex", cfg.DefaultCLI)
	}
	if cfg.CacheTTL != "1m" {
		t.Errorf("CacheTTL = %q, want 1m", cfg.CacheTTL)
	}
}

func TestLoadConfigEnvOverridesFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".usp.yaml")
	body := []byte("default_limit: 5\n")
	if err := os.WriteFile(cfgPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	t.Setenv("USP_DEFAULT_LIMIT", "10")
	v := viper.New()
	if _, err := loadConfig(v); err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	// AutomaticEnv exposes USP_DEFAULT_LIMIT as default_limit.
	if got := v.GetInt("default_limit"); got != 10 {
		t.Errorf("default_limit = %d, want 10 (env override)", got)
	}
}

func TestLoadConfigExtraConfigPath(t *testing.T) {
	tmp := t.TempDir()
	explicit := filepath.Join(tmp, "custom.yaml")
	body := []byte("default_limit: 99\n")
	if err := os.WriteFile(explicit, body, 0o600); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	cfg, err := loadConfigWithLayers(v, []string{explicit}, nil)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.DefaultLimit != 99 {
		t.Errorf("DefaultLimit = %d, want 99 (-c extra file)", cfg.DefaultLimit)
	}
}

func TestLoadConfigOverride(t *testing.T) {
	v := viper.New()
	cfg, err := loadConfigWithLayers(v, nil, map[string]any{
		"default_limit": 99,
	})
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.DefaultLimit != 99 {
		t.Errorf("DefaultLimit = %d, want 99 (-c key=value override)", cfg.DefaultLimit)
	}
}

func TestKitConfigLoadHandlesMissingFile(t *testing.T) {
	var c Config
	err := kitconfig.Load(&c, kitconfig.Options{
		ProjectConfigPath: "/nonexistent/path/.usp.yaml",
	})
	if err != nil {
		t.Errorf("missing file should not error: %v", err)
	}
}
