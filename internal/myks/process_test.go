package myks

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"maps"
	"reflect"
	"slices"
	"sync"
	"testing"
)

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
			got, err := runCmd("test", tt.args.name, tt.args.stdin, tt.args.args, nil, tt.args.log)
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
			got, err := runYttWithFilesAndStdin("test", tt.args.paths, tt.args.stdin, nil, tt.args.log, tt.args.args...)
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
		collection      any
		expectedFnCalls int
		fn              func(int) error
		expectedErr     error
	}{
		{
			name:            "Successful async processing of slice",
			asyncLevel:      2,
			collection:      []int{1, 2, 3, 4, 5},
			expectedFnCalls: 5,
			fn: func(item int) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful async processing of map",
			asyncLevel:      2,
			collection:      map[string]int{"one": 1, "two": 2, "three": 3},
			expectedFnCalls: 3,
			fn: func(item int) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful sync processing of slice",
			asyncLevel:      0,
			collection:      []int{1, 2, 3, 4, 5},
			expectedFnCalls: 5,
			fn: func(item int) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful sync processing of map",
			asyncLevel:      0,
			collection:      map[string]int{"one": 1, "two": 2, "three": 3},
			expectedFnCalls: 3,
			fn: func(item int) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:            "Successful async processing of slice with high async level",
			asyncLevel:      222,
			collection:      []int{1, 2, 3, 4, 5},
			expectedFnCalls: 5,
			fn: func(item int) error {
				return nil
			},
			expectedErr: nil,
		},
		{
			name:       "Error in processing slice",
			asyncLevel: 2,
			collection: []int{1, 2, 3, 4, 5},
			fn: func(item int) error {
				if item == 3 {
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
			fn: func(item int) error {
				if item == 2 {
					return errors.New("error processing item")
				}
				return nil
			},
			expectedErr: errors.New("error processing item"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var counter int
			var mu sync.Mutex

			fnWrapper := func(item int) error {
				mu.Lock()
				counter++
				mu.Unlock()
				return tc.fn(item)
			}

			var collection iter.Seq[int]
			if slice, ok := tc.collection.([]int); ok {
				collection = slices.Values(slice)
			} else if m, ok := tc.collection.(map[string]int); ok {
				collection = maps.Values(m)
			} else {
				t.Fatalf("unexpected type: %T", tc.collection)
			}

			for item := range collection {
				fmt.Println(item)
			}
			err := process(tc.asyncLevel, collection, fnWrapper)
			if fmt.Sprint(err) != fmt.Sprint(tc.expectedErr) {
				t.Errorf("expected error: %v, got: %v", tc.expectedErr, err)
			}

			if tc.expectedFnCalls > 0 && counter != tc.expectedFnCalls {
				t.Errorf("expected fn to be called %d times, got: %d", tc.expectedFnCalls, counter)
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
		{"bitnami", args{"https://charts.bitnami.com/bitnami"}, "charts.bitnami.com-bitnami"},
		{"stable", args{"https://charts.helm.sh/stable"}, "charts.helm.sh-stable"},
		{"grafana", args{"https://grafana.github.io/helm-charts"}, "grafana.github.io-helm-charts"},
		{"nginx", args{"https://helm.nginx.com/stable"}, "helm.nginx.com-stable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createURLSlug(tt.args.input); got != tt.want {
				t.Errorf("createURLSlug() = %v, want %v", got, tt.want)
			}
		})
	}
}
