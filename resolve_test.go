package docs

import "testing"

func TestResolveSlashInArg(t *testing.T) {
	t.Parallel()

	known := map[string]bool{
		"javascript-api/k6-browser/elementhandle": true,
		"javascript-api/k6-http/get":              true,
		"javascript-api/jslib":                    true,
		"using-k6/scenarios":                      true,
	}
	exists := func(s string) bool { return known[s] }

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "slash_shorthand",
			args: []string{"browser/elementhandle"},
			want: "javascript-api/k6-browser/elementhandle",
		},
		{
			name: "space_shorthand",
			args: []string{"browser", "elementhandle"},
			want: "javascript-api/k6-browser/elementhandle",
		},
		{
			name: "full_slug",
			args: []string{"javascript-api/k6-http/get"},
			want: "javascript-api/k6-http/get",
		},
		{
			name: "full_slug_without_k6_prefix",
			args: []string{"javascript-api/browser/elementhandle"},
			want: "javascript-api/k6-browser/elementhandle",
		},
		{
			name: "existing_doc_prioritized_over_k6_prefix",
			args: []string{"jslib"},
			want: "javascript-api/jslib",
		},
		{
			name: "category_slash",
			args: []string{"using-k6/scenarios"},
			want: "using-k6/scenarios",
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
