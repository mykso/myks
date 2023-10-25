package myks

import (
	"reflect"
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

func Test_convertToChangedFiles(t *testing.T) {
	type args struct {
		changes string
	}
	tests := []struct {
		name string
		args args
		want ChangedFiles
	}{
		{
			"git diff",
			args{
				"A\tfile1\n" +
					"M\tfile2\n" +
					"D\tfile3\n",
			},
			ChangedFiles{
				"file1": "A",
				"file2": "M",
				"file3": "D",
			},
		},
		{
			"git status",
			args{
				"A  file1\n" +
					" M file2\n" +
					"?? file3\n",
			},
			ChangedFiles{
				"file1": "A",
				"file2": "M",
				"file3": "?",
			},
		},
		{
			"git diff and git status",
			args{
				"A\tfile1\n" +
					"M\tfile2\n" +
					"D\tfile3\n" +
					"A  file4\n" +
					" M file5\n" +
					"?? file6\n",
			},
			ChangedFiles{
				"file1": "A",
				"file2": "M",
				"file3": "D",
				"file4": "A",
				"file5": "M",
				"file6": "?",
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
