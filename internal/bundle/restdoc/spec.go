// Package restdoc parses the Grafana Cloud k6 OpenAPI specifications and
// renders each endpoint as a markdown documentation page. It is a
// build-time-only port of the xk6-rest internal/cli package, used by
// cmd/prepare to generate the cloud-rest-api section of a doc bundle.
package restdoc

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// OrderedMap is a YAML mapping that preserves key insertion order. The
// Python reference loader relies on Python's dict preserving insertion
// order; we replicate that here so output ordering (paths, methods,
// response statuses, schema properties) is byte-identical to the Python
// renderer.
type OrderedMap struct {
	Keys   []string
	Values map[string]any
}

// NewOrderedMap returns an empty OrderedMap.
func NewOrderedMap() *OrderedMap {
	return &OrderedMap{Values: map[string]any{}}
}

// Get returns the value at key (or nil, false).
func (m *OrderedMap) Get(key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m.Values[key]
	return v, ok
}

// GetMap returns the value at key as *OrderedMap (or nil).
func (m *OrderedMap) GetMap(key string) *OrderedMap {
	if m == nil {
		return nil
	}
	if v, ok := m.Values[key].(*OrderedMap); ok {
		return v
	}
	return nil
}

// GetString returns the value at key as string (or "").
func (m *OrderedMap) GetString(key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m.Values[key].(string); ok {
		return s
	}
	return ""
}

// GetBool returns the value at key as bool (or false).
func (m *OrderedMap) GetBool(key string) bool {
	if m == nil {
		return false
	}
	if b, ok := m.Values[key].(bool); ok {
		return b
	}
	return false
}

// GetSlice returns the value at key as []any (or nil).
func (m *OrderedMap) GetSlice(key string) []any {
	if m == nil {
		return nil
	}
	if s, ok := m.Values[key].([]any); ok {
		return s
	}
	return nil
}

// set appends or overwrites the key with the given value.
func (m *OrderedMap) set(key string, value any) {
	if _, exists := m.Values[key]; !exists {
		m.Keys = append(m.Keys, key)
	}
	m.Values[key] = value
}

// nodeToValue converts a yaml.Node into native Go values:
// - mapping -> *OrderedMap
// - sequence -> []any
// - scalar  -> string / bool / int / float / nil (depending on tag)
func nodeToValue(n *yaml.Node) any {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) > 0 {
			return nodeToValue(n.Content[0])
		}
		return nil
	case yaml.MappingNode:
		m := NewOrderedMap()
		for i := 0; i < len(n.Content); i += 2 {
			k := n.Content[i].Value
			v := nodeToValue(n.Content[i+1])
			m.set(k, v)
		}
		return m
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			out = append(out, nodeToValue(c))
		}
		return out
	case yaml.ScalarNode:
		// Decode the scalar using yaml's own type inference.
		var v any
		if err := n.Decode(&v); err != nil {
			return n.Value
		}
		return v
	case yaml.AliasNode:
		if n.Alias != nil {
			return nodeToValue(n.Alias)
		}
		return nil
	}
	return nil
}

// Parameter is a flattened OpenAPI parameter (after $ref resolution).
type Parameter struct {
	Name        string
	In          string // "path" | "query" | "header" | "cookie"
	Required    bool
	SchemaType  string
	Description string
	Default     any
}

// Response is a single response definition for an operation.
type Response struct {
	Status      string
	Description string
	SchemaName  string
	Examples    *OrderedMap // example name -> raw example value
}

// Operation is a single endpoint.
type Operation struct {
	OperationID         string
	Method              string // uppercase
	Path                string
	Summary             string
	Description         string
	Tags                []string
	Security            []string
	Parameters          []Parameter
	RequestBodySchema   string
	RequestBodyRequired bool
	RequestBodyExamples *OrderedMap // example name -> raw example value
	Responses           []Response
}

// Spec is the normalized OpenAPI document.
type Spec struct {
	Title           string
	Version         string
	BaseURL         string
	Schemas         *OrderedMap // schema name -> schema (*OrderedMap)
	SecuritySchemes *OrderedMap // scheme name -> scheme (*OrderedMap)
	Operations      []Operation
	doc             *OrderedMap // full parsed document, for ref resolution
}

// ByID returns the operation matching operationID, or nil if not found.
func (s *Spec) ByID(operationID string) *Operation {
	for i := range s.Operations {
		if s.Operations[i].OperationID == operationID {
			return &s.Operations[i]
		}
	}
	return nil
}

