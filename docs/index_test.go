package docs

import "testing"

// Lookup is documented as case-insensitive. Slugs that contain uppercase
// letters (e.g. jslib method names like getShardIterator) must resolve both
// when queried exactly as stored and when queried in lower case.
func TestLookup_CaseInsensitiveForUppercaseSlug(t *testing.T) {
	t.Parallel()

	idx := &Index{Sections: []Section{
		{Slug: "javascript-api/jslib/aws/kinesisclient/getShardIterator", Title: "getShardIterator"},
	}}

	for _, q := range []string{
		"javascript-api/jslib/aws/kinesisclient/getShardIterator", // exact case
		"javascript-api/jslib/aws/kinesisclient/getsharditerator", // lower case
	} {
		if _, ok := idx.Lookup(q); !ok {
			t.Errorf("Lookup(%q) = not found, want found", q)
		}
	}
}

// Children resolves child slugs through the same map, so uppercase child
// slugs must be returned rather than silently skipped.
func TestChildren_ResolvesUppercaseChildSlug(t *testing.T) {
	t.Parallel()

	idx := &Index{Sections: []Section{
		{
			Slug:     "javascript-api/jslib/aws/kinesisclient",
			Title:    "kinesisclient",
			Children: []string{"javascript-api/jslib/aws/kinesisclient/getShardIterator"},
		},
		{Slug: "javascript-api/jslib/aws/kinesisclient/getShardIterator", Title: "getShardIterator"},
	}}

	got := idx.Children("javascript-api/jslib/aws/kinesisclient")
	if len(got) != 1 {
		t.Fatalf("Children() returned %d sections, want 1", len(got))
	}
}
