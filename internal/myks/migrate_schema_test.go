package myks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSchemaInspect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		openapiYAML string
		wantErr     error
		check       func(t *testing.T, got *inspectedSchema)
	}{
		{
			name: "nested defaults with array default []",
			openapiYAML: `
components:
  schemas:
    dataValues:
      type: object
      properties:
        application:
          type: object
          properties:
            image:
              type: string
              default: ""
              minLength: 1
            ingress:
              type: boolean
              default: true
            containerPort:
              type: integer
              default: 80
            env:
              type: array
              items:
                type: object
                properties:
                  name: {type: string, default: TZ}
              default: []
`,
			check: func(t *testing.T, got *inspectedSchema) {
				assert.Equal(t, map[string]any{
					"application": map[string]any{
						"image":         "",
						"ingress":       true,
						"containerPort": 80,
						"env":           []any{},
					},
				}, got.defaults)
				assert.Equal(t, "{str:any}", got.types["application"])
				assert.False(t, got.nullable["application"])
			},
		},
		{
			name: "schema default non-empty array",
			openapiYAML: `
components:
  schemas:
    dataValues:
      type: object
      properties:
        items:
          type: array
          items: {type: string}
          default: [a]
`,
			check: func(t *testing.T, got *inspectedSchema) {
				assert.Equal(t, map[string]any{"items": []any{"a"}}, got.defaults)
			},
		},
		{
			name: "nullable scalar",
			openapiYAML: `
components:
  schemas:
    dataValues:
      type: object
      properties:
        foo:
          type: string
          nullable: true
          default: null
`,
			check: func(t *testing.T, got *inspectedSchema) {
				assert.Equal(t, map[string]any{"foo": nil}, got.defaults)
				assert.True(t, got.nullable["foo"])
				assert.Equal(t, "str", got.types["foo"])
			},
		},
		{
			name: "any-typed node has no type key",
			openapiYAML: `
components:
  schemas:
    dataValues:
      type: object
      properties:
        foo:
          nullable: true
          default: {foo: bar}
`,
			check: func(t *testing.T, got *inspectedSchema) {
				assert.Equal(t, "any", got.types["foo"])
				assert.True(t, got.nullable["foo"])
				assert.Equal(t, map[string]any{"foo": map[string]any{"foo": "bar"}}, got.defaults)
			},
		},
		{
			name: "object property with no properties contributes empty map",
			openapiYAML: `
components:
  schemas:
    dataValues:
      type: object
      properties:
        foo:
          type: object
          additionalProperties: false
`,
			check: func(t *testing.T, got *inspectedSchema) {
				assert.Equal(t, map[string]any{"foo": map[string]any{}}, got.defaults)
			},
		},
		{
			name: "all five constraint kinds with their paths",
			openapiYAML: `
components:
  schemas:
    dataValues:
      type: object
      properties:
        application:
          type: object
          properties:
            image:
              type: string
              default: ""
              minLength: 1
              maxLength: 10
            replicas:
              type: integer
              default: 1
              minimum: 1
              maximum: 5
            tier:
              type: string
              default: a
              enum: [a, b]
`,
			check: func(t *testing.T, got *inspectedSchema) {
				assert.Equal(t, []schemaConstraint{
					{path: []string{"application", "image"}, kind: "maxLength", value: 10},
					{path: []string{"application", "image"}, kind: "minLength", value: 1},
					{path: []string{"application", "replicas"}, kind: "maximum", value: 5.0},
					{path: []string{"application", "replicas"}, kind: "minimum", value: 1.0},
					{path: []string{"application", "tier"}, kind: "enum", value: []any{"a", "b"}},
				}, got.constraints)
			},
		},
		{
			name: "non-string source key false renders as string key",
			openapiYAML: `
components:
  schemas:
    dataValues:
      type: object
      properties:
        false: {type: string, default: x}
`,
			check: func(t *testing.T, got *inspectedSchema) {
				assert.Equal(t, map[string]any{"false": "x"}, got.defaults)
				assert.Equal(t, "str", got.types["false"])
			},
		},
		{
			name: "plain data values document has no schema",
			openapiYAML: `
components:
  schemas:
    dataValues:
      nullable: true
      default: null
`,
			wantErr: errNotASchemaDoc,
		},
		{
			name:        "malformed yaml",
			openapiYAML: "components: [this is not: valid",
			wantErr:     nil, // checked separately below: any non-nil, non-sentinel error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseSchemaInspect([]byte(tt.openapiYAML))

			if tt.name == "malformed yaml" {
				require.Error(t, err)
				assert.NotErrorIs(t, err, errNotASchemaDoc)
				assert.Nil(t, got)
				return
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)
			tt.check(t, got)
		})
	}
}
