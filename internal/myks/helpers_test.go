package myks

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

const testDataDir = "../../testData/util"

func assertEqual(t *testing.T, got, want any) {
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected:\n%v\nGot:\n%v\nDifference:\n%v", want, got, diff(want, got))
	}
}

func diff(expected, actual any) string {
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
