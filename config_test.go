package docs

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"go.k6.io/k6/lib/fsext"
)

func TestConfigDir(t *testing.T) {
	t.Parallel()

	t.Run("XDG_CONFIG_HOME has priority", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"XDG_CONFIG_HOME": "/xdg",
			"HOME":            "/home/fallback",
			"USERPROFILE":     "/users/fallback",
		}
		dir, err := configDir(env)
		if err != nil {
			t.Fatalf("configDir: unexpected error: %v", err)
		}
		want := filepath.Join("/xdg", "k6")
		if dir != want {
			t.Errorf("configDir = %q, want %q", dir, want)
		}
	})

	t.Run("HOME preferred over USERPROFILE", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{
			"HOME":        "/home/test",
			"USERPROFILE": "/users/test",
		}
		dir, err := configDir(env)
		if err != nil {
			t.Fatalf("configDir: unexpected error: %v", err)
		}
		want := filepath.Join("/home/test", ".config", "k6")
		if dir != want {
			t.Errorf("configDir = %q, want %q", dir, want)
		}
	})

	t.Run("USERPROFILE fallback when HOME is unset", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{"USERPROFILE": "/users/test"}
		dir, err := configDir(env)
		if err != nil {
			t.Fatalf("configDir: unexpected error: %v", err)
		}
		want := filepath.Join("/users/test", ".config", "k6")
		if dir != want {
			t.Errorf("configDir = %q, want %q", dir, want)
		}
	})

	t.Run("error when neither HOME nor USERPROFILE is set", func(t *testing.T) {
		t.Parallel()

		env := map[string]string{"XDG_CONFIG_HOME": ""}
		_, err := configDir(env)
		if err == nil {
			t.Fatal("configDir: expected error when neither HOME nor USERPROFILE is set")
		}
	})
}

