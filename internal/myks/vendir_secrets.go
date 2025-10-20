package myks

import (
	"bytes"
	_ "embed"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
)

//go:embed templates/vendir/secret.ytt.yaml
var vendirSecretTemplate []byte

func (v *VendirSyncer) collectVendirSecrets(g *Globe) map[string]*VendirCredentials {
	vendirCredentials := make(map[string]*VendirCredentials)

	usrRgx := regexp.MustCompile("^" + g.VendirSecretEnvPrefix + "(.+)_USERNAME=(.*)$")
	pswRgx := regexp.MustCompile("^" + g.VendirSecretEnvPrefix + "(.+)_PASSWORD=(.*)$")

	envvars := os.Environ()
	// Sort envvars to produce deterministic output for testing
	sort.Strings(envvars)
	for _, envPair := range envvars {
		if usrRgx.MatchString(envPair) {
			match := usrRgx.FindStringSubmatch(envPair)
			secretName := strings.ToLower(match[1])
			username := match[2]
			if vendirCredentials[secretName] == nil {
				vendirCredentials[secretName] = &VendirCredentials{}
			}
			vendirCredentials[secretName].Username = username
		} else if pswRgx.MatchString(envPair) {
			match := pswRgx.FindStringSubmatch(envPair)
			secretName := strings.ToLower(match[1])
			password := match[2]
			if vendirCredentials[secretName] == nil {
				vendirCredentials[secretName] = &VendirCredentials{}
			}
			vendirCredentials[secretName].Password = password
		}
	}

	for secretName, credentials := range vendirCredentials {
		if credentials.Username == "" || credentials.Password == "" {
			log.Warn().Msg("Incomplete credentials for secret: " + secretName)
			delete(vendirCredentials, secretName)
		}
	}

	var secretNames []string
	for secretName := range vendirCredentials {
		secretNames = append(secretNames, secretName)
	}
	log.Debug().Msg(msgWithSteps("sync", v.Ident(), "Found vendir secrets: "+strings.Join(secretNames, ", ")))

	return vendirCredentials
}

func (v *VendirSyncer) GenerateSecrets(g *Globe) (string, error) {
	log.Debug().Msg(msgWithSteps("sync", v.Ident(), "Generating Secrets"))
	vendirCredentials := v.collectVendirSecrets(g)

	// sort secret names to produce deterministic output for testing
	var secretNames []string
	for secretName := range vendirCredentials {
		secretNames = append(secretNames, secretName)
	}
	sort.Strings(secretNames)

	var secretYamls string
	for _, secretName := range secretNames {
		credentials := vendirCredentials[secretName]
		secretYaml, err := v.generateVendirSecretYaml(g, secretName, credentials.Username, credentials.Password)
		if err != nil {
			return secretYamls, err
		}
		secretYamls += "---\n" + secretYaml
	}

	return secretYamls, nil
}

func (v *VendirSyncer) generateVendirSecretYaml(g *Globe, secretName string, username string, password string) (string, error) {
	res, err := runYttWithFilesAndStdin(
		nil,
		bytes.NewReader(vendirSecretTemplate),
		func(name string, err error, stderr string, args []string) {
			cmd := msgRunCmd("render vendir secret yaml", name, args)
			if err != nil {
				log.Error().Msg(g.Msg(cmd))
				log.Error().Err(err).Msg(g.Msg(stderr))
			} else {
				log.Debug().Msg(g.Msg(stderr))
			}
		},
		"--data-value=secret_name="+secretName,
		"--data-value=username="+username,
		"--data-value=password="+password,
	)
	if err != nil {
		return "", err
	}

	return res.Stdout, nil
}
