package docs

import "testing"

func TestTransformWithLinkSlugs(t *testing.T) {
	t.Parallel()

	const input = `Refer to [Running k6](https://grafana.com/docs/k6/<K6_VERSION>/get-started/running-k6/).
See [results](/docs/k6/v1.6.1/get-started/running-k6/?view=all#results).
Keep [older docs](https://grafana.com/docs/k6/v1.5.x/get-started/running-k6/) and [other resources](https://example.com/docs/k6/v1.6.x/get-started/running-k6/).`
	const want = "Refer to Running k6 (`get-started/running-k6`).\n" +
		"See results (`get-started/running-k6`).\n" +
		"Keep [older docs](https://grafana.com/docs/k6/v1.5.x/get-started/running-k6/) and " +
		"[other resources](https://example.com/docs/k6/v1.6.x/get-started/running-k6/)."

	if got := Transform(input, "v1.6.x", WithLinkSlugs()); got != want {
		t.Errorf("Transform() = %q, want %q", got, want)
	}
}
