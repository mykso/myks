package myks

import (
	"os"
	"testing"
)

var prototypeDir = "prototypes"

var appName = "test-app"

var testApp = &Application{
	Name:      appName,
	Prototype: prototypeDir + "/" + appName,
	e: &Environment{
		ID: "test-env",
		g: &Globe{
			TempDirName:   "/tmp",
			PrototypesDir: prototypeDir,
			AppsDir:       "_apps",
		},
		Dir: "/tmp",
	},
	yttDataFiles: nil,
	yttPkgDirs:   nil,
}

type TestTemplateTool struct {
	ident        string
	app          *Application
	additive     bool
	renderedYaml string
}

func (h *TestTemplateTool) IsAdditive() bool {
	return h.additive
}

func (h *TestTemplateTool) Ident() string {
	return h.ident
}

func (h *TestTemplateTool) Render(_ string) (string, error) {
	return h.renderedYaml, nil
}

func TestApplication_Render(t *testing.T) {
	tests := []struct {
		name     string
		args     []YamlTemplatingTool
		want     string
		wantYaml string
		wantErr  bool
	}{
		{
			name:    "empty",
			args:    []YamlTemplatingTool{},
			want:    "",
			wantErr: false,
		},
		{
			name: "Additive",
			args: []YamlTemplatingTool{
				&TestTemplateTool{
					ident:        "test-template",
					app:          testApp,
					additive:     false,
					renderedYaml: "step: One",
				},
				&TestTemplateTool{
					ident:        "test-template-2",
					app:          testApp,
					additive:     true,
					renderedYaml: "step: Two",
				},
			},
			want:     "/tmp/_apps/" + appName + "/steps/01-test-template-2.yaml",
			wantYaml: "step: One\n---\nstep: Two",
			wantErr:  false,
		},
		{
			name: "Non-Additive",
			args: []YamlTemplatingTool{
				&TestTemplateTool{
					ident:        "test-template",
					app:          testApp,
					additive:     false,
					renderedYaml: "step: One",
				},
				&TestTemplateTool{
					ident:        "test-template-2",
					app:          testApp,
					additive:     false,
					renderedYaml: "step: Two",
				},
			},
			want:     "/tmp/_apps/" + appName + "/steps/01-test-template-2.yaml",
			wantYaml: "step: Two",
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testApp.Render(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("render() got = %v, want %v", got, tt.want)
			}
			// check actual step file content
			file, err := os.ReadFile(tt.want)
			if err == nil {
				if string(file) != tt.wantYaml {
					t.Errorf("render() got = %v, want %v", string(file), tt.wantYaml)
				}
			}
		})
	}
}

func Test_genRenderedResourceFileName(t *testing.T) {
	type args struct {
		resource         map[string]any
		includeNamespace bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"happy path", args{map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "test", "namespace": "test-ns"}}, true}, "deployment-test_test-ns.yaml", false},
		{"generateName", args{map[string]any{"kind": "Deployment", "metadata": map[string]any{"generateName": "test-", "namespace": "test-ns"}}, true}, "deployment-test_test-ns.yaml", false},
		{"generateName no suffix", args{map[string]any{"kind": "Deployment", "metadata": map[string]any{"generateName": "test", "namespace": "test-ns"}}, true}, "deployment-test_test-ns.yaml", false},
		{"no namespace", args{map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "test", "namespace": "test-ns"}}, false}, "deployment-test.yaml", false},
		{"no kind", args{map[string]any{"metadata": map[string]any{"name": "test", "namespace": "test-ns"}}, false}, "", true},
		{"no name", args{map[string]any{"metadata": map[string]any{"kind": "Deployment", "namespace": "test-ns"}}, false}, "", true},
		// Test cases for Windows-incompatible characters
		{"colon in name", args{map[string]any{"kind": "ClusterRole", "metadata": map[string]any{"name": "cert-manager-controller-approve:cert-manager-io"}}, false}, "clusterrole-cert-manager-controller-approve_cert-manager-io.yaml", false},
		{"multiple colons", args{map[string]any{"kind": "Role", "metadata": map[string]any{"name": "test:one:two", "namespace": "ns:with:colon"}}, true}, "role-test_one_two_ns_with_colon.yaml", false},
		{"slash in name", args{map[string]any{"kind": "ConfigMap", "metadata": map[string]any{"name": "test/config"}}, false}, "configmap-test_config.yaml", false},
		{"backslash in name", args{map[string]any{"kind": "Secret", "metadata": map[string]any{"name": "test\\secret"}}, false}, "secret-test_secret.yaml", false},
		{"pipe in name", args{map[string]any{"kind": "Service", "metadata": map[string]any{"name": "test|service"}}, false}, "service-test_service.yaml", false},
		{"question mark in name", args{map[string]any{"kind": "Pod", "metadata": map[string]any{"name": "test?pod"}}, false}, "pod-test_pod.yaml", false},
		{"asterisk in name", args{map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "test*deploy"}}, false}, "deployment-test_deploy.yaml", false},
		{"angle brackets in name", args{map[string]any{"kind": "Deployment", "metadata": map[string]any{"name": "test<>deploy"}}, false}, "deployment-test__deploy.yaml", false},
		{"quotes in name", args{map[string]any{"kind": "Service", "metadata": map[string]any{"name": "test\"service"}}, false}, "service-test_service.yaml", false},
		{"multiple special chars", args{map[string]any{"kind": "ClusterRole", "metadata": map[string]any{"name": "admin:read|write*all"}}, false}, "clusterrole-admin_read_write_all.yaml", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := genRenderedResourceFileName(tt.args.resource, tt.args.includeNamespace)
			if (err != nil) != tt.wantErr {
				t.Errorf("genRenderedResourceFileName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("genRenderedResourceFileName() got = %v, want %v", got, tt.want)
			}
		})
	}
}
