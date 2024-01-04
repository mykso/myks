package myks

import (
	"testing"
)

func Test_collectVendirSecrets(t *testing.T) {
	g := New(".")
	type args struct {
		secretName string
		envvars    map[string]string
	}
	tests := []struct {
		name string
		args args
		want map[string]*VendirCredentials
	}{
		{
			"correct secret",
			args{
				"loki-secret",
				map[string]string{
					g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "username",
					g.VendirSecretEnvPrefix + "LOKI-SECRET_PASSWORD": "password",
				},
			},
			map[string]*VendirCredentials{
				"loki-secret": {
					Username: "username",
					Password: "password",
				},
			},
		},
		{
			"empty username and password",
			args{
				"loki-secret",
				map[string]string{
					g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "",
					g.VendirSecretEnvPrefix + "LOKI-SECRET_PASSWORD": "",
				},
			},
			map[string]*VendirCredentials{},
		},
		{
			"empty username",
			args{
				"loki-secret",
				map[string]string{
					g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "",
					g.VendirSecretEnvPrefix + "LOKI-SECRET_PASSWORD": "password",
				},
			},
			map[string]*VendirCredentials{},
		},
		{
			"no password envvar",
			args{
				"loki-secret",
				map[string]string{
					g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "username",
				},
			},
			map[string]*VendirCredentials{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Logf("Running test %s", tt.name)
			for k, v := range tt.args.envvars {
				t.Setenv(k, v)
			}
			vendir := VendirSyncer{}
			got := vendir.collectVendirSecrets(New("."))
			assertEqual(t, got, tt.want)
		})
	}
}

func Test_generateVendirSecretYamls(t *testing.T) {
	g := New(".")
	type args struct {
		envvars map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			"one secret",
			args{map[string]string{
				g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "username",
				g.VendirSecretEnvPrefix + "LOKI-SECRET_PASSWORD": "password",
			}},
			"---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: loki-secret\ndata:\n  username: dXNlcm5hbWU=\n  password: cGFzc3dvcmQ=\n",
			false,
		},
		{
			"two secrets",
			args{map[string]string{
				g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "username1",
				g.VendirSecretEnvPrefix + "LOKI-SECRET_PASSWORD": "password1",
				g.VendirSecretEnvPrefix + "IKOL-SECRET_USERNAME": "username2",
				g.VendirSecretEnvPrefix + "IKOL-SECRET_PASSWORD": "password2",
			}},
			"---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: ikol-secret\ndata:\n  username: dXNlcm5hbWUy\n  password: cGFzc3dvcmQy\n" +
				"---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: loki-secret\ndata:\n  username: dXNlcm5hbWUx\n  password: cGFzc3dvcmQx\n",
			false,
		},
		{
			"one good secret, one incomplete secret",
			args{map[string]string{
				g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "username1",
				g.VendirSecretEnvPrefix + "LOKI-SECRET_PASSWORD": "password1",
				g.VendirSecretEnvPrefix + "IKOL-SECRET_USERNAME": "username2",
			}},
			"---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: loki-secret\ndata:\n  username: dXNlcm5hbWUx\n  password: cGFzc3dvcmQx\n",
			false,
		},
		{
			"two incomplete secrets",
			args{map[string]string{
				g.VendirSecretEnvPrefix + "LOKI-SECRET_USERNAME": "username1",
				g.VendirSecretEnvPrefix + "IKOL-SECRET_PASSWORD": "password2",
			}},
			"",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.args.envvars {
				t.Setenv(k, v)
			}
			vendir := VendirSyncer{}
			got, err := vendir.GenerateSecrets(New("."))
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assertEqual(t, got, tt.want)
		})
	}
}

func Test_generateVendirSecretYaml(t *testing.T) {
	type args struct {
		secretName string
		username   string
		password   string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{"happy args", args{"artifactory", "username", "password"}, "apiVersion: v1\nkind: Secret\nmetadata:\n  name: artifactory\ndata:\n  username: dXNlcm5hbWU=\n  password: cGFzc3dvcmQ=\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vendir := VendirSyncer{}
			got, err := vendir.generateVendirSecretYaml(New("."), tt.args.secretName, tt.args.username, tt.args.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateVendirSecretYaml() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assertEqual(t, string(got), tt.want)
		})
	}
}
