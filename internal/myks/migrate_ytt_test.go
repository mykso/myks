package myks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	yaml "gopkg.in/yaml.v3"
)

// TestSplitYttFile covers the split that lets a file mixing plain YAML with ytt computation be
// converted instead of skipped: what ytt computes goes, what the file states stays.
func TestSplitYttFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		content  string
		want     map[string]any
		deferred []string
	}{
		{
			name: "an inline expression takes only its own key",
			content: `#@ load("@myks:data.lib.yaml", "env_data")

#@data/values
---
application:
  tls:
    baseDomains: #@ env_data.environment.hosts
    issuer: cloudflare-zerossl
`,
			want:     map[string]any{"application": map[string]any{"tls": map[string]any{"issuer": "cloudflare-zerossl"}}},
			deferred: []string{".application.tls.baseDomains"},
		},
		{
			name: "a computation inside a sequence takes the whole entry",
			content: `#@data/values
---
helm:
  capabilities:
  - monitoring.coreos.com/v1
kbld:
  enabled: true
  overrides:
    #@ for/end reg in ["ghcr.io"]:
    - match:
        registry: #@ reg
`,
			want: map[string]any{
				"helm": map[string]any{"capabilities": []any{"monitoring.coreos.com/v1"}},
				"kbld": map[string]any{"enabled": true},
			},
			deferred: []string{".kbld.overrides"},
		},
		{
			name: "a mapping whose entries all go, goes with them",
			content: `#@ load("secrets.star", "sops")

#@data/values
---
application:
  nodeName: junior
  wireguard:
    config: #@ sops("0", "wireguard_config")
`,
			want:     map[string]any{"application": map[string]any{"nodeName": "junior"}},
			deferred: []string{".application.wireguard.config"},
		},
		{
			name: "a templated string is computation too",
			content: `#@data/values
---
application:
  plain: keep
  #@yaml/text-templated-strings
  script: |
    (@= secret @)
`,
			want:     map[string]any{"application": map[string]any{"plain": "keep"}},
			deferred: []string{".application.script"},
		},
		{
			name: "an overlay that rewrites instead of merging is computation",
			content: `#@data/values
---
kbld:
  #@overlay/replace
  overrides: []
myks:
  gitRepoBranch: main
`,
			want:     map[string]any{"myks": map[string]any{"gitRepoBranch": "main"}},
			deferred: []string{".kbld.overrides"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			split, err := splitYttFile([]byte(tt.content))
			require.NoError(t, err)
			assert.Equal(t, tt.deferred, split.deferred)

			got := map[string]any{}
			require.NoError(t, yaml.Unmarshal(split.sanitized, &got))
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestSplitYttFileKeepsNothing pins the fall-back signal: a file whose every value is computed
// has nothing left to convert, and the caller skips it whole.
func TestSplitYttFileKeepsNothing(t *testing.T) {
	t.Parallel()
	split, err := splitYttFile([]byte("#@ load(\"secrets.star\", \"sops\")\n#@data/values\n---\napplication:\n  password: #@ sops(\"0\", \"p\")\n"))
	require.NoError(t, err)
	assert.Zero(t, split.kept)
}

func TestDeepestPaths(t *testing.T) {
	t.Parallel()
	assert.Equal(t,
		[]string{".a.b.c", ".a.d", ".e"},
		deepestPaths([]string{".a", ".a.b", ".a.b.c", ".e", ".a.d", ".a"}))
}

func TestYttComments(t *testing.T) {
	t.Parallel()
	comments, err := yttComments([]byte(`#@data/values-schema
#@overlay/match-child-defaults missing_ok=True
---
application:
  #! renovate: datasource=docker
  image: nginx:1.31.5
  #! Two lines,
  #! both kept.
  #@schema/validation min_len=1
  name: ""
  clients:
    #! a comment inside a sequence has no attribute to sit above
    - host: ""
# plain YAML comments travel too
port: 8080

#! this block ends the file and belongs to nothing
`))
	require.NoError(t, err)

	assert.Equal(t, []yttComment{
		{path: []string{"application", "image"}, lines: []string{"# renovate: datasource=docker"}},
		{path: []string{"application", "name"}, lines: []string{"# Two lines,", "# both kept."}},
		{path: []string{"port"}, lines: []string{"# plain YAML comments travel too"}},
		{lines: []string{"# this block ends the file and belongs to nothing"}},
	}, comments)
}

func TestKclComment(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "# renovate: datasource=docker", kclComment("#! renovate: datasource=docker"))
	assert.Equal(t, "# no space after the marker", kclComment("#no space after the marker"))
	assert.Equal(t, "#", kclComment("#"))
}