// PrefixOperationIDs prepends prefix + "/" to every operation's
// OperationID. Used to namespace one spec's operations when merging
// the index with another spec (e.g. "v5" + "/" + "test_run_series_list"
// -> "v5/test_run_series_list"). Subsequent lookups via ByID must use
// the namespaced form.
//
// No-op if prefix is empty.
func (s *Spec) PrefixOperationIDs(prefix string) {
	if prefix == "" {
		return
	}
	for i := range s.Operations {
		s.Operations[i].OperationID = prefix + "/" + s.Operations[i].OperationID
	}
}

// httpVerbs is the set of OpenAPI path-item method keys we recognise.
//
//nolint:gochecknoglobals // immutable lookup set; treated as a constant
var httpVerbs = map[string]bool{
	"get": true, "put": true, "post": true, "delete": true,
	"options": true, "head": true, "patch": true, "trace": true,
}

// LoadSpecFromBytes parses OpenAPI YAML bytes and returns the
// normalised Spec. Returns an error if the YAML is malformed.
//
// Lets callers load a spec from a source other than the build-time
// embed (e.g. a freshly-fetched cache file, or a second spec like the
// hand-authored v5 metrics doc).
func LoadSpecFromBytes(specBytes []byte) (*Spec, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(specBytes, &root); err != nil {
		return nil, fmt.Errorf("parse openapi.yaml: %w", err)
	}
	docVal := nodeToValue(&root)
	doc, ok := docVal.(*OrderedMap)
	if !ok {
		return nil, fmt.Errorf("openapi.yaml top-level is not a mapping")
	}

	info := doc.GetMap("info")
	servers := doc.GetSlice("servers")
	components := doc.GetMap("components")
	schemas := components.GetMap("schemas")
	securitySchemes := components.GetMap("securitySchemes")

	baseURL := "https://api.k6.io"
	if len(servers) > 0 {
		if s0, ok := servers[0].(*OrderedMap); ok {
			baseURL = s0.GetString("url")
		}
	}

	spec := &Spec{
		Title:           info.GetString("title"),
		Version:         info.GetString("version"),
		BaseURL:         baseURL,
		Schemas:         schemas,
		SecuritySchemes: securitySchemes,
		doc:             doc,
	}

	paths := doc.GetMap("paths")
	if paths != nil {
		for _, p := range paths.Keys {
			methods := paths.GetMap(p)
			if methods == nil {
				continue
			}
			// Path-level parameters are shared across every operation under the path.
			pathLevelParams := methods.GetSlice("parameters")
			for _, m := range methods.Keys {
				if !httpVerbs[m] {
					continue
				}
				raw := methods.GetMap(m)
				if raw == nil {
					continue
				}
				op := extractOperation(doc, p, m, raw, pathLevelParams)
				spec.Operations = append(spec.Operations, op)
			}
		}
	}

	return spec, nil
}

