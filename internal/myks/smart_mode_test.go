package myks

import (
	"github.com/creasty/defaults"
	"reflect"
	"sort"
	"testing"
)

func Test_getChanges(t *testing.T) {
	type args struct {
		diff         []string
		regExPattern string
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
					"path1/file1",
					"path1/file2",
				},
				"^path1/(.*)$",
			},
			[]string{
				"file1",
				"file2",
			},
		},
		{
			"no capture group",
			args{
				[]string{
					"path1/file1",
					"path1/file2",
				},
				"^path1/.*$",
			},
			[]string{
				"path1/file1",
				"path1/file2",
			},
		},
		{
			"no match",
			args{
				[]string{
					"nothing-to-match",
				},
				"^path1/.*$",
			},
			nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := getChanges(tt.args.diff, tt.args.regExPattern); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkFileChanged(t *testing.T) {
	type args struct {
		changedFiles []string
		regExps      []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"happy path", args{[]string{"path1/file1"}, []string{"^path1/(.*)$"}}, true},
		{"no match", args{[]string{"path1/file1"}, []string{"no-match"}}, false},
		{"empty", args{[]string{}, []string{"no-match"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkFileChanged(tt.args.changedFiles, tt.args.regExps...); got != tt.want {
				t.Errorf("checkFileChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobe_getModifiedEnvs(t *testing.T) {
	type args struct {
		changedFiles []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"cross check", args{[]string{"some-irrelevant-path.yaml"}}, nil},
		{"happy path", args{[]string{
			"envs/env1/env-data.ytt.yaml",
			"envs/sub-env/env2/env-data.ytt.yaml",
			"envs/sub-env/env4/some-file.ytt.yaml",
		}}, []string{
			"envs/env1",
			"envs/sub-env/env2",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createGlobe(t)
			envs := g.getModifiedEnvs(tt.args.changedFiles)
			sort.Strings(envs)
			if got := envs; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobe_getModifiedBaseApps(t *testing.T) {
	type args struct {
		changedFiles []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{"cross check", args{[]string{"some-irrelevant-path.yaml"}}, nil},
		{"happy path", args{[]string{
			"prototypes/app1/app-data.ytt.yaml",
			"prototypes/app2/vendir/app.yaml",
			"prototypes/app3/ytt/app.yaml",
			"prototypes/app4/ytt-pkg/app.yaml",
			"prototypes/app5/helm/app.yaml",
			"prototypes/app5/any/app.yaml",
		}}, []string{
			"app1",
			"app2",
			"app3",
			"app4",
			"app5",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createGlobe(t)
			apps := g.getModifiedBaseApps(tt.args.changedFiles)
			sort.Strings(apps)
			if got := apps; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobe_getModifiedApps(t *testing.T) {
	type args struct {
		changedFiles []string
		deletedEnvs  []string
	}
	tests := []struct {
		name     string
		args     args
		wantEnvs []string
		wantApps []string
	}{
		{"cross check", args{[]string{"some-irrelevant-path.yaml"}, nil}, nil, nil},
		{"happy path", args{[]string{
			"envs/env1/_apps/app1/app.yaml",
			"envs/env1/env2/_apps/app2/app.yaml",
			"envs/env1/no-app/test.yaml",
			"base/env1/env2/_apps/app2/app.yaml",
		}, nil},
			[]string{
				"envs/env1",
				"envs/env1/env2",
			},
			[]string{
				"app1",
				"app2",
			},
		},
		{"exclude deleted env", args{[]string{
			"envs/env1/_apps/app1/app.yaml",
			"envs/env2/_apps/app2/app.yaml",
		}, []string{
			"envs/env1",
		}},
			[]string{
				"envs/env2",
			},
			[]string{
				"app2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createGlobe(t)
			gotEnvs, gotApps := g.getModifiedApps(tt.args.changedFiles, tt.args.deletedEnvs)
			sort.Strings(gotEnvs)
			sort.Strings(gotApps)
			if !reflect.DeepEqual(gotEnvs, tt.wantEnvs) {
				t.Errorf("getChanges() = %v, want %v", gotEnvs, tt.wantEnvs)
			}
			if !reflect.DeepEqual(gotApps, tt.wantApps) && !reflect.DeepEqual(gotApps, tt.wantApps) {
				t.Errorf("getChanges() = %v, want %v", gotApps, tt.wantApps)
			}
		})
	}
}

func TestGlobe_checkGlobalConfigChanged(t *testing.T) {
	type args struct {
		changedFiles []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"cross check", args{[]string{"envs/some-env/env-data.ytt.yaml"}}, false},
		{"common lib", args{[]string{"lib/file1"}}, true},
		{"common lib sub", args{[]string{"lib/sub/file1"}}, true},
		{"match with additional file", args{[]string{"lib/sub/file1", "some-irrelevant-file"}}, true},
		{"common ytt lib", args{[]string{"envs/_env/ytt/file1"}}, true},
		{"common ytt lib sub", args{[]string{"envs/_env/ytt/sub/file1"}}, true},
		{"root env", args{[]string{"envs/env-data.ytt.yaml"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createGlobe(t)
			if got := g.checkGlobalConfigChanged(tt.args.changedFiles); got != tt.want {
				t.Errorf("checkGlobalConfigChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobe_findBaseAppUsage(t *testing.T) {
	type args struct {
		baseApps []string
		globe    Globe
	}
	tests := []struct {
		name     string
		args     args
		wantEnvs []string
		wantApps []string
	}{
		{
			"happy path",
			args{
				[]string{"app1"},
				Globe{
					environments: map[string]*Environment{
						"env1": {
							Id: "env1",
							foundApplications: map[string]string{
								"app1": "app1",
								"app2": "app2",
							},
						},
					},
				},
			},
			[]string{"env1"},
			[]string{"app1"},
		},
		{
			"prototype ref",
			args{
				[]string{"app1", "app2"},
				Globe{
					environments: map[string]*Environment{
						"env1": {
							Id: "env1",
							foundApplications: map[string]string{
								"app1":      "my-app-1",
								"root/app2": "my-app-2",
							},
						},
					},
				},
			},
			[]string{"env1"},
			[]string{"my-app-1", "my-app-2"},
		},
		{
			"duplicates",
			args{
				[]string{"app1", "app2"},
				Globe{
					environments: map[string]*Environment{
						"env1": {
							Id: "env1",
							foundApplications: map[string]string{
								"app1": "my-app-1",
							},
						},
						"env2": {
							Id: "env2",
							foundApplications: map[string]string{
								"app1": "my-app-1",
							},
						},
					},
				},
			},
			[]string{"env1", "env2"},
			[]string{"my-app-1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEnvs, gotApps := tt.args.globe.findBaseAppUsage(tt.args.baseApps)
			sort.Strings(gotEnvs)
			sort.Strings(gotApps)
			if !reflect.DeepEqual(gotEnvs, tt.wantEnvs) {
				t.Errorf("findBaseAppUsage() got = %v, want %v", gotEnvs, tt.wantEnvs)
			}
			if !reflect.DeepEqual(gotApps, tt.wantApps) {
				t.Errorf("findBaseAppUsage() got1 = %v, want %v", gotApps, tt.wantApps)
			}
		})
	}
}

func TestGlobe_runSmartMode(t *testing.T) {
	g := createGlobe(t)
	g.environments = map[string]*Environment{
		"envs/env1": {
			Id: "env1",
			foundApplications: map[string]string{
				"app1": "app1",
				"app2": "app2",
			},
		},
		"envs/env2": {
			Id: "env2",
			foundApplications: map[string]string{
				"app3": "app3",
				"app2": "app2",
			},
		},
	}
	type args struct {
		changedFiles []ChangedFile
	}
	tests := []struct {
		name     string
		args     args
		wantEnvs []string
		wantApps []string
	}{
		{
			"change to global lib",
			args{
				[]ChangedFile{{"lib/file1", "M"}},
			},
			nil,
			nil,
		},
		{
			"change to base app",
			args{
				[]ChangedFile{{"prototypes/app1/app-data.ytt.yaml", "M"}},
			},
			[]string{"envs/env1"},
			[]string{"app1"},
		},
		{
			"change to app",
			args{
				[]ChangedFile{{"envs/env1/_apps/app1/app-data.ytt.yaml", "M"}},
			},
			[]string{"envs/env1"},
			[]string{"app1"},
		},
		{
			"change to env",
			args{
				[]ChangedFile{
					{"envs/env1/env-data.ytt.yaml", "M"},
					{"envs/env1/_apps/app1/app-data.ytt.yaml", "M"},
				},
			},
			[]string{"envs/env1"},
			nil,
		},
		{
			"ignore env deletion",
			args{
				[]ChangedFile{
					{"envs/env1/env-data.ytt.yaml", "D"},
				},
			},
			nil,
			nil,
		},
		{
			"changes to all multiple envs and apps",
			args{
				[]ChangedFile{
					{"prototypes/app2/app-data.ytt.yaml", "M"},
					{"envs/env2/_apps/app3/some-file.yaml", "M"},
				},
			},
			[]string{"envs/env1", "envs/env2"},
			[]string{"app2", "app3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEnvs, gotApps := g.runSmartMode(tt.args.changedFiles)
			sort.Strings(gotEnvs)
			sort.Strings(gotApps)
			if !reflect.DeepEqual(gotEnvs, tt.wantEnvs) {
				t.Errorf("runSmartMode() got = %v, want %v", gotEnvs, tt.wantEnvs)
			}
			if !reflect.DeepEqual(gotApps, tt.wantApps) {
				t.Errorf("runSmartMode() got1 = %v, want %v", gotApps, tt.wantApps)
			}
		})
	}
}

func createGlobe(t *testing.T) *Globe {
	g := &Globe{}
	if err := defaults.Set(g); err != nil {
		t.Errorf("failed to create Globe object")
	}
	return g
}
