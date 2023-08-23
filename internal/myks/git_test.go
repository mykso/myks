package myks

import (
	"reflect"
	"sort"
	"testing"
)

func Test_getChangedFiles(t *testing.T) {
	type args struct {
		revision string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"happy path", args{"HEAD"}, false},
		{"sad path", args{"unknown-revision"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getChangedFiles(tt.args.revision)
			if (err != nil) != tt.wantErr {
				t.Errorf("getChangedFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_getMainBranchHeadRevision(t *testing.T) {
	type args struct {
		mainBranch string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"happy path", args{"main"}, false},
		{"sad path", args{"unknown-branch"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getMainBranchHeadRevision(tt.args.mainBranch)
			if (err != nil) != tt.wantErr {
				t.Errorf("getMainBranchHeadRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Errorf("getMainBranchHeadRevision() must not be empty")
			}
		})
	}
}

func Test_getCurrentBranchHeadRevision(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"happy path", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getCurrentBranchHeadRevision()
			if (err != nil) != tt.wantErr {
				t.Errorf("getCurrentBranchHeadRevision() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Errorf("getCurrentBranchHeadRevision() must not be empty")
			}
		})
	}
}

func Test_convertToChangedFiles(t *testing.T) {
	type args struct {
		changes string
	}
	tests := []struct {
		name string
		args args
		want []ChangedFile
	}{
		{
			"happy path",
			args{
				"A\tfile1\nM\tfile2\nD\tfile3\n",
			},
			[]ChangedFile{
				{path: "file1", status: "A"},
				{path: "file2", status: "M"},
				{path: "file3", status: "D"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := convertToChangedFiles(tt.args.changes); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToChangedFiles() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractChangedFilePathsWithStatus(t *testing.T) {
	type args struct {
		cfs    []ChangedFile
		status string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"filter out deletions",
			args{
				[]ChangedFile{
					{"file1", "M"},
					{"file2", "D"},
					{"file3", "D"},
				},
				"D",
			},
			[]string{"file2", "file3"},
		},
		{
			"filter out noting",
			args{
				[]ChangedFile{
					{"file1", "M"},
					{"file2", "D"},
					{"file3", "D"},
				},
				"",
			},
			[]string{"file1", "file2", "file3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractChangedFilePathsWithStatus(tt.args.cfs, tt.args.status)
			sort.Strings(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractChangedFilePathsWithStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractChangedFilePathsWithoutStatus(t *testing.T) {
	type args struct {
		cfs    []ChangedFile
		status string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			"filter out deletions",
			args{
				[]ChangedFile{
					{"file1", "M"},
					{"file2", "D"},
					{"file3", "D"},
				},
				"D",
			},
			[]string{"file1"},
		},
		{
			"filter out noting",
			args{
				[]ChangedFile{
					{"file1", "M"},
					{"file2", "D"},
					{"file3", "D"},
				},
				"",
			},
			[]string{"file1", "file2", "file3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractChangedFilePathsWithoutStatus(tt.args.cfs, tt.args.status)
			sort.Strings(got)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractChangedFilePathsWithStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
