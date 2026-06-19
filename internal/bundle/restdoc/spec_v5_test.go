package restdoc

import (
	"net/http"
	"testing"
)

// expectedV5OperationIDs is the contract for the hand-authored v5 spec.
// Any rename or addition needs both the YAML and this list updated.
//
//nolint:gochecknoglobals // table-driven test data
var expectedV5OperationIDs = []string{
	"test_run_metrics_list",
	"load_test_metrics_list",
	"load_test_metrics_list_by_count",
	"load_test_metrics_list_by_ids",
	"load_test_metrics_ms_alias_list",
	"load_test_metrics_ms_alias_by_count",
	"load_test_metrics_ms_alias_by_ids",
	"test_run_series_list",
	"test_run_labels_list",
	"test_run_label_values_list",
	"test_run_query_range_k6",
	"test_run_query_aggregate_k6",
	"load_test_query_aggregate_k6",
}

func TestLoadSpecFromBytes_V5_OperationCoverage(t *testing.T) {
	t.Parallel()

	spec, err := LoadSpecFromBytes(embeddedV5Spec)
	if err != nil {
		t.Fatalf("LoadSpecFromBytes(v5): %v", err)
	}

	if got, want := len(spec.Operations), len(expectedV5OperationIDs); got != want {
		t.Errorf("v5 operation count: got %d, want %d", got, want)
	}

	for _, id := range expectedV5OperationIDs {
		op := spec.ByID(id)
		if op == nil {
			t.Errorf("v5 operation %q not found", id)
			continue
		}
		if op.Method != http.MethodGet {
			t.Errorf("v5 operation %q method: got %q, want GET", id, op.Method)
		}
		if len(op.Security) == 0 {
			t.Errorf("v5 operation %q has no security; expected k6ApiToken", id)
		}
	}
}

func TestLoadSpecFromBytes_V5_ResponseRefsResolve(t *testing.T) {
	t.Parallel()

	spec, err := LoadSpecFromBytes(embeddedV5Spec)
	if err != nil {
		t.Fatalf("LoadSpecFromBytes(v5): %v", err)
	}

	// Every 200 response should resolve to a SchemaName from
	// components.schemas, exercising the spec's $ref plumbing. Every
	// 401/404 reference should resolve to a non-empty Description from
	// components.responses.
	for i := range spec.Operations {
		op := &spec.Operations[i]
		for _, r := range op.Responses {
			switch r.Status {
			case "200":
				if r.SchemaName == "" {
					t.Errorf("v5 %s: 200 has empty SchemaName", op.OperationID)
				}
			case "401", "404":
				if r.Description == "" {
					t.Errorf("v5 %s: %s response has empty description (ref not resolved?)",
						op.OperationID, r.Status)
				}
			}
		}
	}
}
