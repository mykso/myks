package myks

import (
	"errors"
	"fmt"
	"sort"

	yaml "gopkg.in/yaml.v3"
)

// errNotASchemaDoc reports that the inspected file declares no data-values schema.
var errNotASchemaDoc = errors.New("not a data-values schema document")

// inspectedSchema is what a ytt data-values schema says, as the migration needs it.
type inspectedSchema struct {
	// defaults are the resolved default values of the schema, shaped like the data values.
	defaults map[string]any
	// root is the schema of the data values document, kept so that the generated KCL schemas
	// can be typed at every depth, not only at the top level.
	root *openapiNode
	// constraints are the validations the schema carries, at every depth.
	constraints []schemaConstraint
	// demanded holds the paths pruned from the defaults by pruneDemandedDefaults: values the
	// schema validates but supplies no satisfying default for.
	demanded map[string]bool
}

// nodeAt returns the schema node describing the value at path, or nil when the schema says
// nothing about it (a key that only a merged plain data-values file contributed).
func (s *inspectedSchema) nodeAt(path []string) *openapiNode {
	node := s.root
	for _, key := range path {
		if node == nil {
			return nil
		}
		node = node.Properties[key]
	}
	return node
}

// schemaConstraint is one validation, anchored at its path from the document root.
type schemaConstraint struct {
	path  []string // e.g. ["application", "image"]
	kind  string   // one of the constraint kinds below
	value any      // int for lengths, float64 for minimum/maximum, []any for enum
}

// The constraint kinds, named after the OpenAPI keywords ytt reports them under.
const (
	constraintMinLength = "minLength"
	constraintMaxLength = "maxLength"
	constraintMinimum   = "minimum"
	constraintMaximum   = "maximum"
	constraintEnum      = "enum"
)

// openapiDoc is the top-level shape ytt prints for `--data-values-schema-inspect -o openapi-v3`.
type openapiDoc struct {
	Components struct {
		Schemas struct {
			DataValues *openapiNode `yaml:"dataValues"`
		} `yaml:"schemas"`
	} `yaml:"components"`
}

// openapiNode is one schema node. Properties decode into map[string]*openapiNode rather than
// map[string]any: yaml.v3 stringifies a scalar mapping key (e.g. ytt's `false:` for the source
// key `n`) only when the target field type forces it, so a typed decode is what keeps the
// original property name intact.
type openapiNode struct {
	Type                 string                  `yaml:"type"`
	Nullable             bool                    `yaml:"nullable"`
	Default              any                     `yaml:"default"`
	Properties           map[string]*openapiNode `yaml:"properties"`
	AdditionalProperties any                     `yaml:"additionalProperties"`
	Items                *openapiNode            `yaml:"items"`
	MinLength            *int                    `yaml:"minLength"`
	MaxLength            *int                    `yaml:"maxLength"`
	Minimum              *float64                `yaml:"minimum"`
	Maximum              *float64                `yaml:"maximum"`
	Enum                 []any                   `yaml:"enum"`
}

// kclScalarTypes maps an OpenAPI scalar/collection type to its KCL type expression.
var kclScalarTypes = map[string]string{
	"string":  "str",
	"integer": "int",
	"number":  "float",
	"boolean": "bool",
	"object":  "{str:any}",
	"array":   "[any]",
}

// parseSchemaInspect parses the output of `ytt --data-values-schema-inspect -o openapi-v3`
// into the shape the migration needs.
func parseSchemaInspect(openapiYAML []byte) (*inspectedSchema, error) {
	var doc openapiDoc
	if err := yaml.Unmarshal(openapiYAML, &doc); err != nil {
		return nil, fmt.Errorf("parsing schema inspect output: %w", err)
	}

	root := doc.Components.Schemas.DataValues
	if root == nil || (root.Properties == nil && root.Type != "object") {
		return nil, errNotASchemaDoc
	}

	defaults := schemaDefaults(root)

	var constraints []schemaConstraint
	collectConstraints(root, nil, &constraints)
	sort.Slice(constraints, func(i, j int) bool {
		return constraintLess(constraints[i], constraints[j])
	})

	return &inspectedSchema{
		defaults:    defaults,
		root:        root,
		constraints: constraints,
	}, nil
}

