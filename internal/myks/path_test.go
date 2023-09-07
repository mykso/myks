package myks

import (
	"reflect"
	"testing"
)

func Test_findCommonPath(t *testing.T) {
	type args struct {
		changes []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"happy path",
			args{
				[]string{
					"/path1/path3",
					"/path1/path3/file2",
				},
			},
			[]string{
				"/path1/path3",
			},
		},
		{
			"reverse order",
			args{
				[]string{
					"/path1/path3/file2",
					"/path1/path3",
				},
			},
			[]string{
				"/path1/path3",
			},
		},
		{
			"no common path",
			args{
				[]string{
					"/path1/path3/file2",
					"/path1/path4",
				},
			},
			[]string{
				"/path1/path3/file2",
				"/path1/path4",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeSubPaths(tt.args.changes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeSubPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isSubPath(t *testing.T) {
	type args struct {
		path  string
		paths []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSubPath(tt.args.path, tt.args.paths); got != tt.want {
				t.Errorf("isSubPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_removeDuplicates(t *testing.T) {
	type args struct {
		paths []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"happy path", args{[]string{"path1", "path2", "path3", "path2"}}, []string{"path1", "path2", "path3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := removeDuplicates(tt.args.paths); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("removeDuplicates() = %v, want %v", got, tt.want)
			}
		})
	}
}
