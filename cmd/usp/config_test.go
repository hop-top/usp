package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	kitconfig "hop.top/kit/config"
)

func TestDefaultConfig(t *testing.T) {
	c := defaultConfig()
	if c.DefaultLimit != 20 {
		t.Errorf("DefaultLimit = %d, want 20", c.DefaultLimit)
	}
	if c.DefaultTool != "" {
		t.Errorf("DefaultTool = %q, want empty", c.DefaultTool)
	}
}

func TestMergeConfigSkipsZero(t *testing.T) {
	dst := Config{DefaultTool: "claude", DefaultLimit: 50}
	src := Config{}
	mergeConfig(&dst, &src)
	if dst.DefaultTool != "claude" || dst.DefaultLimit != 50 {
		t.Errorf("merge zero src clobbered dst: %+v", dst)
	}
}

func TestMergeConfigOverwritesNonZero(t *testing.T) {
	dst := Config{DefaultTool: "claude", DefaultLimit: 20}
	src := Config{DefaultTool: "codex", DefaultLimit: 50}
	mergeConfig(&dst, &src)
	if dst.DefaultTool != "codex" {
		t.Errorf("DefaultTool = %q, want codex", dst.DefaultTool)
	}
	if dst.DefaultLimit != 50 {
		t.Errorf("DefaultLimit = %d, want 50", dst.DefaultLimit)
	}
}

func TestLoadConfigFromProjectFile(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, ".usp.yaml")
	body := []byte("default_limit: 5\ndefault_tool: codex\n")
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
	if cfg.DefaultTool != "codex" {
		t.Errorf("DefaultTool = %q, want codex", cfg.DefaultTool)
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

func TestLoadConfigExplicitConfigPath(t *testing.T) {
	tmp := t.TempDir()
	explicit := filepath.Join(tmp, "custom.yaml")
	body := []byte("default_limit: 99\n")
	if err := os.WriteFile(explicit, body, 0o600); err != nil {
		t.Fatal(err)
	}

	v := viper.New()
	v.Set("config", explicit)
	cfg, err := loadConfig(v)
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.DefaultLimit != 99 {
		t.Errorf("DefaultLimit = %d, want 99 (--config override)", cfg.DefaultLimit)
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
