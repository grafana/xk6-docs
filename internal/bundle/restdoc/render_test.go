package restdoc

import (
	"strings"
	"testing"
)

// renderSpec exercises the two code paths whose wording was changed
// during the port away from xk6-rest: the X-Stack-Id header parameter
// and an error response that carries a schema. Both previously pointed
// the reader at a "SKILL.md" file that does not exist in the docs
// context.
const renderSpec = `
openapi: 3.0.0
info:
  title: Render Test API
  version: 1.0.0
servers:
  - url: https://api.k6.io
paths:
  /things/{id}:
    get:
      operationId: things_get
      summary: Get a thing
      security:
        - bearerAuth: []
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
        - name: X-Stack-Id
          in: header
          required: true
          schema:
            type: integer
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Thing"
        "404":
          description: Not found
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
components:
  schemas:
    Thing:
      type: object
      properties:
        id:
          type: string
    Error:
      type: object
      properties:
        message:
          type: string
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
`

func TestRenderEndpoint(t *testing.T) {
	t.Parallel()

	spec, err := LoadSpecFromBytes([]byte(renderSpec))
	if err != nil {
		t.Fatalf("LoadSpecFromBytes: %v", err)
	}
	op := spec.ByID("things_get")
	if op == nil {
		t.Fatal("things_get not found")
	}

	out := RenderEndpoint(spec, op)

	for _, want := range []string{
		"# `things_get`",
		"`GET /things/{id}`",
		"## Auth",
		"## Parameters",
		"## Invocation",
		"curl -X GET",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n---\n%s", want, out)
		}
	}
}

// TestRenderEndpoint_NoSkillReferences locks the two wording changes
// made during the port: references to "SKILL.md" must be gone, replaced
// by self-contained phrasing.
func TestRenderEndpoint_NoSkillReferences(t *testing.T) {
	t.Parallel()

	spec, err := LoadSpecFromBytes([]byte(renderSpec))
	if err != nil {
		t.Fatalf("LoadSpecFromBytes: %v", err)
	}
	out := RenderEndpoint(spec, spec.ByID("things_get"))

	if strings.Contains(out, "SKILL.md") {
		t.Errorf("rendered output still references SKILL.md\n---\n%s", out)
	}
	if !strings.Contains(out, "Numeric ID of the Grafana stack.") {
		t.Errorf("X-Stack-Id parameter not reworded as expected\n---\n%s", out)
	}
	if !strings.Contains(out, "Body: `Error`.") {
		t.Errorf("error response schema note not reworded as expected\n---\n%s", out)
	}
}
