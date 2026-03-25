package docs

import "testing"

func TestResolveWithLookup(t *testing.T) {
	t.Parallel()

	known := map[string]bool{
		"javascript-api":             true,
		"javascript-api/k6-mod-a/fn": true,
		"javascript-api/k6-mod-b":    true,
		"javascript-api/lib-c":       true,
		"cat-x":                      true,
		"cat-x/topic-1":              true,
	}
	exists := func(s string) bool { return known[s] }

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "slash_shorthand",
			args: []string{"mod-a/fn"},
			want: "javascript-api/k6-mod-a/fn",
		},
		{
			name: "space_shorthand",
			args: []string{"mod-a", "fn"},
			want: "javascript-api/k6-mod-a/fn",
		},
		{
			name: "full_slug",
			args: []string{"javascript-api/k6-mod-a/fn"},
			want: "javascript-api/k6-mod-a/fn",
		},
		{
			name: "full_slug_without_k6_prefix",
			args: []string{"javascript-api/mod-b"},
			want: "javascript-api/k6-mod-b",
		},
		{
			name: "existing_doc_prioritized_over_k6_prefix",
			args: []string{"lib-c"},
			want: "javascript-api/lib-c",
		},
		{
			name: "category_slash",
			args: []string{"cat-x/topic-1"},
			want: "cat-x/topic-1",
		},
		{
			name: "category_resolved_from_index",
			args: []string{"cat-x"},
			want: "cat-x",
		},
		{
			name: "category_subtopic_from_index",
			args: []string{"cat-x", "topic-1"},
			want: "cat-x/topic-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ResolveWithLookup(tt.args, exists)
			if got != tt.want {
				t.Errorf("ResolveWithLookup(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
