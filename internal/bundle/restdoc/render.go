package restdoc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// RenderEndpoint produces the markdown-rendered detail for one
// endpoint. Port of render_endpoint() in gen_skill.py. The output must
// be byte-identical to the corresponding skill endpoint file (and to
// the Python mock's `show` output) so the C3 fairness guarantee holds.
func RenderEndpoint(spec *Spec, op *Operation) string {
	lines := make([]string, 0, 64)
	lines = append(lines, renderEndpointHeader(op)...)
	lines = append(lines, "## Auth", "", authSummary(spec, op), "")
	lines = append(lines, renderEndpointTags(op)...)
	lines = append(lines, renderEndpointParameters(op)...)
	lines = append(lines, renderEndpointRequestBody(spec, op)...)
	lines = append(lines, renderEndpointResponses(spec, op)...)
	lines = append(lines, renderEndpointInvocation(spec, op)...)
	return strings.Join(lines, "\n")
}

// renderEndpointHeader emits the title, signature, summary, and description.
func renderEndpointHeader(op *Operation) []string {
	lines := []string{
		fmt.Sprintf("# `%s`", op.OperationID),
		"",
		fmt.Sprintf("`%s %s`", op.Method, op.Path),
		"",
	}
	if op.Summary != "" {
		lines = append(lines, op.Summary, "")
	}
	if op.Description != "" && op.Description != op.Summary {
		lines = append(lines, op.Description, "")
	}
	return lines
}

// renderEndpointTags emits the Tags section, or nothing if no tags.
func renderEndpointTags(op *Operation) []string {
	if len(op.Tags) == 0 {
		return nil
	}
	quoted := make([]string, len(op.Tags))
	for i, t := range op.Tags {
		quoted[i] = "`" + t + "`"
	}
	return []string{"## Tags", "", strings.Join(quoted, ", "), ""}
}

// renderEndpointParameters emits the Parameters section, or "None." if empty.
func renderEndpointParameters(op *Operation) []string {
	pathParams := renderParamSection(op, "path", "Path")
	queryParams := renderParamSection(op, "query", "Query")
	headerParams := renderParamSection(op, "header", "Header")
	if len(pathParams)+len(queryParams)+len(headerParams) == 0 {
		return []string{"## Parameters", "", "None.", ""}
	}
	out := make([]string, 0, 2+len(pathParams)+len(queryParams)+len(headerParams))
	out = append(out, "## Parameters", "")
	out = append(out, pathParams...)
	out = append(out, queryParams...)
	out = append(out, headerParams...)
	return out
}

// renderEndpointRequestBody emits the Request body section, including the
// schema fence and an optional example.
func renderEndpointRequestBody(spec *Spec, op *Operation) []string {
	lines := []string{"## Request body", ""}
	if op.RequestBodySchema == "" {
		return append(lines, "None.", "")
	}
	requiredWord := "optional"
	if op.RequestBodyRequired {
		requiredWord = "required"
	}
	lines = append(lines,
		fmt.Sprintf("Content type: `application/json` (%s). Schema `%s`:",
			requiredWord, op.RequestBodySchema),
		"",
		"```",
		formatSchemaBlock(spec, op.RequestBodySchema),
		"```",
		"",
	)
	if op.RequestBodyExamples != nil && len(op.RequestBodyExamples.Keys) > 0 {
		exName := sortedFirst(op.RequestBodyExamples.Keys)
		exVal, _ := op.RequestBodyExamples.Get(exName)
		lines = append(lines,
			"### Example body",
			"",
			fmt.Sprintf("`%s`:", exName),
			"",
			"```json",
			toJSON(exVal),
			"```",
			"",
		)
	}
	return lines
}

// renderEndpointResponses emits the Responses section: success responses
// individually with optional schema + example, then a single Errors list.
func renderEndpointResponses(spec *Spec, op *Operation) []string {
	lines := []string{"## Responses", ""}
	var success, errors []Response
	for _, r := range op.Responses {
		if strings.HasPrefix(r.Status, "2") {
			success = append(success, r)
		} else {
			errors = append(errors, r)
		}
	}
	for _, resp := range success {
		lines = append(lines, renderSuccessResponse(spec, resp)...)
	}
	if len(errors) > 0 {
		lines = append(lines, "### Errors", "")
		for _, resp := range errors {
			schemaNote := ""
			if resp.SchemaName != "" {
				schemaNote = fmt.Sprintf(" Body: `%s`.", resp.SchemaName)
			}
			lines = append(lines, fmt.Sprintf("- `%s` — %s%s",
				resp.Status, resp.Description, schemaNote))
		}
		lines = append(lines, "")
	}
	return lines
}

