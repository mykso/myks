package myks

import (
	"reflect"
	"strings"
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
			_, err := GetChangedFilesGit(tt.args.revision)
			if (err != nil) != tt.wantErr {
				t.Errorf("getChangedFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_convertDiffToChangedFiles(t *testing.T) {
	niceIn := [][]string{
		{"A", "file1"},
		{"M", "file2"},
		{"D", "file -> 3"},
		{"R100", "file\t4"},
		{"file5"},
		{"R066", "file6"},
		{"file7"},
	}
	var in string
	for _, row := range niceIn {
		in += strings.Join(row, "\x00") + "\x00"
	}
	out := ChangedFiles{
		"file1":     "A",
		"file2":     "M",
		"file -> 3": "D",
		"file\t4":   "R",
		"file5":     "R",
		"file6":     "R",
		"file7":     "R",
	}
	t.Run("git diff parsing", func(t *testing.T) {
		if got := convertDiffToChangedFiles(in); !reflect.DeepEqual(got, out) {
			prettyGot := ""
			for k, v := range got {
				prettyGot += k + " " + v + "\n"
			}
			prettyOut := ""
			for k, v := range out {
				prettyOut += k + " " + v + "\n"
			}
			t.Errorf("got:\n%s\nwant:\n%s", prettyGot, prettyOut)
		}
	})
}

func Test_convertStatusToChangedFiles(t *testing.T) {
	niceIn := [][]string{
		{"A ", "file1"},
		{"M ", "file2"},
		{"D ", "file3"},
		{"R ", "file4"},
		{"file5"},
		{"AM", "file6"},
		{"AD", "file7"},
		{"??", "file8"},
	}
	var in string
	for _, row := range niceIn {
		in += strings.Join(row, " ") + "\x00"
	}
	out := ChangedFiles{
		"file1": "A",
		"file2": "M",
		"file3": "D",
		"file4": "R",
		"file5": "R",
		"file6": "A",
		"file7": "A",
		"file8": "?",
	}
	t.Run("git status parsing", func(t *testing.T) {
		if got := convertStatusToChangedFiles(in); !reflect.DeepEqual(got, out) {
			prettyGot := ""
			for k, v := range got {
				prettyGot += k + " " + v + "\n"
			}
			prettyOut := ""
			for k, v := range out {
				prettyOut += k + " " + v + "\n"
			}
			t.Errorf("got:\n%s\nwant:\n%s", prettyGot, prettyOut)
		}
	})
}
