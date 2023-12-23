package myks

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

const testDataDir = "../../testData/util"

func Test_hash(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want string
	}{
		{"happy path", "some-string", "a3635c09bda7293ae1f144a240f155cf151451f2420d11ac385d13cce4eb5fa2"},
	}
	for _, tt := range tests {
		t.Run(tt.a, func(t *testing.T) {
			if got := hashString(tt.b); got != tt.want {
				t.Errorf("hash() = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_sortYaml(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string
		wantErr bool
	}{
		{
			"happy path",
			map[string]interface{}{"key1": "A", "key2": "B"},
			"map[key1:A key2:B]",
			false,
		},
		{
			"fix sorting",
			map[string]interface{}{"key2": "B", "key1": "A"},
			"map[key1:A key2:B]",
			false,
		},
		{
			"empty input",
			nil,
			"",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sortYaml(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("sortYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("sortYaml() got = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_unmarshalYaml(t *testing.T) {
	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]interface{}
		wantErr bool
	}{
		{"happy path", args{"../../testData/util/yaml/simple.yaml"}, map[string]interface{}{"key1": "A", "key2": "B", "arr": []interface{}{"arr1", "arr2"}}, false},
		{"file not exist", args{"non-existing.yaml"}, map[string]interface{}{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalYamlToMap(tt.args.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshalYamlToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unmarshalYamlToMap() got = %v, wantArgs %v", got, tt.want)
			}
		})
	}
}

func Test_createDirectory(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"happy path", args{"/tmp/test-dir"}, false},
		{"sad path", args{"/non-existing/test-dir"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := createDirectory(tt.args.dir); (err != nil) != tt.wantErr {
				t.Errorf("createDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_writeFile(t *testing.T) {
	type args struct {
		path    string
		content []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"happy path", args{"/tmp/test-file", []byte("test")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeFile(tt.args.path, tt.args.content); (err != nil) != tt.wantErr {
				t.Errorf("writeFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			file, err := os.ReadFile(tt.args.path)
			if err != nil {
				t.Errorf("writeFile() error = %v", err)
			}
			if string(file) != string(tt.args.content) {
				t.Errorf("writeFile() got = %v, wantArgs %v", string(file), string(tt.args.content))
			}
		})
	}
}

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

func Test_reductSecrets(t *testing.T) {
	type args struct {
		args []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"happy path", args{[]string{"password=verysecret", "secret=verysecret", "token=verysecret"}}, []string{"password=[REDACTED]", "secret=[REDACTED]", "token=[REDACTED]"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reductSecrets(tt.args.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reductSecrets() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getSubDirs(t *testing.T) {
	baseDir := testDataDir + "/getSubDirs"
	tests := []struct {
		name    string
		dir     string
		want    []string
		wantErr bool
	}{
		{"one subdir", baseDir + "/one", []string{baseDir + "/one/foo"}, false},
		{"two subdirs", baseDir + "/two", []string{baseDir + "/two/.baz", baseDir + "/two/bar"}, false},
		{"no subdirs", baseDir + "/none", nil, false},
		{"empty dir name", "", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getSubDirs(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("error: %v, wantErr: %v", err, tt.wantErr)
			} else {
				assertEqual(t, got, tt.want)
			}
		})
	}
}

func Test_runCmd(t *testing.T) {
	type args struct {
		name  string
		stdin io.Reader
		args  []string
		log   func(name string, err error, stderr string, args []string)
	}
	tests := []struct {
		name    string
		args    args
		want    CmdResult
		wantErr bool
	}{
		{"happy path", args{"echo", nil, []string{"test"}, nil}, CmdResult{"test\n", ""}, false},
		{"sad path", args{"sure-to-fail", nil, []string{}, nil}, CmdResult{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runCmd(tt.args.name, tt.args.stdin, tt.args.args, tt.args.log)
			if (err != nil) != tt.wantErr {
				t.Errorf("runCmd() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("runCmd() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_runYttWithFilesAndStdin(t *testing.T) {
	type args struct {
		paths []string
		stdin io.Reader
		log   func(name string, err error, stderr string, args []string)
		args  []string
	}
	tests := []struct {
		name    string
		args    args
		want    CmdResult
		wantErr bool
	}{
		{"happy path", args{[]string{"../../testData/ytt/simple.yaml"}, nil, nil, []string{}}, CmdResult{"key1: A\n", ""}, false},
		{"sad path", args{[]string{"does-not-exist.yaml"}, nil, nil, []string{}}, CmdResult{"", ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := runYttWithFilesAndStdin(tt.args.paths, tt.args.stdin, tt.args.log, tt.args.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("runYttWithFilesAndStdin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("runYttWithFilesAndStdin() got = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestProcess(t *testing.T) {
	testCases := []struct {
		name            string
		asyncLevel      int
		collection      interface{}
		expectedFnCalls int
		fn              func(interface{}) error
		expectedErr     error
	}{
		{
			name:            "Successful async processing of slice",
			asyncLevel:      2,
			collection:      []int{1, 2, 3, 4, 5},
			expectedFnCalls: 5,
			fn: func(item interface{}) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful async processing of map",
			asyncLevel:      2,
			collection:      map[string]int{"one": 1, "two": 2, "three": 3},
			expectedFnCalls: 3,
			fn: func(item interface{}) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful sync processing of slice",
			asyncLevel:      0,
			collection:      []int{1, 2, 3, 4, 5},
			expectedFnCalls: 5,
			fn: func(item interface{}) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful sync processing of map",
			asyncLevel:      0,
			collection:      map[string]int{"one": 1, "two": 2, "three": 3},
			expectedFnCalls: 3,
			fn: func(item interface{}) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful async processing of slice with high async level",
			asyncLevel:      222,
			collection:      []int{1, 2, 3, 4, 5},
			expectedFnCalls: 5,
			fn: func(item interface{}) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:       "Error in processing slice",
			asyncLevel: 2,
			collection: []int{1, 2, 3, 4, 5},
			fn: func(item interface{}) error {
				if item.(int) == 3 {
					return errors.New("error processing item")
				}
				return nil
			},
			expectedErr: errors.New("error processing item"),
		},
		{
			name:       "Error in processing map",
			asyncLevel: 2,
			collection: map[string]int{"one": 1, "two": 2, "three": 3},
			fn: func(item interface{}) error {
				if item.(int) == 2 {
					return errors.New("error processing item")
				}
				return nil
			},
			expectedErr: errors.New("error processing item"),
		},
		{
			name:       "Invalid collection type",
			asyncLevel: 2,
			collection: 42,
			fn: func(item interface{}) error {
				return nil
			},
			expectedErr: fmt.Errorf("collection must be a slice, array or map, got %s", reflect.Int),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var counter int
			var mu sync.Mutex

			fnWrapper := func(item interface{}) error {
				mu.Lock()
				counter++
				mu.Unlock()
				return tc.fn(item)
			}

			err := process(tc.asyncLevel, tc.collection, fnWrapper)
			if fmt.Sprint(err) != fmt.Sprint(tc.expectedErr) {
				t.Errorf("Expected error: %v, got: %v", tc.expectedErr, err)
			}

			if tc.expectedFnCalls > 0 && counter != tc.expectedFnCalls {
				t.Errorf("Expected fn to be called %d times, got: %d", tc.expectedFnCalls, counter)
			}
		})
	}
}

func assertEqual(t *testing.T, got, want interface{}) {
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expected:\n%v\nGot:\n%v\nDifference:\n%v", want, got, diff(want, got))
	}
}

func diff(expected, actual interface{}) string {
	jsonExpected, err := json.MarshalIndent(expected, "", "  ")
	if err != nil {
		return err.Error()
	}
	jsonActual, err := json.MarshalIndent(actual, "", "  ")
	if err != nil {
		return err.Error()
	}
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(jsonExpected)),
		B:        difflib.SplitLines(string(jsonActual)),
		FromFile: "Expected",
		ToFile:   "Actual",
		Context:  3,
	}

	text, _ := difflib.GetUnifiedDiffString(diff)
	return text
}

// chdir changes the current working directory to the named directory and
// returns a function that, when called, restores the original working
// directory.
// Usage:
//
//	defer chdir(t, "testdata")()
//
// Credit: https://github.com/golang/go/issues/45182#issue-838791504
func chdir(t *testing.T, dir string) func() {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	return func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restoring working directory: %v", err)
		}
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

func Test_createURLSlug(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"bitnami", args{"https://charts.bitnami.com/bitnami"}, "charts.bitnami.com"},
		{"stable", args{"https://charts.helm.sh/stable"}, "charts.helm.sh"},
		{"grafana", args{"https://grafana.github.io/helm-charts"}, "grafana.github.io"},
		{"nginx", args{"https://helm.nginx.com/stable"}, "helm.nginx.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createURLSlug(tt.args.input); got != tt.want {
				t.Errorf("createURLSlug() = %v, want %v", got, tt.want)
			}
		})
	}
}
