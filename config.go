package docs

import (
	"errors"
	"path/filepath"
)

const defaultDepth = 1

// homeDirFromEnv returns the user's home directory from environment variables.
// It checks HOME first, then USERPROFILE as a fallback (for Windows).
func homeDirFromEnv(env map[string]string) (string, error) {
	if home := env["HOME"]; home != "" {
		return home, nil
	}
	if home := env["USERPROFILE"]; home != "" {
		return home, nil
	}
	return "", errors.New("neither HOME nor USERPROFILE is set")
}

// configDir returns the directory where the docs config file lives.
// It uses $XDG_CONFIG_HOME/k6 if set, otherwise ~/.config/k6.
func configDir(env map[string]string) (string, error) {
	if xdg := env["XDG_CONFIG_HOME"]; xdg != "" {
		return filepath.Join(xdg, "k6"), nil
	}
	home, err := homeDirFromEnv(env)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "k6"), nil
}
