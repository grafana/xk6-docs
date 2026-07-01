package restdoc

import (
	"net/http"
	"slices"
	"strings"
	"testing"
)

// minimalSpec is a hand-rolled tiny OpenAPI document used to exercise
// the loader without depending on the full embedded v6 spec. It covers
// enough surface (path, path-level parameter, operation-level
// parameter, request body, response, security, tags) for the loader's
// $ref resolution and merge logic to be exercised.
const minimalSpec = `
openapi: 3.0.0
info:
  title: Test API
  version: 1.0.0
servers:
  - url: https://example.test
paths:
  /widgets/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
        description: Widget identifier
    get:
      operationId: widgets_get
      summary: Fetch one widget
      description: Returns a single widget by id.
      tags: [Widgets]
      security:
        - bearerAuth: []
      parameters:
        - name: verbose
          in: query
          required: false
          schema:
            type: boolean
          description: Verbose body
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Widget"
              examples:
                ok:
                  value:
                    id: w1
                    name: Foo
    put:
      operationId: widgets_update
      summary: Update one widget
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/Widget"
            examples:
              rename:
                value:
                  id: w1
                  name: Bar
      responses:
        "200":
          description: OK
components:
  schemas:
    Widget:
      type: object
      properties:
        id:
          type: string
        name:
          type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
`

func TestLoadSpecFromBytes_Minimal(t *testing.T) {
	t.Parallel()

	spec, err := LoadSpecFromBytes([]byte(minimalSpec))
	if err != nil {
		t.Fatalf("LoadSpecFromBytes: %v", err)
	}

	if spec.Title != "Test API" {
		t.Errorf("title: got %q, want %q", spec.Title, "Test API")
	}
	if spec.Version != "1.0.0" {
		t.Errorf("version: got %q, want %q", spec.Version, "1.0.0")
	}
	if spec.BaseURL != "https://example.test" {
		t.Errorf("baseURL: got %q, want %q", spec.BaseURL, "https://example.test")
	}

	if got := len(spec.Operations); got != 2 {
		t.Fatalf("operations: got %d, want 2", got)
	}

	get := spec.ByID("widgets_get")
	if get == nil {
		t.Fatal("widgets_get not found")
	}
	if get.Method != http.MethodGet || get.Path != "/widgets/{id}" {
		t.Errorf("widgets_get: got %s %s, want GET /widgets/{id}", get.Method, get.Path)
	}
	// Path-level + operation-level params merged: expect id (path) + verbose (query).
	if got := len(get.Parameters); got != 2 {
		t.Errorf("widgets_get parameters: got %d, want 2", got)
	}
	if got := len(get.Responses); got != 1 || get.Responses[0].SchemaName != "Widget" {
		t.Errorf("widgets_get response: got %+v, want one response with SchemaName=Widget", get.Responses)
	}
	if !slices.Contains(get.Security, "bearerAuth") {
		t.Errorf("widgets_get security: got %v, want bearerAuth", get.Security)
	}

	put := spec.ByID("widgets_update")
	if put == nil {
		t.Fatal("widgets_update not found")
	}
	if put.RequestBodySchema != "Widget" || !put.RequestBodyRequired {
		t.Errorf("widgets_update request body: got schema=%q required=%v, want Widget/true",
			put.RequestBodySchema, put.RequestBodyRequired)
	}
}

func TestLoadSpecFromBytes_MalformedYAML(t *testing.T) {
	t.Parallel()

	_, err := LoadSpecFromBytes([]byte("openapi: 3.0.0\npaths: [oops"))
	if err == nil {
		t.Fatal("expected error on malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parse openapi.yaml") {
		t.Errorf("error: got %q, want it to mention 'parse openapi.yaml'", err.Error())
	}
}
