package docs

import "errors"

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
