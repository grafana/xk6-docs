package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/grafana/xk6-docs/docs"
	"github.com/grafana/xk6-docs/internal/bundle"
)

// failingTransport makes every HTTP request fail, forcing fetchV6Spec to fall
// back to the embedded v6 spec without any network access.
type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("offline")
}

// TestRun_RESTSectionEmbeddedFallback runs the prepare pipeline against the
// mock docs tree with an HTTP client that always fails. This exercises the
// embedded-spec fallback (no real network) and asserts the generated
// cloud-rest-api section, including a rendered v6 endpoint, lands in the
// bundle.
func TestRun_RESTSectionEmbeddedFallback(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	afs := bundle.NewOSFS()
	client := &http.Client{Transport: failingTransport{}}
	if err := run("v0.99.x", "testdata/mockdocs", out, client, afs, io.Discard); err != nil {
		t.Fatalf("run: %v", err)
	}

	data, err := afs.ReadFile(filepath.Join(out, "sections.json"))
	if err != nil {
		t.Fatalf("read sections.json: %v", err)
	}
	var idx docs.Index
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("unmarshal sections.json: %v", err)
	}

	haveIndex := false
	var leaf *docs.Section
	for i := range idx.Sections {
		s := &idx.Sections[i]
		if s.Slug == "cloud-rest-api" {
			haveIndex = true
		}
		if leaf == nil && strings.HasPrefix(s.Slug, "cloud-rest-api/v6/") {
			leaf = s
		}
	}
	if !haveIndex {
		t.Error("missing cloud-rest-api index section")
	}
	if leaf == nil {
		t.Fatal("no cloud-rest-api/v6 endpoint section generated from embedded spec")
	}

	body, err := afs.ReadFile(filepath.Join(out, "markdown", filepath.FromSlash(leaf.RelPath)))
	if err != nil {
		t.Fatalf("read leaf markdown: %v", err)
	}
	if !strings.Contains(string(body), "## Invocation") {
		t.Errorf("rendered endpoint %q missing Invocation section", leaf.Slug)
	}
}
