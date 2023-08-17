package myks

import (
	"os"
	"testing"
)

func Test_writeSecretFile(t *testing.T) {
	type args struct {
		secretName string
		username   string
		password   string
		path       string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"happy path", args{"artifactory", "username", "password", "/tmp/secretfile"}, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: artifactory\ndata:\n  username: dXNlcm5hbWU=\n  password: cGFzc3dvcmQ=\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := writeSecretFile(tt.args.secretName, tt.args.path, tt.args.username, tt.args.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("writeSecretFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			file, err := os.ReadFile(tt.args.path)
			if err != nil {
				t.Errorf("writeFile() error = %v", err)
			}
			if string(file) != tt.want {
				t.Errorf("writeSecretFile() got = %v, wantArgs %v", string(file), tt.want)
			}
		})
	}
}
