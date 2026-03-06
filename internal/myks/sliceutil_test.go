package myks

import (
	"reflect"
	"testing"
)

func Test_appendIfNotExists(t *testing.T) {
	type args struct {
		slice   []string
		element string
	}
	tests := []struct {
		name      string
		args      args
		wantArgs  []string
		wantAdded bool
	}{
		{"add dup", args{[]string{"test"}, "test"}, []string{"test"}, false},
		{"add new element", args{[]string{"test"}, "test2"}, []string{"test", "test2"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, added := appendIfNotExists(tt.args.slice, tt.args.element)
			if !reflect.DeepEqual(got, tt.wantArgs) {
				t.Errorf("appendIfNotExists() = %v, wantArgs %v", got, tt.wantArgs)
			}
			if !added == tt.wantAdded {
				t.Errorf("appendIfNotExists() = %v, wantAdded %v", got, tt.wantArgs)
			}
		})
	}
}

func Test_extract(t *testing.T) {
	type TestMe struct {
		Name string
	}
	type args[T any] struct {
		items      []T
		filterFunc func(cf T) bool
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want []T
	}
	tests := []testCase[TestMe]{
		{
			name: "happy path",
			args: args[TestMe]{
				[]TestMe{
					{Name: "test1"},
					{Name: "test2"},
				},
				func(cf TestMe) bool {
					return cf.Name == "test1"
				},
			},
			want: []TestMe{
				{Name: "test1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := filterSlice(tt.args.items, tt.args.filterFunc); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extract() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_unique(t *testing.T) {
	type args struct {
		slice []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"happy path", args{[]string{"test", "test", "test2"}}, []string{"test", "test2"}},
		{"empty slice", args{[]string{}}, []string{}},
		{"one element", args{[]string{"test"}}, []string{"test"}},
		{"several duplicates", args{[]string{"test", "test", "test"}}, []string{"test"}},
		{"grouped duplicates", args{[]string{"test", "test2", "test", "test2"}}, []string{"test", "test2"}},
		{"no duplicates", args{[]string{"test", "test2"}}, []string{"test", "test2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unique(tt.args.slice); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unique() = %v, want %v", got, tt.want)
			}
		})
	}
}
