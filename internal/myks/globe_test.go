package myks

import "testing"

func TestGlobe_SingleEnv(t *testing.T) {
	type fields struct {
		environments map[string]*Environment
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"before init", fields{map[string]*Environment{}}, false},
		{"happy path", fields{map[string]*Environment{"test-env": {}}}, true},
		{"sad path", fields{map[string]*Environment{"test-env": {}, "test-env2": {}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &Globe{
				environments: tt.fields.environments,
			}
			if got := g.SingleEnv(); got != tt.want {
				t.Errorf("SingleEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
