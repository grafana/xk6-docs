package docs

import "testing"

func TestSlugResolution(t *testing.T) {
	t.Parallel()

	run, _ := setupCommand(t)

	t.Run("full_slug", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-get.txt", run(t, "javascript-api/k6-http/get"))
	})
	t.Run("category_prefix", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/using-k6-scenarios.txt", run(t, "using-k6", "scenarios"))
	})
	t.Run("js_api_shortcut", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-get.txt", run(t, "http", "get"))
	})
	t.Run("case_insensitive", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-get.txt", run(t, "HTTP", "GET"))
	})
	t.Run("k6_prefix_dedup", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-get.txt", run(t, "k6-http", "get"))
	})
	t.Run("parent_prefix_fallback", func(t *testing.T) {
		t.Parallel()
		assertGolden(t, "view/http-cookiejar-clear.txt", run(t, "http", "cookiejar", "clear"))
	})
	t.Run("bare_name_prefers_unprefixed", func(t *testing.T) {
		t.Parallel()
		// Both javascript-api/jslib and javascript-api/k6-jslib exist.
		// ResolveWithLookup tries unprefixed first, so "jslib" resolves to
		// javascript-api/jslib (not javascript-api/k6-jslib).
		assertGolden(t, "view/jslib.txt", run(t, "jslib"))
	})
}
