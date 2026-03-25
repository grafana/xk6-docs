package docs

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteTopicArgs(t *testing.T) {
	t.Parallel()

	afs, dir := setupTestCache(t)
	idx, err := LoadIndex(afs, dir)
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	t.Run("zero args empty toComplete returns all", func(t *testing.T) {
		t.Parallel()
		comps, dir := completionTopicArgs(idx, nil, "")
		if dir != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %d, want %d", dir, cobra.ShellCompDirectiveNoFileComp)
		}
		got := completionValues(comps)
		for _, want := range []string{"javascript-api", "alpha", "beta", "mod-a", "mod-b", "lib-c", "best-practices"} {
			if !slices.Contains(got, want) {
				t.Errorf("missing %q in completions: %v", want, got)
			}
		}
	})

	t.Run("zero args toComplete=mo returns mod-a and mod-b", func(t *testing.T) {
		t.Parallel()
		comps, _ := completionTopicArgs(idx, nil, "mo")
		got := completionValues(comps)
		if !slices.Contains(got, "mod-a") {
			t.Errorf("expected mod-a, got %v", got)
		}
		if !slices.Contains(got, "mod-b") {
			t.Errorf("expected mod-b, got %v", got)
		}
		if slices.Contains(got, "alpha") || slices.Contains(got, "beta") {
			t.Errorf("unexpected completions: %v", got)
		}
	})

	t.Run("zero args toComplete=al returns alpha", func(t *testing.T) {
		t.Parallel()
		comps, _ := completionTopicArgs(idx, nil, "al")
		got := completionValues(comps)
		if !slices.Contains(got, "alpha") {
			t.Errorf("expected alpha, got %v", got)
		}
		if slices.Contains(got, "beta") {
			t.Errorf("unexpected completions: %v", got)
		}
	})

	t.Run("one arg mod-a returns children", func(t *testing.T) {
		t.Parallel()
		comps, _ := completionTopicArgs(idx, []string{"mod-a"}, "")
		got := completionValues(comps)
		for _, want := range []string{"fn-one", "fn-two", "child-a"} {
			if !slices.Contains(got, want) {
				t.Errorf("missing %q in completions: %v", want, got)
			}
		}
	})

	t.Run("one arg mod-a toComplete=f returns fn-one and fn-two", func(t *testing.T) {
		t.Parallel()
		comps, _ := completionTopicArgs(idx, []string{"mod-a"}, "f")
		got := completionValues(comps)
		if !slices.Contains(got, "fn-one") {
			t.Errorf("expected fn-one, got %v", got)
		}
		if !slices.Contains(got, "fn-two") {
			t.Errorf("expected fn-two, got %v", got)
		}
		if slices.Contains(got, "child-a") {
			t.Errorf("unexpected completions: %v", got)
		}
	})

	t.Run("one arg alpha returns children", func(t *testing.T) {
		t.Parallel()
		comps, _ := completionTopicArgs(idx, []string{"alpha"}, "")
		got := completionValues(comps)
		if !slices.Contains(got, "topic-one") {
			t.Errorf("expected topic-one, got %v", got)
		}
	})

	t.Run("two args mod-a child-a returns children", func(t *testing.T) {
		t.Parallel()
		comps, _ := completionTopicArgs(idx, []string{"mod-a", "child-a"}, "")
		got := completionValues(comps)
		if !slices.Contains(got, "clear") {
			t.Errorf("expected clear, got %v", got)
		}
	})

	t.Run("nil index returns empty", func(t *testing.T) {
		t.Parallel()
		comps, dir := completionTopicArgs(nil, nil, "")
		if len(comps) != 0 {
			t.Errorf("expected no completions, got %v", comps)
		}
		if dir != cobra.ShellCompDirectiveNoFileComp {
			t.Errorf("directive = %d, want %d", dir, cobra.ShellCompDirectiveNoFileComp)
		}
	})

	t.Run("unresolvable args returns empty", func(t *testing.T) {
		t.Parallel()
		comps, _ := completionTopicArgs(idx, []string{"nonexistent-topic-xyz"}, "")
		if len(comps) != 0 {
			t.Errorf("expected no completions, got %v", comps)
		}
	})

	t.Run("all results have NoFileComp directive", func(t *testing.T) {
		t.Parallel()
		cases := []struct {
			args       []string
			toComplete string
		}{
			{nil, ""},
			{nil, "mo"},
			{[]string{"mod-a"}, ""},
			{[]string{"mod-a"}, "f"},
			{[]string{"alpha"}, ""},
		}
		for _, tc := range cases {
			_, dir := completionTopicArgs(idx, tc.args, tc.toComplete)
			if dir != cobra.ShellCompDirectiveNoFileComp {
				t.Errorf("args=%v toComplete=%q: directive = %d, want %d",
					tc.args, tc.toComplete, dir, cobra.ShellCompDirectiveNoFileComp)
			}
		}
	})
}

func completionValues(comps []cobra.Completion) []string {
	vals := make([]string, len(comps))
	for i, c := range comps {
		vals[i] = strings.SplitN(c, "\t", 2)[0]
	}
	return vals
}
