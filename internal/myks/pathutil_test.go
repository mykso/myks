package myks

import (
	"testing"
)

func Test_findSubPath(t *testing.T) {
	type args struct {
		path    string
		subPath string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want2 bool
	}{
		{"short path", args{"/tmp/test", "/tmp"}, "/tmp", true},
		{"long path", args{"/tmp/test/charts/multus", "charts"}, "/tmp/test/charts", true},
		{"no match", args{"/tmp/test/charts/multus", "no-match"}, "", false},
		{"double match", args{"/tmp/test/charts/multus/charts/test", "charts"}, "/tmp/test/charts", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, found := findSubPath(tt.args.path, tt.args.subPath); got != tt.want || found != tt.want2 {
				t.Errorf("findSubPath() = %v, want %v and want2 %v", got, tt.want, tt.want2)
			}
		})
	}
}
