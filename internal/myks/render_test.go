package myks

import (
	"os"
	"testing"
)

var testApp = &Application{
	Name:      "",
	Prototype: "",
	e: &Environment{
		Id: "test-env",
		g: &Globe{
			TempDirName: "/tmp",
		},
		Dir: "/tmp",
	},
	yttDataFiles: nil,
	cached:       false,
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
			want:     "/tmp/_apps/tmp/steps/01-test-template-2.yaml",
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
			want:     "/tmp/_apps/tmp/steps/01-test-template-2.yaml",
			wantYaml: "step: Two",
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testApp.Render(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Render() got = %v, want %v", got, tt.want)
			}
			// check actual step file content
			file, err := os.ReadFile(tt.want)
			if err == nil {
				if string(file) != tt.wantYaml {
					t.Errorf("Render() got = %v, want %v", string(file), tt.wantYaml)
				}
			}
		})
	}
}