// resolveRef resolves a local JSON-pointer ref like "#/components/schemas/X"
// against doc. Returns nil if the ref can't be resolved.
func resolveRef(doc *OrderedMap, ref string) any {
	if !strings.HasPrefix(ref, "#/") {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var node any = doc
	for _, p := range parts {
		m, ok := node.(*OrderedMap)
		if !ok {
			return nil
		}
		v, exists := m.Get(p)
		if !exists {
			return nil
		}
		node = v
	}
	return node
}

func resolveParameter(doc *OrderedMap, raw *OrderedMap) Parameter {
	if ref := raw.GetString("$ref"); ref != "" {
		if resolved, ok := resolveRef(doc, ref).(*OrderedMap); ok {
			raw = resolved
		}
	}
	schema := raw.GetMap("schema")
	schemaType := "string"
	var defaultVal any
	if schema != nil {
		if t := schema.GetString("type"); t != "" {
			schemaType = t
		}
		if v, ok := schema.Get("default"); ok {
			defaultVal = v
		}
	}
	return Parameter{
		Name:        raw.GetString("name"),
		In:          raw.GetString("in"),
		Required:    raw.GetBool("required"),
		SchemaType:  schemaType,
		Description: strings.TrimSpace(raw.GetString("description")),
		Default:     defaultVal,
	}
}

func resolveResponse(doc *OrderedMap, status string, raw *OrderedMap) Response {
	if ref := raw.GetString("$ref"); ref != "" {
		if resolved, ok := resolveRef(doc, ref).(*OrderedMap); ok {
			raw = resolved
		}
	}
	resp := Response{
		Status:      status,
		Description: strings.TrimSpace(raw.GetString("description")),
		Examples:    NewOrderedMap(),
	}
	media := firstMedia(raw.GetMap("content"))
	if media == nil {
		return resp
	}
	if schema := media.GetMap("schema"); schema != nil {
		if ref := schema.GetString("$ref"); ref != "" {
			parts := strings.Split(ref, "/")
			resp.SchemaName = parts[len(parts)-1]
		}
	}
	collectExamples(media, resp.Examples)
	return resp
}

func extractOperation(doc *OrderedMap, path, method string, raw *OrderedMap, pathLevelParams []any) Operation {
	op := Operation{
		OperationID:         raw.GetString("operationId"),
		Method:              strings.ToUpper(method),
		Path:                path,
		Summary:             strings.TrimSpace(raw.GetString("summary")),
		Description:         strings.TrimSpace(raw.GetString("description")),
		Tags:                extractTags(raw),
		Parameters:          extractParameters(doc, raw, pathLevelParams),
		RequestBodyExamples: NewOrderedMap(),
	}
	extractRequestBody(raw, &op)
	op.Responses = extractResponses(doc, raw)
	op.Security = extractSecurity(raw)
	return op
}

// extractTags flattens the operation's tags array to []string, dropping
// any non-string entries.
func extractTags(raw *OrderedMap) []string {
	tagsRaw := raw.GetSlice("tags")
	var tagStrs []string
	for _, t := range tagsRaw {
		if s, ok := t.(string); ok {
			tagStrs = append(tagStrs, s)
		}
	}
	return tagStrs
}

// extractParameters merges path-level parameters first, then operation-level,
// matching the Python loader's merge so output stays consistent. Each
// entry is resolved through $ref before being returned.
func extractParameters(doc *OrderedMap, raw *OrderedMap, pathLevelParams []any) []Parameter {
	merged := append([]any{}, pathLevelParams...)
	merged = append(merged, raw.GetSlice("parameters")...)
	var params []Parameter
	for _, p := range merged {
		if pm, ok := p.(*OrderedMap); ok {
			params = append(params, resolveParameter(doc, pm))
		}
	}
	return params
}

// extractRequestBody fills in op.RequestBodyRequired, op.RequestBodySchema,
// and op.RequestBodyExamples from raw["requestBody"], if present.
func extractRequestBody(raw *OrderedMap, op *Operation) {
	rb := raw.GetMap("requestBody")
	if rb == nil {
		return
	}
	op.RequestBodyRequired = rb.GetBool("required")
	media := firstMedia(rb.GetMap("content"))
	if media == nil {
		return
	}
	if schema := media.GetMap("schema"); schema != nil {
		if ref := schema.GetString("$ref"); ref != "" {
			parts := strings.Split(ref, "/")
			op.RequestBodySchema = parts[len(parts)-1]
		}
	}
	collectExamples(media, op.RequestBodyExamples)
}

// extractResponses walks raw["responses"] in declaration order and
// resolves each via resolveResponse.
func extractResponses(doc *OrderedMap, raw *OrderedMap) []Response {
	responses := raw.GetMap("responses")
	if responses == nil {
		return nil
	}
	var out []Response
	for _, status := range responses.Keys {
		respRaw := responses.GetMap(status)
		if respRaw == nil {
			continue
		}
		out = append(out, resolveResponse(doc, status, respRaw))
	}
	return out
}

// extractSecurity flattens raw["security"] (a list of single-key maps)
// to a flat []string of scheme names.
func extractSecurity(raw *OrderedMap) []string {
	securityList := raw.GetSlice("security")
	if securityList == nil {
		return nil
	}
	var out []string
	for _, entry := range securityList {
		if e, ok := entry.(*OrderedMap); ok {
			out = append(out, e.Keys...)
		}
	}
	return out
}

// firstMedia returns the first media-type entry of an OpenAPI `content`
// map (matching the Python loader, which only inspects the first
// media type), or nil if content is empty.
func firstMedia(content *OrderedMap) *OrderedMap {
	if content == nil || len(content.Keys) == 0 {
		return nil
	}
	return content.GetMap(content.Keys[0])
}

// collectExamples copies media["examples"][name]["value"] entries into
// the destination OrderedMap in declaration order.
func collectExamples(media *OrderedMap, dst *OrderedMap) {
	examples := media.GetMap("examples")
	if examples == nil {
		return
	}
	for _, exName := range examples.Keys {
		ex := examples.GetMap(exName)
		if ex == nil {
			continue
		}
		v, _ := ex.Get("value")
		dst.set(exName, v)
	}
}

// Doc returns the underlying parsed OpenAPI document. Used by the render
// layer to resolve schema $refs lazily.
func (s *Spec) Doc() *OrderedMap { return s.doc }