// renderSuccessResponse emits one 2xx response block.
func renderSuccessResponse(spec *Spec, resp Response) []string {
	lines := []string{
		fmt.Sprintf("### `%s` — %s", resp.Status, resp.Description),
		"",
	}
	if resp.SchemaName != "" {
		lines = append(lines,
			fmt.Sprintf("Schema `%s`:", resp.SchemaName),
			"",
			"```",
			formatSchemaBlock(spec, resp.SchemaName),
			"```",
			"",
		)
	}
	if resp.Examples != nil && len(resp.Examples.Keys) > 0 {
		exName := sortedFirst(resp.Examples.Keys)
		exVal, _ := resp.Examples.Get(exName)
		lines = append(lines,
			fmt.Sprintf("Example response (`%s`):", exName),
			"",
			"```json",
			toJSON(exVal),
			"```",
			"",
		)
	}
	return lines
}

// renderEndpointInvocation emits the curl invocation block.
func renderEndpointInvocation(spec *Spec, op *Operation) []string {
	lines := []string{
		"## Invocation",
		"",
		"Base URL: `" + spec.BaseURL + "`.",
		"",
		"```bash",
		fmt.Sprintf(`curl -X %s "$BASE_URL%s" \`, op.Method, op.Path),
		`  -H "Authorization: Bearer $GCK6_TOKEN" \`,
	}
	hasStackID := false
	hasStackURL := false
	for _, p := range op.Parameters {
		if p.Name == "X-Stack-Id" {
			hasStackID = true
		}
		if p.Name == "X-Stack-Url" {
			hasStackURL = true
		}
	}
	if hasStackID {
		lines = append(lines, `  -H "X-Stack-Id: $STACK_ID" \`)
	}
	if hasStackURL {
		lines = append(lines, `  -H "X-Stack-Url: $STACK_URL" \`)
	}
	if op.RequestBodySchema != "" {
		lines = append(lines, `  -H "Content-Type: application/json" \`, "  -d @body.json")
	} else {
		// Strip trailing " \" on the last header line.
		last := len(lines) - 1
		lines[last] = strings.TrimSuffix(lines[last], ` \`)
	}
	lines = append(lines, "```", "")
	return lines
}

// --- parameter rendering ----------------------------------------------------

// formatParamLine mirrors _format_param_line in gen_skill.py. The
// X-Stack-Id header gets a compact one-liner that points to SKILL.md
// instead of repeating the multi-line description per endpoint.
func formatParamLine(p Parameter) string {
	if p.Name == "X-Stack-Id" && p.In == "header" {
		return "- `X-Stack-Id` `integer` required — Numeric ID of the Grafana stack."
	}
	bits := []string{"`" + p.Name + "`", "`" + p.SchemaType + "`"}
	if p.Required {
		bits = append(bits, "required")
	}
	line := "- " + strings.Join(bits, " ")
	if p.Default != nil {
		line += fmt.Sprintf(" (default `%s`)", pyDefault(p.Default))
	}
	if p.Description != "" {
		line += " — " + p.Description
	}
	return line
}

func renderParamSection(op *Operation, location, heading string) []string {
	var params []Parameter
	for _, p := range op.Parameters {
		if p.In == location {
			params = append(params, p)
		}
	}
	if len(params) == 0 {
		return nil
	}
	out := []string{"### " + heading + " parameters", ""}
	for _, p := range params {
		out = append(out, formatParamLine(p))
	}
	out = append(out, "")
	return out
}

// pyDefault renders a default value the way Python's f-string does:
// strings unquoted, bools as True/False, None as None, numbers as-is.
func pyDefault(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case string:
		return x
	default:
		return fmt.Sprintf("%v", x)
	}
}

// --- auth -------------------------------------------------------------------

// authSummary mirrors auth_summary() in openapi_loader.py.
func authSummary(spec *Spec, op *Operation) string {
	if len(op.Security) == 0 {
		return "No authentication required."
	}
	var parts []string
	for _, name := range op.Security {
		scheme := spec.SecuritySchemes.GetMap(name)
		desc := strings.TrimSpace(scheme.GetString("description"))
		if desc != "" {
			parts = append(parts, fmt.Sprintf("`%s` (%s)", name, desc))
		} else {
			parts = append(parts, fmt.Sprintf("`%s`", name))
		}
	}
	return "Authentication: " + strings.Join(parts, "; ") + " via `Authorization: Bearer <token>`."
}

// --- schema rendering -------------------------------------------------------

// formatSchemaBlock renders a top-level schema as a fenced markdown
// block. Mirrors format_schema_block() in openapi_loader.py.
func formatSchemaBlock(spec *Spec, schemaName string) string {
	if spec.Schemas == nil {
		return "<unresolved schema: " + schemaName + ">"
	}
	schema := spec.Schemas.GetMap(schemaName)
	if schema == nil {
		return "<unresolved schema: " + schemaName + ">"
	}
	return renderSchema(spec, schema, 0, nil)
}

// renderSchema is the recursive schema renderer. Port of _render_schema()
// in openapi_loader.py. Produces a JSON-like indented tree of type
// information, marking required props and inlining descriptions.
func renderSchema(spec *Spec, schema *OrderedMap, depth int, seen map[string]bool) string {
	if seen == nil {
		seen = map[string]bool{}
	}
	if schema == nil {
		return "any"
	}

	if ref := schema.GetString("$ref"); ref != "" {
		return renderSchemaRef(spec, ref, depth, seen)
	}

	tRaw, _ := schema.Get("type")
	props := schema.GetMap("properties")
	if isObjectSchema(tRaw, props) {
		return renderObjectProperties(spec, schema, props, depth, seen)
	}

	if t, ok := tRaw.(string); ok && t == "array" {
		item := schema.GetMap("items")
		return "array<" + renderSchema(spec, item, depth, seen) + ">"
	}

	if out, ok := renderSchemaEnum(schema, tRaw); ok {
		return out
	}

	if fmtStr := schema.GetString("format"); fmtStr != "" {
		return pyStr(tRaw) + "<" + fmtStr + ">"
	}

	if tRaw == nil {
		return "any"
	}
	if s, ok := tRaw.(string); ok && s == "" {
		return "any"
	}
	return pyStr(tRaw)
}

// isObjectSchema reports whether this schema node should be rendered
// as a `{ ... }` block. An explicit `type: object` or the mere
// presence of `properties` both qualify (matching the Python loader).
func isObjectSchema(tRaw any, props *OrderedMap) bool {
	if t, ok := tRaw.(string); ok && t == "object" {
		return true
	}
	return props != nil
}

// renderSchemaRef resolves a local $ref and recurses, guarding against
// cycles by tracking visited schema names per branch.
func renderSchemaRef(spec *Spec, ref string, depth int, seen map[string]bool) string {
	parts := strings.Split(ref, "/")
	name := parts[len(parts)-1]
	if seen[name] {
		return "<recursive: " + name + ">"
	}
	if spec.Schemas != nil {
		if sub := spec.Schemas.GetMap(name); sub != nil {
			next := map[string]bool{name: true}
			for k := range seen {
				next[k] = true
			}
			return renderSchema(spec, sub, depth, next)
		}
	}
	return "<unresolved ref: " + name + ">"
}

// renderObjectProperties emits the `{ ... }` block for an object-typed
// schema, including the required marker and description suffix on
// each property line.
func renderObjectProperties(spec *Spec, schema, props *OrderedMap, depth int, seen map[string]bool) string {
	requiredSet := map[string]bool{}
	for _, r := range schema.GetSlice("required") {
		if s, ok := r.(string); ok {
			requiredSet[s] = true
		}
	}
	lines := []string{"{"}
	if props != nil {
		indent := strings.Repeat("  ", depth+1)
		for _, propName := range props.Keys {
			lines = append(lines, renderObjectProperty(spec, props, propName, requiredSet, indent, depth, seen))
		}
	}
	lines = append(lines, strings.Repeat("  ", depth)+"}")
	return strings.Join(lines, "\n")
}

// renderObjectProperty formats a single `<name>(required)?: <type><desc>,` line.
func renderObjectProperty(
	spec *Spec,
	props *OrderedMap,
	propName string,
	requiredSet map[string]bool,
	indent string,
	depth int,
	seen map[string]bool,
) string {
	prop := props.GetMap(propName)
	marker := ""
	if requiredSet[propName] {
		marker = " (required)"
	}
	desc := ""
	if prop != nil {
		desc = strings.ReplaceAll(strings.TrimSpace(prop.GetString("description")), "\n", " ")
	}
	descSuffix := ""
	if desc != "" {
		descSuffix = " -- " + desc
	}
	sub := renderSchema(spec, prop, depth+1, seen)
	return fmt.Sprintf("%s%s%s: %s%s,", indent, propName, marker, sub, descSuffix)
}

// renderSchemaEnum returns the `<type> (a | b | c)` form for an enum
// schema. The second result is false if there is no enum or the enum
// value isn't a []any.
func renderSchemaEnum(schema *OrderedMap, tRaw any) (string, bool) {
	enum, ok := schema.Get("enum")
	if !ok {
		return "", false
	}
	enumSlice, ok := enum.([]any)
	if !ok {
		return "", false
	}
	parts := make([]string, len(enumSlice))
	for i, e := range enumSlice {
		parts[i] = pyRepr(e)
	}
	typeLabel := "enum"
	if t, ok := tRaw.(string); ok && t != "" {
		typeLabel = t
	}
	return fmt.Sprintf("%s (%s)", typeLabel, strings.Join(parts, " | ")), true
}

// pyStr renders a value the way Python's str() would when interpolated
// into an f-string. For string scalars this is the raw value; for a
// list of strings (the common multi-type pattern in OpenAPI 3.1, e.g.
// type: [string, null]) it produces Python's list-repr: ['string', 'null'].
func pyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = pyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// pyRepr replicates Python's repr() for the scalar types we encounter in
// OpenAPI enums: strings get single-quoted (with embedded quote escaping
// to match Python's behaviour for the simple case), numbers/booleans/None
// are rendered as Python literals.
func pyRepr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		// Python repr prefers single quotes when there's no single quote in
		// the string; uses double quotes only if there's a single quote and
		// no double quote. Our OpenAPI enum strings never contain quotes,
		// so single quotes are always correct here.
		if strings.ContainsRune(x, '\'') && !strings.ContainsRune(x, '"') {
			return `"` + x + `"`
		}
		return "'" + strings.ReplaceAll(x, `'`, `\'`) + "'"
	case bool:
		if x {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		// Python repr of an integer-valued float like 1.0 is "1.0".
		if x == math.Trunc(x) && !math.IsInf(x, 0) {
			return strconv.FormatFloat(x, 'f', 1, 64)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// --- JSON serialisation -----------------------------------------------------

// toJSON reproduces Python's json.dumps(value, indent=2, default=str)
// on the parsed YAML value tree. Order of object keys is preserved
// from the YAML source.
func toJSON(v any) string {
	var buf bytes.Buffer
	writeJSON(&buf, v, 0)
	return buf.String()
}

func writeJSON(buf *bytes.Buffer, v any, depth int) {
	indent := strings.Repeat("  ", depth)
	innerIndent := strings.Repeat("  ", depth+1)
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		writeJSONString(buf, x)
	case int:
		buf.WriteString(strconv.Itoa(x))
	case int64:
		buf.WriteString(strconv.FormatInt(x, 10))
	case uint64:
		buf.WriteString(strconv.FormatUint(x, 10))
	case float64:
		// Python json.dumps emits integer-valued floats with no fractional
		// part as "1" (since 1.0 == 1 in float, but if the original YAML
		// scalar parsed as int it'd already be int). For true floats this
		// matches Go's float formatting.
		if x == math.Trunc(x) && !math.IsInf(x, 0) && math.Abs(x) < 1e16 {
			buf.WriteString(strconv.FormatFloat(x, 'f', 1, 64))
		} else {
			buf.WriteString(strconv.FormatFloat(x, 'g', -1, 64))
		}
	case *OrderedMap:
		if x == nil || len(x.Keys) == 0 {
			buf.WriteString("{}")
			return
		}
		buf.WriteString("{\n")
		for i, k := range x.Keys {
			buf.WriteString(innerIndent)
			writeJSONString(buf, k)
			buf.WriteString(": ")
			writeJSON(buf, x.Values[k], depth+1)
			if i < len(x.Keys)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString(indent)
		buf.WriteString("}")
	case []any:
		if len(x) == 0 {
			buf.WriteString("[]")
			return
		}
		buf.WriteString("[\n")
		for i, e := range x {
			buf.WriteString(innerIndent)
			writeJSON(buf, e, depth+1)
			if i < len(x)-1 {
				buf.WriteString(",")
			}
			buf.WriteString("\n")
		}
		buf.WriteString(indent)
		buf.WriteString("]")
	default:
		// Fallback: stringify via Go's json package; if that fails, use %v.
		b, err := json.Marshal(x)
		if err != nil {
			writeJSONString(buf, fmt.Sprintf("%v", x))
			return
		}
		buf.Write(b)
	}
}

// writeJSONString writes a JSON-escaped string matching Python's
// json.dumps default behaviour. Go's encoding/json escapes `&`, `<`,
// and `>` to their `\uXXXX` form for HTML-safety; Python does not. We
// use a SetEscapeHTML(false) encoder to match.
func writeJSONString(buf *bytes.Buffer, s string) {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	//nolint:errchkjson // safe: encoding a Go string cannot fail
	_ = enc.Encode(s)
	// Encoder.Encode appends a trailing newline; strip it.
	out := tmp.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	buf.Write(out)
}

// sortedFirst returns the lexicographically smallest string in keys.
// Mirrors `sorted(keys)[0]` in Python.
func sortedFirst(keys []string) string {
	if len(keys) == 0 {
		return ""
	}
	best := keys[0]
	for _, k := range keys[1:] {
		if k < best {
			best = k
		}
	}
	return best
}
