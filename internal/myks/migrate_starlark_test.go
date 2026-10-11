package myks

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTranslateYttLib(t *testing.T) {
	lib := translateYttLib("lib/secrets.star", []byte(`
load("@ytt:struct", "struct")

def sops(name, key):
    return "ref+sops://static/{}.sops.yaml#/{}+".format(name, key.replace(".", "/"))
end

sec = struct.make(sops=sops)
`))
	require.NotNil(t, lib)
	assert.Equal(t, "secrets", lib.name)
	assert.True(t, lib.funcs["sops"])
	assert.Equal(t, `
sops = lambda name, key {
    "ref+sops://static/{}.sops.yaml#/{}+".format(name, key.replace(".", "/"))
}
`, lib.source)
}

func TestTranslateYttLibDicts(t *testing.T) {
	lib := translateYttLib("lib/util.star", []byte(`
#! Container resources with the memory limit pinned.
def resources(cpu, memory):
    return {"requests": {"cpu": cpu, "memory": memory}, "with-dash": 1}
end
`))
	require.NotNil(t, lib)
	assert.Equal(t, `
# Container resources with the memory limit pinned.
resources = lambda cpu, memory {
    {requests = {cpu = cpu, memory = memory}, "with-dash": 1}
}
`, lib.source)
}

func TestYttDerivationsComments(t *testing.T) {
	d := yttDerivations("app-data.yaml", []byte(`#@data/values
#@ # The port every listener shares.
#@ port = 8080
---
application:
  port: #@ port
`), nil, "", "")
	require.NotNil(t, d)
	assert.Equal(t, []string{"_port = 8080"}, d.prelude)
	assert.Equal(t, map[string][]string{"_port = 8080": {"# The port every listener shares."}}, d.notes)
}

func TestYttDerivations(t *testing.T) {
	libs := map[string]*yttLib{"secrets": {name: "secrets", funcs: map[string]bool{"sops": true}}}

	tests := []struct {
		name    string
		content string
		exprs   map[string]string
		prelude []string
		imports []string
	}{
		{
			name: "constant arithmetic",
			content: `#@data/values-schema
---
application:
  config:
    download_timeout: #@ 60 * 10
    purge_files_after: #@ 60 * 60 * 24 * 30
`,
			exprs: map[string]string{
				".application.config.download_timeout":  "60 * 10",
				".application.config.purge_files_after": "60 * 60 * 24 * 30",
			},
		},
		{
			name: "prelude constant read twice",
			content: `#@data/values-schema

#@ port = 8080
---
application:
  port: #@ port
  config:
    listen:
      port: #@ port
`,
			exprs: map[string]string{
				".application.port":               "_port",
				".application.config.listen.port": "_port",
			},
			prelude: []string{"_port = 8080"},
		},
		{
			name: "library call",
			content: `#@ load("secrets.star", "sops")

#@data/values-schema
---
application:
  cloudflare:
    account: #@ sops("0", "cloudflare_account")
`,
			exprs:   map[string]string{".application.cloudflare.account": `lib.sops("0", "cloudflare_account")`},
			imports: []string{"import lib"},
		},
		{
			name: "append loop becomes a comprehension",
			content: `#@data/values

#@ edge_nodes = ["junior"]
#@ base_domain = "zebradil.dev"
#@ lan_domain = "lan." + base_domain
#@ base_hosts = [base_domain]
#@ lan_hosts = [lan_domain]
#@ for node in edge_nodes:
#@   base_hosts.append(node + "." + base_domain)
#@   lan_hosts.append(node + "." + lan_domain)
#@ end
---
environment:
  baseDomain: #@ base_domain
  hosts: #@ base_hosts + lan_hosts
`,
			exprs: map[string]string{
				".environment.baseDomain": "_base_domain",
				".environment.hosts":      "_base_hosts + _lan_hosts",
			},
			prelude: []string{
				`_edge_nodes = ["junior"]`,
				`_base_domain = "zebradil.dev"`,
				`_lan_domain = "lan." + _base_domain`,
				`_base_hosts = [_base_domain] + [node + "." + _base_domain for node in _edge_nodes]`,
				`_lan_hosts = [_lan_domain] + [node + "." + _lan_domain for node in _edge_nodes]`,
			},
		},
		{
			name: "unbound name is left to the literal",
			content: `#@ load("@ytt:data", "data")
#@data/values
---
application:
  host: #@ data.values.environment.baseDomain
  port: #@ 80
`,
			exprs: map[string]string{".application.port": "80"},
		},
		{
			name: "unused prelude is not emitted",
			content: `#@data/values

#@ unused = "x"
#@ used = 1
---
application:
  port: #@ used
`,
			exprs:   map[string]string{".application.port": "_used"},
			prelude: []string{"_used = 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := yttDerivations("test.yaml", []byte(tt.content), libs, "lib", "")
			require.NotNil(t, d)
			assert.Equal(t, tt.exprs, d.exprs)
			assert.Equal(t, tt.prelude, d.prelude)
			assert.Equal(t, tt.imports, d.imports)
		})
	}
}

