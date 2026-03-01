package docs

import "testing"

func TestResolveSlashInLaterArg(t *testing.T) {
	t.Parallel()

	// Rule 1: when any arg contains "/", all args are joined with "/".
	// If the slash is in a later arg (not args[0]), the joined result
	// is unlikely to match any real slug. This can't be proven through
	// command output because the error message uses the original args
	// joined by space, making it indistinguishable from a Rule 3 failure.
	got := Resolve([]string{"http", "k6-http/get"})
	want := "http/k6-http/get"
	if got != want {
		t.Errorf("Resolve([http, k6-http/get]) = %q, want %q", got, want)
	}
}
