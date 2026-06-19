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

// fixedTransport returns a 200 response with a fixed body for any request,
// simulating an endpoint that answers but with unusable content.
type fixedTransport struct{ body string }

func (t fixedTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Header:     make(http.Header),
	}, nil
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

// TestRun_InvalidFetchedSpecFallsBackToEmbedded verifies that a 200 response
// whose body parses but has zero operations is rejected, and the embedded v6
// spec is used instead of shipping an empty v6 section.
func TestRun_InvalidFetchedSpecFallsBackToEmbedded(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	afs := bundle.NewOSFS()
	// 200 OK, valid YAML, but no paths -> zero operations.
	client := &http.Client{Transport: fixedTransport{body: "openapi: 3.0.0\ninfo:\n  title: x\n  version: \"1\"\n"}}
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

	v6 := 0
	for i := range idx.Sections {
		s := &idx.Sections[i]
		if !s.IsIndex && strings.HasPrefix(s.Slug, "cloud-rest-api/v6/") {
			v6++
		}
	}
	if v6 == 0 {
		t.Error("expected embedded v6 fallback to populate cloud-rest-api/v6 endpoints")
	}
}
