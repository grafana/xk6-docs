package docs

import (
	"path/filepath"
	"testing"
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
