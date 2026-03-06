package myks

import (
	"testing"
)

func Test_hash(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want string
	}{
		{"happy path", "some-string", "90f97071bce4fa95"},
		{"happy path", "some-other-string", "b14167e5c06889c"},
		{"empty string", "", "cbf29ce484222325"},
	}
	for _, tt := range tests {
		t.Run(tt.a, func(t *testing.T) {
			if got, err := hashString(tt.b); got != tt.want {
				t.Errorf("hash() = %v, wantArgs %v", got, tt.want)
			} else if err != nil {
				t.Errorf("hash() error = %v", err)
			}
		})
	}
}