// translateStarExpr translates one Starlark expression given in source form.
func translateStarExpr(scope *starScope, src string) (string, error) {
	parsed, err := starSyntax.ParseExpr("expr.star", src, 0)
	if err != nil {
		return "", err
	}
	return scope.expr(parsed)
}

func TestStarScopeExprPrecedence(t *testing.T) {
	scope := newStarScope(nil, "lib", "")
	scope.names["a"] = "_a"
	for expr, want := range map[string]string{
		"60 * 60 * 24":            "60 * 60 * 24",
		"(1 + 2) * 3":             "(1 + 2) * 3",
		"1 + 2 * 3":               "1 + 2 * 3",
		"1 - (2 - 3)":             "1 - (2 - 3)",
		`"a" + "b" + "c"`:         `"a" + "b" + "c"`,
		"a if a else None":        "_a if _a else None",
		"[a, 1, True]":            "[_a, 1, True]",
		"not a":                   "not _a",
		"{'k': a}":                `{"k": _a}`,
		"[x * 2 for x in a if x]": "[x * 2 for x in _a if x]",
	} {
		got, err := translateStarExpr(scope, expr)
		require.NoError(t, err, expr)
		assert.Equal(t, want, got, expr)
	}

	for _, expr := range []string{`"%s" % a`, "data.values.x", "f(a)", "a.nope()", "lambda x: x"} {
		_, err := translateStarExpr(scope, expr)
		assert.Error(t, err, expr)
	}
}

func TestVerifyDerivations(t *testing.T) {
	newMigrator := func(dir string) *migrator {
		return &migrator{
			g: &Globe{Config: Config{RootDir: dir, ServiceDirName: ".myks", TempDirName: "tmp", YttLibraryDirName: "lib"}},
			libs: map[string]*yttLib{"secrets": {
				name:   "secrets",
				funcs:  map[string]bool{"sops": true},
				source: "\nsops = lambda name, key {\n    \"ref+\" + name + \"/\" + key\n}\n",
			}},
		}
	}

	t.Run("keeps what KCL reproduces", func(t *testing.T) {
		d := &derivations{
			exprs: map[string]string{
				".timeout": "60 * 10",
				".ref":     `lib.sops("0", "k")`,
				".wrong":   "_port + 1",
			},
			prelude: []string{"_port = 8080", "_unused = 1"},
			imports: []string{"import lib"},
		}
		newMigrator(t.TempDir()).verifyDerivations("app-data.ytt.yaml", d, map[string]any{
			"timeout": 600,
			"ref":     "ref+0/k",
			"wrong":   5,
		})
		assert.Equal(t, map[string]string{".timeout": "60 * 10", ".ref": `lib.sops("0", "k")`}, d.exprs)
		// The prelude of the dropped derivation goes with it.
		assert.Empty(t, d.prelude)
	})

	t.Run("drops everything when the KCL does not evaluate", func(t *testing.T) {
		d := &derivations{exprs: map[string]string{".broken": "1 +"}}
		newMigrator(t.TempDir()).verifyDerivations("app-data.ytt.yaml", d, map[string]any{"broken": 1})
		assert.Empty(t, d.exprs)
	})

	t.Run("drops a value it cannot address", func(t *testing.T) {
		d := &derivations{exprs: map[string]string{".a.b": "1"}}
		newMigrator(t.TempDir()).verifyDerivations("app-data.ytt.yaml", d, map[string]any{"a": 1})
		assert.Empty(t, d.exprs)
	})
}

func TestYttDerivationsWithFunctions(t *testing.T) {
	t.Run("a prelude function becomes a lambda", func(t *testing.T) {
		d := yttDerivations("test.yaml", []byte(`#@data/values

#@ def uri(service):
#@   return "https://{}.example.com".format(service)
#@ end
---
application:
  home: #@ uri("home")
`), nil, "lib", "")
		require.NotNil(t, d)
		assert.Equal(t, map[string]string{".application.home": `_uri("home")`}, d.exprs)
		assert.Equal(t, []string{"_uri = lambda service {\n    \"https://{}.example.com\".format(service)\n}"}, d.prelude)
	})

	t.Run("a ytt template function does not stop the rest", func(t *testing.T) {
		d := yttDerivations("test.yaml", []byte(`#@ port = 8080

#@ def default_env():
- name: ROCKET_PORT
  value: #@ str(port)
#@ end

#@data/values-schema
---
application:
  containerPort: #@ port
  #@schema/default default_env()
  env:
    - name: ""
`), nil, "lib", "")
		require.NotNil(t, d)
		assert.Equal(t, map[string]string{".application.containerPort": "_port"}, d.exprs)
		assert.Equal(t, []string{"_port = 8080"}, d.prelude)
	})
}

