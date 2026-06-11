package scan

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// OperationSchema is the fully resolved request/response contract of one
// OpenAPI operation.
type OperationSchema struct {
	Service     string         `json:"service"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	OperationID string         `json:"operation_id,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Parameters  []any          `json:"parameters,omitempty"`
	RequestBody any            `json:"request_body,omitempty"`
	Responses   map[string]any `json:"responses,omitempty"`
}

// maxRefDepth caps $ref resolution to keep recursive schemas bounded.
const maxRefDepth = 12

// LoadOperationSchema parses a spec file and returns one operation with all
// local $refs resolved. The operation is selected by method+path, or by
// operationId when path is empty.
func LoadOperationSchema(specFile, method, path, operationID string) (*OperationSchema, error) {
	data, err := os.ReadFile(specFile)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	paths, _ := doc["paths"].(map[string]any)
	if paths == nil {
		return nil, fmt.Errorf("spec %s has no paths", specFile)
	}

	method = strings.ToLower(method)
	r := &refResolver{doc: doc}

	for p, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		for m, rawOp := range item {
			op, ok := rawOp.(map[string]any)
			if !ok {
				continue
			}
			opID, _ := op["operationId"].(string)
			pathMatch := path != "" && p == path && m == method
			idMatch := path == "" && operationID != "" && strings.EqualFold(opID, operationID)
			if !pathMatch && !idMatch {
				continue
			}

			out := &OperationSchema{
				Method:      strings.ToUpper(m),
				Path:        p,
				OperationID: opID,
			}
			out.Summary, _ = op["summary"].(string)
			if params, ok := op["parameters"].([]any); ok {
				out.Parameters = r.resolve(params, 0).([]any)
			}
			if body, ok := op["requestBody"]; ok {
				out.RequestBody = r.resolve(body, 0)
			}
			if resp, ok := op["responses"].(map[string]any); ok {
				resolved := map[string]any{}
				for code, v := range resp {
					resolved[code] = r.resolve(v, 0)
				}
				out.Responses = resolved
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("operation not found in %s (method=%s path=%s operationId=%s)", specFile, strings.ToUpper(method), path, operationID)
}

// refResolver inlines local "#/..." references, cycle-safe and depth-capped.
type refResolver struct {
	doc  map[string]any
	seen map[string]bool // refs on the current resolution path
}

func (r *refResolver) resolve(node any, depth int) any {
	if depth > maxRefDepth {
		return map[string]any{"$note": "max resolution depth reached"}
	}
	switch v := node.(type) {
	case map[string]any:
		if ref, ok := v["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
			if r.seen[ref] {
				return map[string]any{"$recursive_ref": ref[strings.LastIndex(ref, "/")+1:]}
			}
			target := r.lookup(ref)
			if target == nil {
				return map[string]any{"$unresolved": ref}
			}
			if r.seen == nil {
				r.seen = map[string]bool{}
			}
			r.seen[ref] = true
			resolved := r.resolve(target, depth+1)
			delete(r.seen, ref)
			// Keep the schema name as provenance.
			if m, ok := resolved.(map[string]any); ok {
				out := make(map[string]any, len(m)+1)
				for k, val := range m {
					out[k] = val
				}
				out["$schema_name"] = ref[strings.LastIndex(ref, "/")+1:]
				return out
			}
			return resolved
		}
		out := make(map[string]any, len(v))
		for k, val := range v {
			out[k] = r.resolve(val, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, val := range v {
			out[i] = r.resolve(val, depth+1)
		}
		return out
	default:
		return node
	}
}

func (r *refResolver) lookup(ref string) any {
	parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
	var current any = r.doc
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = m[part]
		if !ok {
			return nil
		}
	}
	return current
}

// SpecInfo is the info block of a vendored client spec.
type SpecInfo struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// LoadSpecInfo reads only the info block of an OpenAPI file.
func LoadSpecInfo(specFile string) (*SpecInfo, error) {
	data, err := os.ReadFile(specFile)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Info SpecInfo `yaml:"info"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc.Info, nil
}

// SpecContains reports whether a spec file defines the given operation
// (method+path) or, when schemaName is set, the given component schema.
func SpecContains(specFile, method, path, schemaName string) (bool, error) {
	data, err := os.ReadFile(specFile)
	if err != nil {
		return false, err
	}
	var doc struct {
		Paths      map[string]map[string]any `yaml:"paths"`
		Components struct {
			Schemas map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false, err
	}
	if schemaName != "" {
		_, ok := doc.Components.Schemas[schemaName]
		return ok, nil
	}
	item, ok := doc.Paths[path]
	if !ok {
		return false, nil
	}
	_, ok = item[strings.ToLower(method)]
	return ok, nil
}
