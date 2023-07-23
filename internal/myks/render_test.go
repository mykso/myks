package myks

import (
	"reflect"
	"testing"
)

func Test_getVendoredResourceDirs(t *testing.T) {
	type args struct {
		resourceDir string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"happy path", args{"../../testData/vendor/charts"}, []string{"../../testData/vendor/charts/test-chart"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getVendoredResourceDirs(tt.args.resourceDir); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getVendoredResourceDirs() = %v, want %v", got, tt.want)
			}
		})
	}
}
