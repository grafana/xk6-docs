package restdoc

import (
	_ "embed"
	"fmt"
	"log"
	"path"
	"path/filepath"
	"strings"

	"github.com/grafana/xk6-docs/docs"
)

//go:embed openapi.yaml
var embeddedV6Spec []byte

//go:embed openapi-v5.yaml
var embeddedV5Spec []byte

// sectionSlug is the top-level docs slug under which the generated REST
// API reference lives.
const sectionSlug = "cloud-rest-api"

// apiVersion pairs an API version's display title with its spec bytes.
type apiVersion struct {
	name  string
	title string
	spec  []byte
}

// Generate parses the Grafana Cloud k6 v5 and v6 OpenAPI specs and writes
// one markdown page per endpoint under markdownDir/cloud-rest-api/<version>/,
// plus _index.md pages for the section and each version. It returns the
// docs.Section entries (index pages and endpoint leaves) to be appended to a
// bundle's section list before parent/child relationships are populated.
//
// v6Spec, when non-empty, overrides the embedded v6 spec (cmd/prepare passes
// a freshly fetched copy). The v5 spec is always the embedded, hand-authored
// reference, as the v5 API publishes no OpenAPI document.
func Generate(afs docs.FS, markdownDir string, v6Spec []byte) ([]docs.Section, error) {
	v6 := v6Spec
	if len(v6) == 0 {
		v6 = embeddedV6Spec
	}
	versions := []apiVersion{
		{name: "v5", title: "v5 — Metrics API", spec: embeddedV5Spec},
		{name: "v6", title: "v6 — General-purpose API", spec: v6},
	}

	sections := []docs.Section{newIndexSection(sectionSlug, "Grafana Cloud k6 REST API", 0)}
	if err := writePage(afs, markdownDir, path.Join(sectionSlug, "_index.md"),
		indexBody("Grafana Cloud k6 REST API",
			"Reference for the Grafana Cloud k6 REST API, generated from its OpenAPI specifications.")); err != nil {
		return nil, err
	}

	for vi := range versions {
		v := versions[vi]
		verSections, err := generateVersion(afs, markdownDir, v, vi)
		if err != nil {
			return nil, err
		}
		sections = append(sections, verSections...)
	}

	return sections, nil
}

// generateVersion writes the _index and endpoint pages for one API version
// and returns their sections. weight orders the version under the top-level
// section (v5 before v6).
func generateVersion(afs docs.FS, markdownDir string, v apiVersion, weight int) ([]docs.Section, error) {
	spec, err := LoadSpecFromBytes(v.spec)
	if err != nil {
		return nil, fmt.Errorf("parse %s spec: %w", v.name, err)
	}
	spec.PrefixOperationIDs(v.name)

	verSlug := path.Join(sectionSlug, v.name)
	sections := []docs.Section{newIndexSection(verSlug, v.title, weight)}
	if err := writePage(afs, markdownDir, path.Join(verSlug, "_index.md"),
		indexBody(v.title, fmt.Sprintf("Grafana Cloud k6 %s REST API endpoints.", v.name))); err != nil {
		return nil, err
	}

	for i := range spec.Operations {
		op := &spec.Operations[i]
		bareID := strings.TrimPrefix(op.OperationID, v.name+"/")
		if !safeOperationID(bareID) {
			log.Printf("warning: skipping %s operation with unsafe operationId %q", v.name, op.OperationID)
			continue
		}
		slug := path.Join(verSlug, bareID)
		relPath := slug + ".md"
		if err := writePage(afs, markdownDir, relPath, RenderEndpoint(spec, op)); err != nil {
			return nil, err
		}
		sections = append(sections, docs.Section{
			Slug:        slug,
			RelPath:     relPath,
			Title:       bareID,
			Description: op.Summary,
			Weight:      i,
			Category:    sectionSlug,
		})
	}
	return sections, nil
}

func newIndexSection(slug, title string, weight int) docs.Section {
	return docs.Section{
		Slug:     slug,
		RelPath:  path.Join(slug, "_index.md"),
		Title:    title,
		Weight:   weight,
		Category: sectionSlug,
		IsIndex:  true,
	}
}

func indexBody(title, intro string) string {
	return fmt.Sprintf("# %s\n\n%s\n", title, intro)
}

// safeOperationID reports whether id is a single, non-escaping path segment.
// An empty id, a path separator, or a "."/".." segment could place the
// generated page outside the section directory (path traversal) or collide
// with an index page, so such operations are skipped. operationIds in the wild
// are simple identifiers (e.g. "load_tests_list"); this rejects only malformed
// or hostile input, which matters now that the v6 spec is fetched at build
// time rather than embedded.
func safeOperationID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, `/\`)
}

func writePage(afs docs.FS, markdownDir, relPath, body string) error {
	outPath := filepath.Join(markdownDir, filepath.FromSlash(relPath))
	// Defense in depth: never write outside the markdown directory, whatever
	// relPath contains.
	if rel, err := filepath.Rel(markdownDir, outPath); err != nil ||
		rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("refusing to write outside markdown dir: %s", relPath)
	}
	if err := afs.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(outPath), err)
	}
	if err := afs.WriteFile(outPath, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}