func TestCacheDirUSERPROFILE(t *testing.T) {
	t.Parallel()

	env := map[string]string{"USERPROFILE": "/users/test"}
	dir, err := CacheDir(env, "v1.0.0")
	if err != nil {
		t.Fatalf("CacheDir: unexpected error: %v", err)
	}
	want := filepath.Join("/users/test", ".local", "share", "k6", "docs", "v1.0.0")
	if dir != want {
		t.Errorf("CacheDir = %q, want %q", dir, want)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		afs := fsext.NewMemMapFs()
		dir := "/tmp/config"
		env := map[string]string{"XDG_CONFIG_HOME": dir}

		k6Dir := filepath.Join(dir, "k6")
		if err := afs.MkdirAll(k6Dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fsext.WriteFile(afs, filepath.Join(k6Dir, "docs.yaml"), []byte("renderer: glow -p 200\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := loadConfig(afs, env)
		if err != nil {
			t.Fatalf("loadConfig: unexpected error: %v", err)
		}
		if cfg.Renderer != "glow -p 200" {
			t.Errorf("loadConfig: Renderer = %q, want %q", cfg.Renderer, "glow -p 200")
		}
	})

	t.Run("missing file returns empty config", func(t *testing.T) {
		t.Parallel()
		afs := fsext.NewMemMapFs()
		dir := "/tmp/config-missing"
		env := map[string]string{"XDG_CONFIG_HOME": dir}

		cfg, err := loadConfig(afs, env)
		if err != nil {
			t.Fatalf("loadConfig: unexpected error: %v", err)
		}
		if cfg.Renderer != "" {
			t.Errorf("loadConfig: Renderer = %q, want empty", cfg.Renderer)
		}
	})

	t.Run("invalid YAML returns error", func(t *testing.T) {
		t.Parallel()
		afs := fsext.NewMemMapFs()
		dir := "/tmp/config-invalid"
		env := map[string]string{"XDG_CONFIG_HOME": dir}

		k6Dir := filepath.Join(dir, "k6")
		if err := afs.MkdirAll(k6Dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fsext.WriteFile(afs, filepath.Join(k6Dir, "docs.yaml"), []byte(":\n  :\n    : [invalid"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := loadConfig(afs, env)
		if err == nil {
			t.Fatal("loadConfig: expected error for invalid YAML, got nil")
		}
		if cfg.Renderer != "" {
			t.Errorf("loadConfig: Renderer = %q on error, want empty", cfg.Renderer)
		}
	})

	t.Run("config via HOME fallback", func(t *testing.T) {
		t.Parallel()
		afs := fsext.NewMemMapFs()
		home := "/home/testuser"
		env := map[string]string{"HOME": home}

		k6Dir := filepath.Join(home, ".config", "k6")
		if err := afs.MkdirAll(k6Dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fsext.WriteFile(afs, filepath.Join(k6Dir, "docs.yaml"), []byte("renderer: glow -w 200\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := loadConfig(afs, env)
		if err != nil {
			t.Fatalf("loadConfig: unexpected error: %v", err)
		}
		if cfg.Renderer != "glow -w 200" {
			t.Errorf("loadConfig: Renderer = %q, want %q", cfg.Renderer, "glow -w 200")
		}
	})

	t.Run("empty renderer field", func(t *testing.T) {
		t.Parallel()
		afs := fsext.NewMemMapFs()
		dir := "/tmp/config-empty"
		env := map[string]string{"XDG_CONFIG_HOME": dir}

		k6Dir := filepath.Join(dir, "k6")
		if err := afs.MkdirAll(k6Dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := fsext.WriteFile(afs, filepath.Join(k6Dir, "docs.yaml"), []byte("renderer: \"\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		cfg, err := loadConfig(afs, env)
		if err != nil {
			t.Fatalf("loadConfig: unexpected error: %v", err)
		}
		if cfg.Renderer != "" {
			t.Errorf("loadConfig: Renderer = %q, want empty", cfg.Renderer)
		}
	})
}

func TestRendererUsedWhenConfigured(t *testing.T) {
	t.Parallel()

	afs, cacheDir := setupTestCache(t)
	gs := newTestGlobalState(t, afs)
	gs.Env["XDG_CONFIG_HOME"] = "/tmp/renderer-used-config"
	gs.Stdout.IsTTY = true

	k6Dir := filepath.Join(gs.Env["XDG_CONFIG_HOME"], "k6")
	if err := afs.MkdirAll(k6Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fsext.WriteFile(afs, filepath.Join(k6Dir, "docs.yaml"), []byte("renderer: cat -n\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdoutBuf bytes.Buffer
	gs.Stdout.Writer = &stdoutBuf

	cmd := newCmd(gs)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--cache-dir", cacheDir, "--version", "v0.55.x", "http", "get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	out := stdoutBuf.String()
	if !strings.Contains(out, "http.get") {
		t.Errorf("expected topic content, got: %s", out)
	}
	if !strings.Contains(out, "\t") {
		t.Error("expected renderer (cat -n) to add tab characters, but output has none")
	}
}

func TestRendererNotUsedWhenNotConfigured(t *testing.T) {
	t.Parallel()

	afs, cacheDir := setupTestCache(t)
	gs := newTestGlobalState(t, afs)
	gs.Stdout.IsTTY = true

	var stdoutBuf bytes.Buffer
	gs.Stdout.Writer = &stdoutBuf

	cmd := newCmd(gs)
	var cmdBuf bytes.Buffer
	cmd.SetOut(&cmdBuf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--cache-dir", cacheDir, "--version", "v0.55.x", "http", "get"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("cmd.Execute: %v", err)
	}

	if !strings.Contains(cmdBuf.String(), "http.get(url)") {
		t.Errorf("expected raw output in cmd buffer, got: %s", cmdBuf.String())
	}
	if stdoutBuf.Len() != 0 {
		t.Errorf("expected renderer stdout to be empty, got: %s", stdoutBuf.String())
	}
}
