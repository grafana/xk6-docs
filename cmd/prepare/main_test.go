package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func buildPrepareBinary(ctx context.Context, t *testing.T) string {
	t.Helper()

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("cache dir: %v", err)
	}
	dir := filepath.Join(cacheDir, "xk6-docs-test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	bin := filepath.Join(dir, "prepare")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	build.Dir, _ = os.Getwd()
	out, err := build.CombinedOutput()
	if err != nil {
		t.Fatalf("build prepare: %v\n%s", err, out)
	}
	return bin
}

func TestScripts(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	bin := buildPrepareBinary(ctx, t)

	testscript.Run(t, testscript.Params{
		Dir: "testdata/scripts",
		Setup: func(env *testscript.Env) error {
			if err := os.Symlink(bin, filepath.Join(env.WorkDir, "prepare")); err != nil {
				return err
			}
			// Copy mock docs for tests that need them.
			return copyDir("testdata/mockdocs", filepath.Join(env.WorkDir, "mockdocs"))
		},
		Cmds: map[string]func(ts *testscript.TestScript, neg bool, args []string){
			"gitinit": func(ts *testscript.TestScript, neg bool, args []string) {
				runGitInit(ctx, ts, neg, args)
			},
		},
		UpdateScripts: os.Getenv("UPDATE_GOLDEN") != "",
	})
}

// runGitInit creates a minimal local git repo for clone tests.
// Usage: gitinit <dir>
func runGitInit(ctx context.Context, ts *testscript.TestScript, _ bool, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: gitinit <dir>")
	}
	dir := ts.MkAbs(args[0])

	if err := os.MkdirAll(filepath.Join(dir, "docs"), 0o755); err != nil {
		ts.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", ".gitkeep"), nil, 0o644); err != nil {
		ts.Fatalf("write: %v", err)
	}

	for _, cmdArgs := range [][]string{
		{"git", "init"},
		{"git", "add", "."},
		{"git", "-c", "user.name=test", "-c", "user.email=test@test", "-c", "commit.gpgsign=false", "commit", "-m", "init"},
	} {
		cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			ts.Fatalf("%v: %v\n%s", cmdArgs, err, out)
		}
	}
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}