// mergeOpenapiNodes folds two schema nodes together, the later one winning: two nodes that
// both describe properties merge property by property, anything else is replaced wholesale.
func mergeOpenapiNodes(base, next *openapiNode) *openapiNode {
	if base == nil || next == nil || base.Properties == nil || next.Properties == nil {
		if next == nil {
			return base
		}
		return next
	}
	for name, prop := range next.Properties {
		base.Properties[name] = mergeOpenapiNodes(base.Properties[name], prop)
	}
	return base
}

// schemaDefaults walks a schema node into its resolved default value: an object property
// recurses into its own properties (an empty map for one that declares none), everything else
// is its own `default` verbatim (ytt already resolves an unset array default to `[]`, so no
// special-casing is needed here). It never descends into `items` — that describes one element,
// not the collection's default.
func schemaDefaults(node *openapiNode) map[string]any {
	out := map[string]any{}
	for name, prop := range node.Properties {
		if prop.Type == "object" {
			out[name] = schemaDefaults(prop)
		} else {
			out[name] = normalizeYAML(prop.Default)
		}
	}
	return out
}

// kclType maps a property's OpenAPI type to its KCL type expression.
func kclType(node *openapiNode) string {
	if t, ok := kclScalarTypes[node.Type]; ok {
		return t
	}
	return "any"
}

// collectConstraints appends every validation found at node and below (excluding items, which
// constrain an element rather than a path in the document) to out.
func collectConstraints(node *openapiNode, path []string, out *[]schemaConstraint) {
	// A validation on the document root constrains no attribute, so there is nowhere to put it.
	if len(path) == 0 {
		for name, prop := range node.Properties {
			collectConstraints(prop, []string{name}, out)
		}
		return
	}
	if node.MinLength != nil {
		*out = append(*out, schemaConstraint{path: path, kind: constraintMinLength, value: *node.MinLength})
	}
	if node.MaxLength != nil {
		*out = append(*out, schemaConstraint{path: path, kind: constraintMaxLength, value: *node.MaxLength})
	}
	if node.Minimum != nil {
		*out = append(*out, schemaConstraint{path: path, kind: constraintMinimum, value: *node.Minimum})
	}
	if node.Maximum != nil {
		*out = append(*out, schemaConstraint{path: path, kind: constraintMaximum, value: *node.Maximum})
	}
	if node.Enum != nil {
		*out = append(*out, schemaConstraint{path: path, kind: constraintEnum, value: normalizeYAML(node.Enum)})
	}
	for name, prop := range node.Properties {
		collectConstraints(prop, append(append([]string{}, path...), name), out)
	}
}

// constraintLess orders constraints by path element by element, then by kind, so the result is
// deterministic regardless of the map iteration order that produced it.
func constraintLess(a, b schemaConstraint) bool {
	for i := 0; i < len(a.path) && i < len(b.path); i++ {
		if a.path[i] != b.path[i] {
			return a.path[i] < b.path[i]
		}
	}
	if len(a.path) != len(b.path) {
		return len(a.path) < len(b.path)
	}
	return a.kind < b.kind
}

// normalizeYAML recursively converts yaml.v3's map[string]interface{} decode of a mapping with
// a non-string key (map[interface{}]interface{}) into map[string]any, stringifying keys with
// fmt.Sprint, so downstream consumers never have to special-case the two shapes.
func normalizeYAML(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[k] = normalizeYAML(vv)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[fmt.Sprint(k)] = normalizeYAML(vv)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = normalizeYAML(vv)
		}
		return out
	default:
		return v
	}
}