func TestYttDerivationsFromEnvironment(t *testing.T) {
	d := yttDerivations("test.yaml", []byte(`#@ load("@myks:data.lib.yaml", "env_data")

#@ def uri(service):
#@   return "https://{}.{}".format(service, env_data.environment.baseDomain)
#@ end

#@data/values
---
application:
  tls:
    baseDomains: #@ env_data.environment.hosts
  links:
    - name: home
      uri: #@ uri("home")
    - name: docs
      uri: #@ uri("docs")
`), nil, "lib", levelVarName)
	require.NotNil(t, d)
	assert.Equal(t, map[string]string{
		".application.tls.baseDomains": "_level.environment.hosts",
		".application.links[0].uri":    `_uri("home")`,
		".application.links[1].uri":    `_uri("docs")`,
	}, d.exprs)
	assert.Equal(t, []string{"_uri = lambda service {\n    \"https://{}.{}\".format(service, _level.environment.baseDomain)\n}"}, d.prelude)
}

func TestValueAtPath(t *testing.T) {
	values := map[string]any{"a": map[string]any{
		"list":    []any{map[string]any{"k": "v"}, map[string]any{"k": "w"}},
		"init.sh": "script",
		"init":    map[string]any{"x": 1},
		"nested":  []any{[]any{"deep"}},
	}}
	for path, want := range map[string]any{
		".a.list[1].k":    "w",
		".a.list[0].k":    "v",
		".a.init.sh":      "script", // a key may contain a dot
		".a.init.x":       1,
		".a.nested[0][0]": "deep",
	} {
		got, found := valueAtPath(values, path)
		require.True(t, found, path)
		assert.Equal(t, want, got, path)
	}
	for _, path := range []string{".a.list[2].k", ".a.list.k", ".a.missing", ".a.list[x].k"} {
		_, found := valueAtPath(values, path)
		assert.False(t, found, path)
	}
}

func TestFoldAppends(t *testing.T) {
	notes := map[string][]string{"_hosts = [_domain]": {"# The apex first."}}
	assert.Equal(t, []string{
		`_domain = "example.com"`,
		`_lan = ["lan"]`,
		`_hosts = [_domain] + [n + "." + _domain for n in _nodes]`,
		`_lan_hosts = _lan + _hosts`,
	}, foldAppends([]string{
		`_domain = "example.com"`,
		`_hosts = [_domain]`,
		`_lan = ["lan"]`,
		`_hosts = _hosts + [n + "." + _domain for n in _nodes]`,
		`_lan_hosts = _lan + _hosts`,
	}, notes))
	assert.Equal(t, []string{"# The apex first."}, notes[`_hosts = [_domain] + [n + "." + _domain for n in _nodes]`])

	assert.Equal(t, []string{`_xs = (_a if _c else _b) + [1]`},
		foldAppends([]string{`_xs = _a if _c else _b`, `_xs = _xs + [1]`}, map[string][]string{}))

	// Read in between, or reading what is rebound in between: folding would change the value.
	kept := []string{`_a = [1]`, `_xs = _a`, `_a = _a + [2]`, `_xs = _xs + [3]`}
	assert.Equal(t, kept, foldAppends(kept, map[string][]string{}))
}

func TestYttDerivationsTextTemplates(t *testing.T) {
	libs := map[string]*yttLib{"secrets": {name: "secrets", funcs: map[string]bool{"sops": true}}}
	d := yttDerivations("app-data.yaml", []byte(`#@ load("secrets.star", "sops")
#@ name = "rsa"
#@data/values
---
files:
  #@yaml/text-templated-strings
  init.sh: |
    #!/bin/sh
    echo "(@= sops("0", "key") @)" > /etc/(@= name @)
  #@yaml/text-templated-strings
  line: 'host-(@= name @) isn''t ${x}'
  #@yaml/text-templated-strings
  code: |
    (@ if True: @)yes(@ end @)
`), libs, "lib", "")
	require.NotNil(t, d)
	assert.Equal(t, map[string]string{
		".files.init.sh": "\"\"\"\\\n#!/bin/sh\necho \"${lib.sops(\"0\", \"key\")}\" > /etc/${_name}\n\"\"\"",
		".files.line":    `'host-${_name} isn\'t \${x}'`,
	}, d.exprs, "code blocks other than (@= @) have no KCL string counterpart")
}

func TestYttDerivationsForEndLoop(t *testing.T) {
	d := yttDerivations("env-data.values.yaml", []byte(`#@data/values
---
kbld:
  overrides:
    #@ registries = [
    #@   "ghcr.io",
    #@   # Disabled until the fix is released.
    #@   #"index.docker.io",
    #@   "quay.io",
    #@ ]
    #@ for/end reg in registries:
    - match:
        registry: #@ reg.replace(".", "\\.")
        repository: (.+)
      replace:
        registry: oci.example
        port: 443
`), nil, "", "")
	require.NotNil(t, d)
	assert.Equal(t, map[string]string{
		".kbld.overrides": `[{match = {registry = reg.replace(".", "\\."), repository = "(.+)"}, replace = {registry = "oci.example", port = 443}} for reg in _registries]`,
	}, d.exprs)
	assert.Equal(t, []string{"_registries = [\n    \"ghcr.io\"\n    # Disabled until the fix is released.\n    # \"index.docker.io\",\n    \"quay.io\"\n]"}, d.prelude)
}
